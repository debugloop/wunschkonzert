package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Server is a implementation of StartStopServer which embeds a http.Server.
type Server struct {
	Name       string
	listenAddr string
	mux        *http.ServeMux
	httpServer http.Server
}

// NewServer returns a new http.Server that implements StartStopServer.
func NewServer(name string, listenAddr string) *Server {
	return &Server{
		Name:       name,
		listenAddr: listenAddr,
		mux:        http.NewServeMux(),
	}
}

// Handle is a convenience wrapper around the embedded mux Handle function. It automatically wraps the handler in a otel
// handler.
func (s *Server) Handle(pattern string, handler http.Handler) {
	s.mux.Handle(
		pattern,
		otelhttp.WithRouteTag(pattern, handler),
	)
}

// Run the Server with ListenAndServe. It is supposed to be called from a go routine.
func (s *Server) Run() error {
	s.httpServer = http.Server{
		Addr:    s.listenAddr,
		Handler: otelhttp.NewHandler(s.mux, s.Name),
	}
	slog.Info("Listening.", "name", s.Name, "address", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}

// Shutdown signals the server to terminate, ending the go routine around Run.
func (s *Server) Shutdown(grace time.Duration) {
	if grace == 0 {
		slog.Info("Shutting down with immediate stop.", "name", s.Name)
		s.Stop()
		return
	}

	slog.Info("Shutting down.", "name", s.Name, "grace", grace)

	deadline, deadlineCancel := context.WithTimeout(
		context.Background(),
		grace,
	)
	defer deadlineCancel()

	// Try a graceful shutdown, but close everything up after the deadline.
	err := s.httpServer.Shutdown(deadline)
	if errors.Is(err, context.DeadlineExceeded) {
		slog.Info("Deadline exceeded, stopping now.", "name", s.Name)
		s.Stop()
	}
}

// Stop signals an immediate stop, ending the go routine around Run.
func (s *Server) Stop() {
	err := s.httpServer.Close()
	if err == nil {
		slog.Info("Stopped, kthxbye.")
		return
	}
	if !errors.Is(err, http.ErrServerClosed) {
		slog.Error("Unclean termination of HTTP server.", "name", s.Name, "error", err)
	}
}
