// Package cli implements the Cobra-based command line interface for engram.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of engram",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s (built %s)\n", BuildVersion, BuildTime)
		},
	}
}
