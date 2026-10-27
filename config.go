package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultMQTTPort = "8883"

type config struct {
	printerAddr     string
	serial          string
	accessCode      string
	accessCodeFile  string
	listenAddr      string
	metricsPath     string
	refreshInterval time.Duration
	connectTimeout  time.Duration
	logLevel        slog.Level
	showVersion     bool
}

func parseConfig(args []string, getenv func(string) string, output io.Writer) (config, error) {
	var cfg config

	refreshDefault, err := envDuration(getenv, "BAMBU_REFRESH_INTERVAL", 5*time.Minute)
	if err != nil {
		return cfg, err
	}
	connectDefault, err := envDuration(getenv, "BAMBU_CONNECT_TIMEOUT", 10*time.Second)
	if err != nil {
		return cfg, err
	}
	listenDefault, err := listenAddressFromEnv(getenv)
	if err != nil {
		return cfg, err
	}

	fs := flag.NewFlagSet("bambulab-exporter", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: bambulab-exporter [flags]")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Flags:")
		fs.PrintDefaults()
	}

	fs.StringVar(&cfg.printerAddr, "printer", getenv("BAMBU_PRINTER"), "printer address (host or host:port); port defaults to 8883")
	fs.StringVar(&cfg.serial, "serial", getenv("BAMBU_SERIAL"), "printer serial; discovered from the TLS certificate CN when empty")
	fs.StringVar(&cfg.accessCode, "access-code", getenv("BAMBU_ACCESS_CODE"), "LAN access code (prefer --access-code-file or BAMBU_ACCESS_CODE)")
	fs.StringVar(&cfg.accessCodeFile, "access-code-file", getenv("BAMBU_ACCESS_CODE_FILE"), "file containing the LAN access code")
	fs.StringVar(&cfg.listenAddr, "listen-address", listenDefault, "address to serve metrics on; defaults to LISTEN_ADDR:LISTEN_PORT")
	fs.StringVar(&cfg.metricsPath, "metrics-path", envString(getenv, "BAMBU_METRICS_PATH", "/metrics"), "HTTP path for metrics")
	fs.DurationVar(&cfg.refreshInterval, "refresh-interval", refreshDefault, "how often to request a full printer status")
	fs.DurationVar(&cfg.connectTimeout, "connect-timeout", connectDefault, "timeout for a single MQTT connection attempt")
	logLevel := envString(getenv, "BAMBU_LOG_LEVEL", "info")
	fs.StringVar(&logLevel, "log-level", logLevel, "log level: debug, info, warn, error")
	fs.BoolVar(&cfg.showVersion, "version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if cfg.showVersion {
		return cfg, nil
	}

	cfg.printerAddr = strings.TrimSpace(cfg.printerAddr)
	if cfg.printerAddr == "" {
		return cfg, errors.New("printer address is required (--printer or BAMBU_PRINTER)")
	}
	cfg.printerAddr, err = normalizePrinterAddr(cfg.printerAddr)
	if err != nil {
		return cfg, err
	}

	cfg.serial = strings.TrimSpace(cfg.serial)
	cfg.accessCode = strings.TrimSpace(cfg.accessCode)
	cfg.accessCodeFile = strings.TrimSpace(cfg.accessCodeFile)
	if cfg.accessCodeFile == "" && cfg.accessCode == "" {
		return cfg, errors.New("access code is required (--access-code, --access-code-file, or BAMBU_ACCESS_CODE)")
	}

	cfg.listenAddr = strings.TrimSpace(cfg.listenAddr)
	if cfg.listenAddr == "" {
		return cfg, errors.New("listen address must not be empty")
	}
	if _, _, err := net.SplitHostPort(cfg.listenAddr); err != nil {
		return cfg, fmt.Errorf("invalid listen address %q: %w", cfg.listenAddr, err)
	}
	cfg.metricsPath = strings.TrimSpace(cfg.metricsPath)
	if !strings.HasPrefix(cfg.metricsPath, "/") {
		return cfg, fmt.Errorf("metrics path %q must start with /", cfg.metricsPath)
	}
	if cfg.refreshInterval <= 0 {
		return cfg, errors.New("refresh interval must be positive")
	}
	if cfg.connectTimeout <= 0 {
		return cfg, errors.New("connect timeout must be positive")
	}
	cfg.logLevel, err = parseLogLevel(logLevel)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

func (c config) loadAccessCode() (string, error) {
	if c.accessCodeFile != "" {
		data, err := os.ReadFile(c.accessCodeFile)
		if err != nil {
			return "", fmt.Errorf("read access code file: %w", err)
		}
		code := strings.TrimSpace(string(data))
		if code == "" {
			return "", errors.New("access code file is empty")
		}
		return code, nil
	}
	if c.accessCode == "" {
		return "", errors.New("access code is required")
	}
	return c.accessCode, nil
}

func listenAddressFromEnv(getenv func(string) string) (string, error) {
	addr := strings.TrimSpace(getenv("LISTEN_ADDR"))
	port := strings.TrimSpace(getenv("LISTEN_PORT"))

	// Preserve the old variable when the new ones are not used at all.
	if addr == "" && port == "" {
		if legacy := strings.TrimSpace(getenv("BAMBU_LISTEN_ADDRESS")); legacy != "" {
			return legacy, nil
		}
	}

	if addr == "" {
		addr = "0.0.0.0"
	}
	if port == "" {
		port = "9610"
	}
	if err := validatePort(port); err != nil {
		return "", fmt.Errorf("invalid LISTEN_PORT: %w", err)
	}

	addr = strings.TrimSpace(strings.Trim(addr, "[]"))
	if addr == "" {
		return "", errors.New("LISTEN_ADDR must not be empty")
	}
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return "", fmt.Errorf("LISTEN_ADDR %q must not include a port; use LISTEN_PORT", addr)
	}

	return net.JoinHostPort(addr, port), nil
}

func validatePort(value string) error {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%q is not a valid TCP port", value)
	}
	return nil
}

func normalizePrinterAddr(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("printer address is required")
	}

	if host, port, err := net.SplitHostPort(value); err == nil {
		if host == "" {
			return "", fmt.Errorf("invalid printer address %q: empty host", value)
		}
		if port == "" {
			return "", fmt.Errorf("invalid printer address %q: empty port", value)
		}
		return net.JoinHostPort(host, port), nil
	}

	host := strings.TrimSpace(strings.Trim(value, "[]"))
	if host == "" {
		return "", fmt.Errorf("invalid printer address %q", value)
	}
	return net.JoinHostPort(host, defaultMQTTPort), nil
}

func parseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log level %q (want debug, info, warn, or error)", value)
	}
}

func envString(getenv func(string) string, key, fallback string) string {
	if value := strings.TrimSpace(getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envDuration(getenv func(string) string, key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return duration, nil
}
