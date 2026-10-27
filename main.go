package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var version = "dev"

func main() {
	if err := runApp(os.Args[1:], os.Getenv, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func (e *exporter) healthHandler(w http.ResponseWriter, r *http.Request) {
	if e.connected.Load() && e.subscribed.Load() {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok\n")
		return
	}
	http.Error(w, "printer MQTT session is not ready", http.StatusServiceUnavailable)
}

func runApp(args []string, getenv func(string) string, stdout, stderr io.Writer) error {
	cfg, err := parseConfig(args, getenv, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if cfg.showVersion {
		fmt.Fprintln(stdout, version)
		return nil
	}

	logger := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: cfg.logLevel}))
	logger.Info("starting bambulab-exporter", "version", version, "go", runtime.Version())

	if cfg.accessCodeFile != "" {
		if info, err := os.Stat(cfg.accessCodeFile); err == nil {
			if permissions := info.Mode().Perm(); permissions&0o077 != 0 {
				logger.Warn("access code file is accessible by group or others",
					"path", cfg.accessCodeFile, "mode", fmt.Sprintf("%#o", permissions))
			}
		}
	}
	accessCode, err := cfg.loadAccessCode()
	if err != nil {
		return err
	}
	cfg.accessCode = accessCode

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serial := cfg.serial
	if serial == "" {
		logger.Info("serial not configured; discovering from the printer TLS certificate")
		serial, err = discoverSerial(ctx, cfg.printerAddr, cfg.connectTimeout)
		if err != nil {
			return fmt.Errorf("discover printer serial: %w (set --serial to skip discovery)", err)
		}
		logger.Info("discovered printer serial", "serial", serial)
	}

	exporter := newExporter(cfg, serial, version, logger)

	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewGoCollector())
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	exporter.registerMetrics(registry)

	mux := http.NewServeMux()
	mux.Handle(cfg.metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
		EnableOpenMetrics: true,
	}))
	mux.HandleFunc("/health-check", exporter.healthHandler)
	mux.HandleFunc("/healthz", exporter.healthHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, "<!doctype html><html><head><title>bambulab-exporter</title></head><body><h1>bambulab-exporter</h1><p><a href=%q>metrics</a></p></body></html>\n", cfg.metricsPath)
	})

	server := &http.Server{
		Addr:              cfg.listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("serving metrics", "address", cfg.listenAddr, "path", cfg.metricsPath)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	exporterDone := make(chan struct{})
	go func() {
		defer close(exporterDone)
		exporter.run(ctx)
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("serve metrics: %w", err)
	case <-ctx.Done():
	}
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	select {
	case <-exporterDone:
	case <-time.After(5 * time.Second):
		logger.Warn("MQTT shutdown timed out")
	}

	logger.Info("stopped")
	return nil
}
