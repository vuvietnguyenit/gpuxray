package mon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// PIDResolver maps a PID to optional Kubernetes metadata.
// Implement this interface to plug in a real k8s resolver.
type PIDResolver interface {
	// Resolve attempts to return PodInfo for the given PID.
	// Returns nil, nil when the PID is not found in any pod.
}

// NoopResolver is the default resolver that returns nothing.
// Replace with a real implementation (e.g. via /proc + cgroup inspection) when needed.
type NoopResolver struct{}

// Config holds all tunables for the monitor server.
type Config struct {
	// ListenAddr is the TCP address for the HTTP metrics server,
	// e.g. ":9400" or "0.0.0.0:9400".
	ListenAddr string

	// MetricsPath is the HTTP path that exposes Prometheus metrics.
	// Defaults to "/metrics".
	MetricsPath string

	// Resolver is used to enrich PIDs with Kubernetes metadata.
	// Pass nil to use the no-op resolver (safe in non-k8s envs).
	Resolver PIDResolver

	// Logger is used for structured logging. Pass nil to use slog.Default().
	Logger *zerolog.Logger
}

func (c *Config) defaults() {
	if c.Logger == nil {
		var out io.Writer = os.Stderr
		l := zerolog.New(
			zerolog.ConsoleWriter{
				Out:        out,
				TimeFormat: "15:04:05",
			},
		).With().Timestamp().Logger()
		c.Logger = &l
	}
	if c.Resolver == nil {
		c.Resolver = &NoopResolver{}
	}
}

// Monitor owns the NVML lifecycle and the Prometheus HTTP server.
type Monitor struct {
	cfg Config
	srv *http.Server
}

// New creates a Monitor with the given config.
func New(cfg Config) *Monitor {
	cfg.defaults()
	return &Monitor{cfg: cfg}
}

// Run initialises NVML, registers the collector, starts the HTTP server,
// and blocks until ctx is cancelled.
// It always shuts down cleanly before returning.
func (m *Monitor) Run(ctx context.Context) error {
	// ── NVML init ─────────────────────────────────────────────────────────
	if ret := nvml.Init(); ret != nvml.SUCCESS {
		return fmt.Errorf("nvml init: %s", nvml.ErrorString(ret))
	}
	defer func() {
		if ret := nvml.Shutdown(); ret != nvml.SUCCESS {
			m.cfg.Logger.Warn().Msgf("nvml shutdown error: %s", nvml.ErrorString(ret))
		}
	}()

	driverVer, ret := nvml.SystemGetDriverVersion()
	if ret == nvml.SUCCESS {
		m.cfg.Logger.Info().Msgf("NVML initialised with driver version: %s", driverVer)
	}

	// ── Prometheus registry ───────────────────────────────────────────────
	reg := prometheus.NewRegistry()
	// Standard Go runtime + process metrics.
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	if _, err := NewCollector(reg, m.cfg.Resolver, m.cfg.Logger); err != nil {
		return fmt.Errorf("create gpu collector: %w", err)
	}

	// ── HTTP server ───────────────────────────────────────────────────────
	mux := http.NewServeMux()
	mux.Handle(m.cfg.MetricsPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	m.srv = &http.Server{
		Addr:         m.cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start the server in a goroutine.
	srvErr := make(chan error, 1)
	go func() {
		m.cfg.Logger.Info().Msgf("GPU monitor listening on %s at path %s", m.cfg.ListenAddr, m.cfg.MetricsPath)
		if err := m.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErr <- err
		}
	}()

	// Block until ctx is done or the server errors out.
	select {
	case <-ctx.Done():
		m.cfg.Logger.Info().Msg("shutting down GPU monitor")
	case err := <-srvErr:
		return fmt.Errorf("http server: %w", err)
	}

	// Graceful shutdown with a timeout.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.srv.Shutdown(shutdownCtx)
}
