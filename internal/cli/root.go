// Package cli implements the Cobra-based command line interface for engram.
package cli

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/andrewhowdencom/engram/internal/config"
	istore "github.com/andrewhowdencom/engram/internal/store"
	"github.com/andrewhowdencom/engram/pkg/engram"
	"github.com/spf13/cobra"
)

// store holds the engram Store instance used by all subcommands.
// In the prototype this is always a FakeStore. In production it would
// be injected via Wire.
var store engram.Store

// Execute runs the CLI.
func Execute(ctx context.Context) error {
	return NewRootCmd().ExecuteContext(ctx)
}

// NewRootCmd creates and configures the root cobra command.
func NewRootCmd() *cobra.Command {
	var logLevel string

	cmd := &cobra.Command{
		Use:   "engram",
		Short: "engram is a memory agent store",
		Long:  "engram manages memory for agentic workflows.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Initialize slog level from flag.
			var level slog.Level
			if err := level.UnmarshalText([]byte(logLevel)); err != nil {
				return fmt.Errorf("invalid log level %q: %w", logLevel, err)
			}
			slog.SetLogLoggerLevel(level)

			// Load configuration (Viper + XDG).
			if err := config.Load(cmd.Context()); err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Initialise the SQLite-backed store.
			persistPath := filepath.Join(xdg.DataHome, "engram", "engram.db")
			var err error
			store, err = istore.NewSQLiteStore(persistPath)
			if err != nil {
				return fmt.Errorf("failed to initialise store: %w", err)
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newQueryCmd())
	cmd.AddCommand(newStoreCmd())
	cmd.AddCommand(newLinkCmd())
	cmd.AddCommand(newMCPCmd())

	return cmd
}

// BuildVersion holds the application version, injected at build time.
// Use go build -ldflags "-X github.com/andrewhowdencom/engram/internal/cli.BuildVersion=v0.1.0".
var BuildVersion string

// BuildTime holds the build timestamp, injected at build time.
var BuildTime string

func init() {
	if BuildVersion == "" {
		BuildVersion = "unknown"
	}
	if BuildTime == "" {
		BuildTime = "unknown"
	}
}
