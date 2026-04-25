package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/andrewhowdencom/engram/pkg/engram"
	"github.com/spf13/cobra"
)

func newQueryCmd() *cobra.Command {
	var (
		contextFilters []string
		similarityText string
		similarityThr  float32
		relFrom        string
		relType        string
		relDepth       int
		after          string
		before         string
		orderBy        string
		limit          int
		focusPairs     []string
	)

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query memories across all four dimensions",
		Long: `Query retrieves memories matching constraints on context,
similarity, relationship, and time. Results are ranked by composite
relevance. An optional focus can be supplied per-query to warm up
results toward the agent's current operational context.

Focus is agent-managed: the agent maintains its own focus state and
passes it explicitly with each query. This allows the agent to
implement its own focus lifecycle (auto-clear on file switch,
decay, etc.) independently of the memory store.

Examples:
  # Query by context only
  engram query --context agent=coder --context project=engram

  # Query by similarity with focus
  engram query --similar "how do I configure embeddings" --focus agent=coder --limit 3

  # Query by relationship traversal
  engram query --rel-from code-1 --rel-depth 2

  # Query by time range, ordered by recency
  engram query --after "24h ago" --order recency`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runQuery(cmd.Context(), contextFilters, similarityText, similarityThr,
				relFrom, relType, relDepth, after, before, orderBy, limit, focusPairs)
		},
	}

	cmd.Flags().StringArrayVar(&contextFilters, "context", nil, "context key=value filter (can repeat)")
	cmd.Flags().StringVar(&similarityText, "similar", "", "text for semantic similarity search")
	cmd.Flags().Float32Var(&similarityThr, "similar-threshold", 0.0, "minimum similarity score (0-1)")
	cmd.Flags().StringVar(&relFrom, "rel-from", "", "relationship origin memory ID")
	cmd.Flags().StringVar(&relType, "rel-type", "", "relationship type filter")
	cmd.Flags().IntVar(&relDepth, "rel-depth", 1, "relationship traversal depth")
	cmd.Flags().StringVar(&after, "after", "", "only memories created after this duration (e.g. 24h, 7d)")
	cmd.Flags().StringVar(&before, "before", "", "only memories created before this duration")
	cmd.Flags().StringVar(&orderBy, "order", "relevance", "result order: relevance, recency, created")
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum number of results")
	cmd.Flags().StringArrayVar(&focusPairs, "focus", nil, "focus context key=value (can repeat, agent-managed)")

	return cmd
}

func runQuery(
	ctx context.Context,
	contextFilters []string,
	similarityText string,
	similarityThr float32,
	relFrom, relType string,
	relDepth int,
	after, before, orderBy string,
	limit int,
	focusPairs []string,
) error {
	q := engram.Query{Limit: limit}

	if len(contextFilters) > 0 {
		pairs := make(map[string]string)
		for _, f := range contextFilters {
			parts := strings.SplitN(f, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid context filter %q, expected key=value", f)
			}
			pairs[parts[0]] = parts[1]
		}
		q.ContextFilter = &engram.ContextFilter{Pairs: pairs}
	}

	if similarityText != "" {
		q.Similarity = &engram.SimilarityQuery{
			Text:      similarityText,
			Threshold: similarityThr,
		}
	}

	if relFrom != "" {
		q.Relationship = &engram.RelationshipQuery{
			FromID: relFrom,
			Type:   relType,
			Depth:  relDepth,
		}
	}

	tq := &engram.TemporalQuery{OrderBy: orderBy}
	if after != "" {
		d, err := parseDuration(after)
		if err != nil {
			return fmt.Errorf("invalid --after: %w", err)
		}
		t := time.Now().Add(-d)
		tq.After = &t
	}
	if before != "" {
		d, err := parseDuration(before)
		if err != nil {
			return fmt.Errorf("invalid --before: %w", err)
		}
		t := time.Now().Add(-d)
		tq.Before = &t
	}
	q.Temporal = tq

	if len(focusPairs) > 0 {
		pairs := make(map[string]string)
		for _, f := range focusPairs {
			parts := strings.SplitN(f, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid focus pair %q, expected key=value", f)
			}
			pairs[parts[0]] = parts[1]
		}
		q.Focus = &engram.Focus{Context: pairs}
	}

	results, err := store.Query(ctx, q)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		fmt.Println("No memories found.")
		return nil
	}

	fmt.Printf("Found %d memory(s):\n\n", len(results))
	for i, m := range results {
		fmt.Printf("--- %d. %s ---\n", i+1, m.ID)
		fmt.Printf("Content: %s\n", string(m.Content))
		fmt.Printf("Context: %v\n", m.Context)
		fmt.Printf("Links:   %v\n", formatLinks(m.Links))
		fmt.Printf("Created: %s\n\n", m.CreatedAt.Format(time.RFC3339))
	}
	return nil
}

func formatLinks(links []engram.Link) string {
	if len(links) == 0 {
		return "none"
	}
	var parts []string
	for _, l := range links {
		parts = append(parts, fmt.Sprintf("%s→%s", l.Type, l.To))
	}
	return strings.Join(parts, ", ")
}

// parseDuration extends time.ParseDuration with "d" for days.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
