package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kubectl-k8i/pkg/cli"
)

func main() {
	// Set up signal handling for graceful cancellation (SIGINT, SIGTERM).
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Create the root cobra command.
	cmd := cli.NewRootCommand()

	// Execute the command with the signal-aware context.
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
