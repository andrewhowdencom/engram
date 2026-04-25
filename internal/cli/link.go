package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLinkCmd() *cobra.Command {
	var (
		fromID   string
		toID     string
		linkType string
	)

	cmd := &cobra.Command{
		Use:   "link",
		Short: "Create a unidirected relationship between two memories",
		Long: `Link creates a relationship from one memory to another.
The relationship is unidirected (symmetric traversal in queries).

Examples:
  engram link --from code-1 --to code-2 --type depends_on`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := store.Link(cmd.Context(), fromID, toID, linkType); err != nil {
				return err
			}
			fmt.Printf("Linked %s → %s (%s)\n", fromID, toID, linkType)
			return nil
		},
	}

	cmd.Flags().StringVar(&fromID, "from", "", "source memory ID (required)")
	cmd.Flags().StringVar(&toID, "to", "", "target memory ID (required)")
	cmd.Flags().StringVar(&linkType, "type", "relates_to", "relationship type")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")

	return cmd
}
