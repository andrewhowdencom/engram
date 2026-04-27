package onnx

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"
)

// tokenizer implements a simplified BERT WordPiece tokenizer.
type tokenizer struct {
	vocab    map[string]int
	unkID    int
	clsID    int
	sepID    int
	padID    int
	maxLen   int
}

// newTokenizer loads a vocab.txt file (one token per line, index = ID).
func newTokenizer(vocabPath string) (*tokenizer, error) {
	//nolint:gosec // vocabPath is derived from a configurable model directory, not untrusted user input.
	f, err := os.Open(vocabPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	tok := &tokenizer{
		vocab: make(map[string]int),
		unkID: 100,
		clsID: 101,
		sepID: 102,
		padID: 0,
		maxLen: 128,
	}

	scanner := bufio.NewScanner(f)
	idx := 0
	for scanner.Scan() {
		tok.vocab[scanner.Text()] = idx
		idx++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Verify special tokens exist and set their IDs.
	for token, id := range map[string]int{"[UNK]": 100, "[CLS]": 101, "[SEP]": 102, "[PAD]": 0} {
		if _, ok := tok.vocab[token]; !ok {
			return nil, fmt.Errorf("vocab missing required token %q", token)
		}
		switch token {
		case "[UNK]":
			tok.unkID = id
		case "[CLS]":
			tok.clsID = id
		case "[SEP]":
			tok.sepID = id
		case "[PAD]":
			tok.padID = id
		}
	}

	return tok, nil
}

// encode converts text to token IDs, attention mask, and returns the valid length.
func (t *tokenizer) encode(text string) (inputIDs []int64, attentionMask []int64, length int) {
	// Pre-tokenise: lowercase, split on whitespace and punctuation, strip accents.
	words := basicTokenize(text)

	// Build token IDs.
	ids := []int64{int64(t.clsID)}
	for _, w := range words {
		ids = append(ids, t.wordPieceIDs(w)...)
	}
	ids = append(ids, int64(t.sepID))

	// Truncate or pad to maxLen.
	if len(ids) > t.maxLen {
		ids = ids[:t.maxLen-1]
		ids = append(ids, int64(t.sepID))
	}

	length = len(ids)
	mask := make([]int64, length)
	for i := range mask {
		mask[i] = 1
	}

	// Pad.
	for len(ids) < t.maxLen {
		ids = append(ids, int64(t.padID))
		mask = append(mask, 0)
	}

	return ids, mask, length
}

// basicTokenize does simple pre-tokenisation: lowercase, strip accents,
// split on whitespace and common punctuation.
func basicTokenize(text string) []string {
	var words []string
	var current strings.Builder

	for _, r := range strings.ToLower(text) {
		if unicode.IsSpace(r) || r == '.' || r == ',' || r == '!' || r == '?' ||
			r == ';' || r == ':' || r == '-' || r == '_' || r == '/' || r == '\\' {
			if current.Len() > 0 {
				words = append(words, stripAccents(current.String()))
				current.Reset()
			}
		} else {
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		words = append(words, stripAccents(current.String()))
	}
	return words
}

// stripAccents removes diacritics from Latin characters.
func stripAccents(s string) string {
	// Simplified: just strip combining marks. Full NFD decomposition is
	// overkill for an MVP; we handle the common cases.
	var out strings.Builder
	for _, r := range s {
		switch r {
		case 'á', 'à', 'â', 'ä', 'ã', 'å', 'ā':
			out.WriteRune('a')
		case 'é', 'è', 'ê', 'ë', 'ē':
			out.WriteRune('e')
		case 'í', 'ì', 'î', 'ï', 'ī':
			out.WriteRune('i')
		case 'ó', 'ò', 'ô', 'ö', 'õ', 'ō':
			out.WriteRune('o')
		case 'ú', 'ù', 'û', 'ü', 'ū':
			out.WriteRune('u')
		case 'ñ':
			out.WriteRune('n')
		case 'ç':
			out.WriteRune('c')
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

// wordPieceIDs applies WordPiece segmentation to a single word.
func (t *tokenizer) wordPieceIDs(word string) []int64 {
	if id, ok := t.vocab[word]; ok {
		return []int64{int64(id)}
	}

	var ids []int64
	remaining := word
	isBad := false
	start := 0

	for len(remaining) > 0 {
		end := len(remaining)
		curSubstr := ""
		found := false

		for end > start {
			substr := remaining[start:end]
			if start > 0 {
				substr = "##" + substr
			}
		if _, ok := t.vocab[substr]; ok {
			curSubstr = substr
			found = true
			break
		}
			end--
		}

		if !found {
			isBad = true
			break
		}

		ids = append(ids, int64(t.vocab[curSubstr]))
		start = end
	}

	if isBad {
		return []int64{int64(t.unkID)}
	}
	return ids
}
