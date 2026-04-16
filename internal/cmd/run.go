// Package cmd provides command-line interface functionality for the CLI Proxy API server.
// It includes authentication flows for various AI service providers, service startup,
// and other command-line operations.
package cmd

import (
	"context"
	"errors"
	"os/signal"
	"syscall"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/api"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy"
	log "github.com/sirupsen/logrus"
)

// BackgroundServiceHandle exposes lifecycle controls for a background service.
type BackgroundServiceHandle struct {
	cancel func()
	done   <-chan struct{}
	save   func() error
}

// Cancel requests graceful shutdown of the background service.
func (h *BackgroundServiceHandle) Cancel() {
	if h == nil || h.cancel == nil {
		return
	}
	h.cancel()
}

// Done returns a channel that closes when the background service exits.
func (h *BackgroundServiceHandle) Done() <-chan struct{} {
	if h == nil {
		return nil
	}
	return h.done
}

// PersistUsageSnapshot triggers an explicit usage snapshot save.
func (h *BackgroundServiceHandle) PersistUsageSnapshot() error {
	if h == nil || h.save == nil {
		return nil
	}
	return h.save()
}

// StartService builds and runs the proxy service using the exported SDK.
// It creates a new proxy service instance, sets up signal handling for graceful shutdown,
// and starts the service with the provided configuration.
//
// Parameters:
//   - cfg: The application configuration
//   - configPath: The path to the configuration file
//   - localPassword: Optional password accepted for local management requests
func StartService(cfg *config.Config, configPath string, localPassword string) {
	builder := cliproxy.NewBuilder().
		WithConfig(cfg).
		WithConfigPath(configPath).
		WithLocalManagementPassword(localPassword)

	ctxSignal, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	runCtx := ctxSignal
	if localPassword != "" {
		var keepAliveCancel context.CancelFunc
		runCtx, keepAliveCancel = context.WithCancel(ctxSignal)
		builder = builder.WithServerOptions(api.WithKeepAliveEndpoint(10*time.Second, func() {
			log.Warn("keep-alive endpoint idle for 10s, shutting down")
			keepAliveCancel()
		}))
	}

	service, err := builder.Build()
	if err != nil {
		log.Errorf("failed to build proxy service: %v", err)
		return
	}

	err = service.Run(runCtx)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Errorf("proxy service exited with error: %v", err)
	}
}

// StartServiceBackground starts the proxy service in a background goroutine
// and returns a cancel function for shutdown and a done channel.
func StartServiceBackground(cfg *config.Config, configPath string, localPassword string) (cancel func(), done <-chan struct{}) {
	handle := StartServiceBackgroundHandle(cfg, configPath, localPassword)
	if handle == nil {
		doneCh := make(chan struct{})
		close(doneCh)
		return func() {}, doneCh
	}
	return handle.Cancel, handle.Done()
}

// StartServiceBackgroundHandle starts the proxy service in a background goroutine
// and returns a handle for explicit shutdown and usage snapshot persistence.
func StartServiceBackgroundHandle(cfg *config.Config, configPath string, localPassword string) *BackgroundServiceHandle {
	builder := cliproxy.NewBuilder().
		WithConfig(cfg).
		WithConfigPath(configPath).
		WithLocalManagementPassword(localPassword)

	ctx, cancelFn := context.WithCancel(context.Background())
	doneCh := make(chan struct{})

	service, err := builder.Build()
	if err != nil {
		log.Errorf("failed to build proxy service: %v", err)
		close(doneCh)
		return &BackgroundServiceHandle{
			cancel: cancelFn,
			done:   doneCh,
		}
	}

	go func() {
		defer close(doneCh)
		if err := service.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Errorf("proxy service exited with error: %v", err)
		}
	}()

	return &BackgroundServiceHandle{
		cancel: cancelFn,
		done:   doneCh,
		save: func() error {
			return service.PersistUsageSnapshot(time.Now().UTC())
		},
	}
}

// WaitForCloudDeploy waits indefinitely for shutdown signals in cloud deploy mode
// when no configuration file is available.
func WaitForCloudDeploy() {
	// Clarify that we are intentionally idle for configuration and not running the API server.
	log.Info("Cloud deploy mode: No config found; standing by for configuration. API server is not started. Press Ctrl+C to exit.")

	ctxSignal, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Block until shutdown signal is received
	<-ctxSignal.Done()
	log.Info("Cloud deploy mode: Shutdown signal received; exiting")
}
