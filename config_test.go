package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNormalizePrinterAddr(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "host only", input: "192.0.2.10", want: "192.0.2.10:8883"},
		{name: "host and port", input: "192.0.2.10:1234", want: "192.0.2.10:1234"},
		{name: "ipv6 host", input: "[fe80::1]", want: "[fe80::1]:8883"},
		{name: "ipv6 host and port", input: "[fe80::1]:8883", want: "[fe80::1]:8883"},
		{name: "empty", input: "", wantErr: true},
		{name: "empty brackets", input: "[]", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePrinterAddr(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseConfigDefaults(t *testing.T) {
	env := map[string]string{
		"BAMBU_PRINTER":     "192.0.2.10",
		"BAMBU_ACCESS_CODE": "test-access-code",
	}
	cfg, err := parseConfig(nil, func(key string) string { return env[key] }, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.printerAddr != "192.0.2.10:8883" {
		t.Fatalf("printer address = %q", cfg.printerAddr)
	}
	if cfg.accessCode != "test-access-code" {
		t.Fatalf("access code = %q", cfg.accessCode)
	}
	if cfg.listenAddr != "0.0.0.0:9610" {
		t.Fatalf("listen address = %q", cfg.listenAddr)
	}
	if cfg.metricsPath != "/metrics" {
		t.Fatalf("metrics path = %q", cfg.metricsPath)
	}
	if cfg.refreshInterval != 5*time.Minute {
		t.Fatalf("refresh interval = %s", cfg.refreshInterval)
	}
	if cfg.connectTimeout != 10*time.Second {
		t.Fatalf("connect timeout = %s", cfg.connectTimeout)
	}
	if cfg.logLevel != slog.LevelInfo {
		t.Fatalf("log level = %s", cfg.logLevel)
	}
}

func TestParseConfigListenAddress(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		want    string
		wantErr bool
	}{
		{name: "defaults", want: "0.0.0.0:9610"},
		{
			name: "address and port",
			env:  map[string]string{"LISTEN_ADDR": "127.0.0.1", "LISTEN_PORT": "9999"},
			want: "127.0.0.1:9999",
		},
		{
			name: "port only",
			env:  map[string]string{"LISTEN_PORT": "9999"},
			want: "0.0.0.0:9999",
		},
		{
			name: "address only",
			env:  map[string]string{"LISTEN_ADDR": "127.0.0.1"},
			want: "127.0.0.1:9610",
		},
		{
			name: "ipv6 address",
			env:  map[string]string{"LISTEN_ADDR": "::"},
			want: "[::]:9610",
		},
		{
			name: "legacy variable",
			env:  map[string]string{"BAMBU_LISTEN_ADDRESS": "127.0.0.1:7777"},
			want: "127.0.0.1:7777",
		},
		{
			name: "new variables win over legacy",
			env: map[string]string{
				"LISTEN_ADDR":          "127.0.0.1",
				"LISTEN_PORT":          "9999",
				"BAMBU_LISTEN_ADDRESS": "127.0.0.1:7777",
			},
			want: "127.0.0.1:9999",
		},
		{
			name: "flag overrides environment",
			args: []string{"--listen-address", "127.0.0.1:1234"},
			env:  map[string]string{"LISTEN_ADDR": "0.0.0.0", "LISTEN_PORT": "9999"},
			want: "127.0.0.1:1234",
		},
		{
			name:    "invalid port",
			env:     map[string]string{"LISTEN_PORT": "abc"},
			wantErr: true,
		},
		{
			name:    "port out of range",
			env:     map[string]string{"LISTEN_PORT": "70000"},
			wantErr: true,
		},
		{
			name:    "address with embedded port",
			env:     map[string]string{"LISTEN_ADDR": "127.0.0.1:9999"},
			wantErr: true,
		},
		{
			name:    "invalid flag",
			args:    []string{"--listen-address", "127.0.0.1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{
				"BAMBU_PRINTER":     "192.0.2.10",
				"BAMBU_ACCESS_CODE": "test-access-code",
			}
			for key, value := range tt.env {
				env[key] = value
			}

			cfg, err := parseConfig(tt.args, func(key string) string { return env[key] }, io.Discard)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got listen address %q", cfg.listenAddr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.listenAddr != tt.want {
				t.Fatalf("listen address = %q, want %q", cfg.listenAddr, tt.want)
			}
		})
	}
}

func TestParseConfigValidation(t *testing.T) {
	baseEnv := func() map[string]string {
		return map[string]string{
			"BAMBU_PRINTER":     "192.0.2.10",
			"BAMBU_ACCESS_CODE": "test-access-code",
		}
	}

	tests := []struct {
		name      string
		args      []string
		mutateEnv func(map[string]string)
		wantErr   bool
		version   bool
	}{
		{
			name: "missing printer",
			mutateEnv: func(env map[string]string) {
				delete(env, "BAMBU_PRINTER")
			},
			wantErr: true,
		},
		{
			name: "missing access code",
			mutateEnv: func(env map[string]string) {
				delete(env, "BAMBU_ACCESS_CODE")
			},
			wantErr: true,
		},
		{
			name:    "invalid log level",
			args:    []string{"--log-level", "verbose"},
			wantErr: true,
		},
		{
			name:    "invalid metrics path",
			args:    []string{"--metrics-path", "metrics"},
			wantErr: true,
		},
		{
			name:    "zero refresh interval",
			args:    []string{"--refresh-interval", "0s"},
			wantErr: true,
		},
		{
			name: "version without printer",
			args: []string{"--version"},
			mutateEnv: func(env map[string]string) {
				delete(env, "BAMBU_PRINTER")
				delete(env, "BAMBU_ACCESS_CODE")
			},
			version: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := baseEnv()
			if tt.mutateEnv != nil {
				tt.mutateEnv(env)
			}
			cfg, err := parseConfig(tt.args, func(key string) string { return env[key] }, io.Discard)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.showVersion != tt.version {
				t.Fatalf("showVersion = %v, want %v", cfg.showVersion, tt.version)
			}
		})
	}
}

func TestParseConfigInvalidEnvDuration(t *testing.T) {
	env := map[string]string{
		"BAMBU_PRINTER":          "192.0.2.10",
		"BAMBU_ACCESS_CODE":      "test-access-code",
		"BAMBU_REFRESH_INTERVAL": "soon",
	}
	if _, err := parseConfig(nil, func(key string) string { return env[key] }, io.Discard); err == nil {
		t.Fatal("expected an error")
	}
}

func TestLoadAccessCode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access-code")
	if err := os.WriteFile(path, []byte(" 12345678\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	code, err := (config{accessCodeFile: path}).loadAccessCode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "12345678" {
		t.Fatalf("code = %q", code)
	}

	code, err = (config{accessCode: "87654321"}).loadAccessCode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "87654321" {
		t.Fatalf("code = %q", code)
	}

	if _, err := (config{}).loadAccessCode(); err == nil {
		t.Fatal("expected an error for an empty access code")
	}
}
