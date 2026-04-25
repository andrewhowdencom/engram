package cli

import (
	"fmt"
	"strings"

	"github.com/andrewhowdencom/engram/pkg/engram"
	"github.com/spf13/cobra"
)

func newStoreCmd() *cobra.Command {
	var (
		content string
		ctx     []string
	)

	cmd := &cobra.Command{
		Use:   "store",
		Short: "Store a new memory",
		Long: `Store creates a new memory in the agent store.

In the prototype this stores only in-memory; it is lost when the
process exits.

Examples:
  engram store --content "User prefers dark mode" --context agent=support-bot --context topic=preference`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pairs := make(map[string]string)
			for _, p := range ctx {
				parts := strings.SplitN(p, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid context pair %q, expected key=value", p)
				}
				pairs[parts[0]] = parts[1]
			}
			m := engram.Memory{
				Content: []byte(content),
				Context: pairs,
			}
			stored, err := store.Put(cmd.Context(), m)
			if err != nil {
				return err
			}
			fmt.Printf("Stored memory %s\n", stored.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&content, "content", "", "memory content (required)")
	cmd.Flags().StringArrayVar(&ctx, "context", nil, "context key=value (can repeat)")
	_ = cmd.MarkFlagRequired("content")

	return cmd
}
