// Command archivist sorts markdown files into a target directory tree and adds
// YAML frontmatter tags, driven by a System 1 Decision Model (von, laya).
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/jo-hoe/archivist/internal/cli"
)

func main() {
	// Cancel the run cleanly on Ctrl-C / SIGTERM so in-flight work stops at the
	// next file boundary — never mid-move.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := cli.NewRootCmd().ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
