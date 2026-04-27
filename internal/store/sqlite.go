// Package store provides implementations of the engram.Store interface.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andrewhowdencom/engram/pkg/engram"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore is an ACID persistent Store backed by SQLite.
type SQLiteStore struct {
	db       *sql.DB
	embedder engram.Embedder
}

// NewSQLiteStore opens (and optionally creates) a SQLite-backed engram store.
// It enables WAL mode and foreign key enforcement.
func NewSQLiteStore(path string, opts ...Option) (*SQLiteStore, error) {
	var cfg storeConfig
	for _, o := range opts {
		o(&cfg)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	db, err := sql.Open("sqlite3", path+"?_fk=1&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &SQLiteStore{db: db, embedder: cfg.embedder}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS memories (
		id          TEXT PRIMARY KEY,
		content     BLOB NOT NULL,
		created_at  DATETIME NOT NULL,
		updated_at  DATETIME NOT NULL,
		accessed_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS memory_context (
		memory_id TEXT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
		key       TEXT NOT NULL,
		value     TEXT NOT NULL,
		PRIMARY KEY (memory_id, key)
	);

	CREATE TABLE IF NOT EXISTS memory_links (
		from_id TEXT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
		to_id   TEXT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
		type    TEXT NOT NULL,
		PRIMARY KEY (from_id, to_id, type)
	);

	CREATE TABLE IF NOT EXISTS memory_embeddings (
		memory_id TEXT PRIMARY KEY REFERENCES memories(id) ON DELETE CASCADE,
		embedding TEXT NOT NULL   -- JSON array of float32
	);

	CREATE INDEX IF NOT EXISTS idx_memories_created_at ON memories(created_at);
	CREATE INDEX IF NOT EXISTS idx_memory_context_kv   ON memory_context(key, value);
	CREATE INDEX IF NOT EXISTS idx_memory_links_from   ON memory_links(from_id);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Put stores a new memory (or replaces an existing one with the same ID).
// If an embedder is configured and the memory has no embedding, one is
// generated automatically.
func (s *SQLiteStore) Put(ctx context.Context, m engram.Memory) (engram.Memory, error) {
	if m.ID == "" {
		m.ID = fmt.Sprintf("mem-%d", time.Now().UnixNano())
	}
	now := time.Now()
	m.CreatedAt = now
	m.UpdatedAt = now
	m.AccessedAt = now

	// Auto-generate embedding if missing.
	if s.embedder != nil && len(m.Embedding) == 0 && len(m.Content) > 0 {
		emb, err := s.embedder.Embed(ctx, string(m.Content))
		if err != nil {
			return m, fmt.Errorf("embed memory: %w", err)
		}
		m.Embedding = emb
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return m, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Upsert memory row.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO memories (id, content, created_at, updated_at, accessed_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			content     = excluded.content,
			updated_at  = excluded.updated_at,
			accessed_at = excluded.accessed_at
	`, m.ID, m.Content, m.CreatedAt, m.UpdatedAt, m.AccessedAt)
	if err != nil {
		return m, fmt.Errorf("insert memory: %w", err)
	}

	// Replace context.
	_, err = tx.ExecContext(ctx, `DELETE FROM memory_context WHERE memory_id = ?`, m.ID)
	if err != nil {
		return m, fmt.Errorf("delete old context: %w", err)
	}
	for k, v := range m.Context {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO memory_context (memory_id, key, value) VALUES (?, ?, ?)`,
			m.ID, k, v)
		if err != nil {
			return m, fmt.Errorf("insert context: %w", err)
		}
	}

	// Replace embedding.
	_, err = tx.ExecContext(ctx, `DELETE FROM memory_embeddings WHERE memory_id = ?`, m.ID)
	if err != nil {
		return m, fmt.Errorf("delete old embedding: %w", err)
	}
	if len(m.Embedding) > 0 {
		embJSON, err := json.Marshal(m.Embedding)
		if err != nil {
			return m, fmt.Errorf("marshal embedding: %w", err)
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO memory_embeddings (memory_id, embedding) VALUES (?, ?)`,
			m.ID, embJSON)
		if err != nil {
			return m, fmt.Errorf("insert embedding: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return m, fmt.Errorf("commit: %w", err)
	}
	return m, nil
}

// Query retrieves all memories that satisfy the hard filters (temporal,
// context, relationship). Results are returned unranked; scoring is the
// responsibility of the caller (e.g. search.Searcher).
func (s *SQLiteStore) Query(ctx context.Context, q engram.Query) ([]engram.Memory, error) {
	candidateIDs, err := s.filteredIDs(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(candidateIDs) == 0 {
		return nil, nil
	}
	return s.hydrateMemories(ctx, candidateIDs)
}

// filteredIDs returns memory IDs that satisfy the hard filters
// (temporal, context, relationship). If no hard filters are set,
// it returns every ID in the store.
func (s *SQLiteStore) filteredIDs(ctx context.Context, q engram.Query) ([]string, error) {
	var args []interface{}
	var where []string

	if q.Temporal != nil {
		if q.Temporal.After != nil {
			where = append(where, "m.created_at > ?")
			args = append(args, *q.Temporal.After)
		}
		if q.Temporal.Before != nil {
			where = append(where, "m.created_at < ?")
			args = append(args, *q.Temporal.Before)
		}
	}

	if q.ContextFilter != nil && len(q.ContextFilter.Pairs) > 0 {
		n := len(q.ContextFilter.Pairs)
		conds := make([]string, 0, n)
		for k, v := range q.ContextFilter.Pairs {
			conds = append(conds, "(key = ? AND value = ?)")
			args = append(args, k, v)
		}
		where = append(where, fmt.Sprintf(
			"m.id IN (SELECT memory_id FROM memory_context WHERE %s GROUP BY memory_id HAVING COUNT(DISTINCT key) = %d)",
			strings.Join(conds, " OR "), n,
		))
	}

	if q.Relationship != nil && q.Relationship.FromID != "" {
		ids, err := s.reachableIDs(ctx, q.Relationship.FromID, q.Relationship.Type, q.Relationship.Depth)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, nil
		}
		ph := make([]string, len(ids))
		for i := range ids {
			ph[i] = "?"
			args = append(args, ids[i])
		}
		where = append(where, fmt.Sprintf("m.id IN (%s)", strings.Join(ph, ",")))
	}

	query := "SELECT m.id FROM memories m"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("filter query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// reachableIDs returns all memory IDs reachable from origin within the
// given depth, following links of the optional type filter.
func (s *SQLiteStore) reachableIDs(ctx context.Context, origin, linkType string, depth int) ([]string, error) {
	if depth <= 0 {
		return nil, nil
	}

	var query string
	var args []interface{}

	if linkType != "" {
		query = `
		WITH RECURSIVE reachable(id, depth) AS (
			SELECT ?1, 0
			UNION ALL
			SELECT l.to_id, r.depth + 1
			FROM memory_links l
			JOIN reachable r ON l.from_id = r.id
			WHERE r.depth < ?2 AND l.type = ?3
		)
		SELECT DISTINCT id FROM reachable WHERE depth > 0`
		args = []interface{}{origin, depth, linkType}
	} else {
		query = `
		WITH RECURSIVE reachable(id, depth) AS (
			SELECT ?1, 0
			UNION ALL
			SELECT l.to_id, r.depth + 1
			FROM memory_links l
			JOIN reachable r ON l.from_id = r.id
			WHERE r.depth < ?2
		)
		SELECT DISTINCT id FROM reachable WHERE depth > 0`
		args = []interface{}{origin, depth}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("reachability cte: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan reachable id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// hydrateMemories loads full Memory structs for the given IDs in a single query.
func (s *SQLiteStore) hydrateMemories(ctx context.Context, ids []string) ([]engram.Memory, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	ph := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}

	//nolint:gosec // Only ? placeholders are injected via Sprintf; all values are passed as args.
	query := fmt.Sprintf(`
		SELECT
			m.id,
			m.content,
			m.created_at,
			m.updated_at,
			m.accessed_at,
			IFNULL(e.embedding, '[]') AS embedding,
			IFNULL((SELECT json_group_object(key, value) FROM memory_context WHERE memory_id = m.id), '{}') AS ctx,
			IFNULL((SELECT json_group_array(json_object('to', to_id, 'type', type)) FROM memory_links WHERE from_id = m.id), '[]') AS links
		FROM memories m
		LEFT JOIN memory_embeddings e ON m.id = e.memory_id
		WHERE m.id IN (%s)
	`, strings.Join(ph, ","))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("hydrate query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var memories []engram.Memory
	for rows.Next() {
		var m engram.Memory
		var embJSON, ctxJSON, linksJSON string

		if err := rows.Scan(&m.ID, &m.Content, &m.CreatedAt, &m.UpdatedAt, &m.AccessedAt, &embJSON, &ctxJSON, &linksJSON); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}

		if err := json.Unmarshal([]byte(ctxJSON), &m.Context); err != nil {
			return nil, fmt.Errorf("unmarshal context: %w", err)
		}
		if err := json.Unmarshal([]byte(embJSON), &m.Embedding); err != nil {
			return nil, fmt.Errorf("unmarshal embedding: %w", err)
		}

		var rawLinks []struct {
			To   string `json:"to"`
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(linksJSON), &rawLinks); err != nil {
			return nil, fmt.Errorf("unmarshal links: %w", err)
		}
		m.Links = make([]engram.Link, 0, len(rawLinks))
		for _, rl := range rawLinks {
			if rl.To != "" {
				m.Links = append(m.Links, engram.Link{To: rl.To, Type: rl.Type})
			}
		}

		memories = append(memories, m)
	}
	return memories, rows.Err()
}

// Link creates a unidirected relationship between two memories.
func (s *SQLiteStore) Link(ctx context.Context, from, to, linkType string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var count int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM memories WHERE id IN (?, ?)", from, to,
	).Scan(&count); err != nil {
		return fmt.Errorf("verify memories: %w", err)
	}
	if count != 2 {
		return fmt.Errorf("one or both memories not found")
	}

	_, err = tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO memory_links (from_id, to_id, type) VALUES (?, ?, ?)`,
		from, to, linkType,
	)
	if err != nil {
		return fmt.Errorf("insert link: %w", err)
	}

	return tx.Commit()
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

var _ engram.Store = (*SQLiteStore)(nil)
