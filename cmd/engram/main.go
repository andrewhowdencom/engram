// Package main is the entrypoint for the engram CLI.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/andrewhowdencom/engram/internal/cli"
)

func main() {
	ctx := context.Background()
	if err := cli.Execute(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
