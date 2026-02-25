package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/scottwater/stooges/internal/cli"
	"github.com/scottwater/stooges/internal/engine"
	apperrors "github.com/scottwater/stooges/internal/errors"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	svc := engine.NewService()
	cmd := cli.NewRootCmd(svc, cli.Streams{})
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(apperrors.ExitCode(err))
	}
}
