package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/kubectl-k8i/pkg/cli"
	"k8s.io/klog/v2"
)

func main() {
	silenceClientGoLogs()

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

// silenceClientGoLogs discards client-go's internal klog output, such as the
// informational "Waited before sending request ... client-side throttling"
// message. These lines are not errors: real API failures are returned as
// errors and reported by the commands themselves. Left enabled, they are
// written to stderr and corrupt the table and watch screen.
func silenceClientGoLogs() {
	klog.LogToStderr(false)
	klog.SetOutput(io.Discard)
}
