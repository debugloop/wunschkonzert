package api

import (
	"context"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"
)

// OrchestratedServer is a interface for longer running processes manageable by the run method. It is used for API servers.
type OrchestratedServer interface {
	Run() error
	Shutdown(time.Duration)
}

// Orchestrate can run multiple OrchestratedServer instances. It will handle their shutdown based on the context
// parameter.
func Orchestrate(ctx context.Context, grace time.Duration, servers ...OrchestratedServer) *errgroup.Group {
	eg, groupCtx := errgroup.WithContext(ctx)

	go func() {
		select {
		case <-ctx.Done():
			slog.Info("Initiating requested shutdown for servers.")
		case <-groupCtx.Done():
			slog.Warn("Initiating shutdown for remaining servers after at least one exited with an error.")
		}
		for _, srv := range servers {
			go srv.Shutdown(grace)
		}
		slog.Info("Triggered shutdown on all servers.", "grace-period", grace)
	}()

	for _, srv := range servers {
		eg.Go(srv.Run)
	}
	return eg
}
