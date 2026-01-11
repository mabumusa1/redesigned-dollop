package app

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// WaitForShutdown blocks until a shutdown signal is received, then gracefully
// shuts down the application with the given timeout.
//
// It listens for:
// - SIGINT (Ctrl+C)
// - SIGTERM (container orchestration, systemd, etc.)
//
// The function ensures all resources are properly cleaned up before returning.
func WaitForShutdown(ctx *AppContext, timeout time.Duration) {
	// Create a channel to receive OS signals
	sigCh := make(chan os.Signal, 1)

	// Register for shutdown signals
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive a signal
	sig := <-sigCh

	ctx.Logger.Info("Shutdown signal received",
		slog.String("signal", sig.String()),
	)

	// Create a context with timeout for graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Perform graceful shutdown
	if err := ctx.Shutdown(shutdownCtx); err != nil {
		ctx.Logger.Error("Shutdown completed with errors",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	ctx.Logger.Info("Shutdown completed successfully")
}

// WaitForShutdownWithContext is similar to WaitForShutdown but also respects
// a parent context cancellation in addition to OS signals.
func WaitForShutdownWithContext(parentCtx context.Context, ctx *AppContext, timeout time.Duration) {
	// Create a channel to receive OS signals
	sigCh := make(chan os.Signal, 1)

	// Register for shutdown signals
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive a signal or parent context is cancelled
	select {
	case sig := <-sigCh:
		ctx.Logger.Info("Shutdown signal received",
			slog.String("signal", sig.String()),
		)
	case <-parentCtx.Done():
		ctx.Logger.Info("Parent context cancelled, initiating shutdown")
	}

	// Stop receiving signals
	signal.Stop(sigCh)

	// Create a context with timeout for graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Perform graceful shutdown
	if err := ctx.Shutdown(shutdownCtx); err != nil {
		ctx.Logger.Error("Shutdown completed with errors",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	ctx.Logger.Info("Shutdown completed successfully")
}

// RunWithGracefulShutdown is a convenience function that runs a startup function
// and then waits for shutdown signals. It's useful for main() functions.
//
// Example usage:
//
//	func main() {
//	    cfg := app.LoadConfig()
//	    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//
//	    app.RunWithGracefulShutdown(cfg, logger, 30*time.Second, opts, func(ctx *app.AppContext) error {
//	        // Start your servers and workers here
//	        return nil
//	    })
//	}
func RunWithGracefulShutdown(cfg *Config, logger *slog.Logger, timeout time.Duration, opts ContextOptions, startup func(*AppContext) error) {
	// Initialize the application context
	ctx, err := NewContext(cfg, logger, opts)
	if err != nil {
		logger.Error("Failed to initialize application context",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	// Run the startup function
	if err := startup(ctx); err != nil {
		logger.Error("Startup failed",
			slog.String("error", err.Error()),
		)
		// Attempt cleanup even on startup failure
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ctx.Shutdown(shutdownCtx)
		os.Exit(1)
	}

	// Wait for shutdown signal
	WaitForShutdown(ctx, timeout)
}
