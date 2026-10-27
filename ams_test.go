package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestAMSStateName(t *testing.T) {
	tests := []struct {
		raw  int64
		want string
	}{
		{raw: 0x0000, want: "idle"},
		{raw: 0x0100, want: "filament_change"},
		{raw: 0x0102, want: "nozzle_heat"},
		{raw: 0x0103, want: "cut_filament"},
		{raw: 0x0104, want: "pull_current_filament"},
		{raw: 0x0105, want: "push_new_filament"},
		{raw: 0x0106, want: "push_new_filament"},
		{raw: 0x0107, want: "purge_old_filament"},
		{raw: 0x0108, want: "check_position"},
		{raw: 0x0109, want: "wait"},
		{raw: 0x010B, want: "check_position_again"},
		{raw: 0x0200, want: "rfid_identifying"},
		{raw: 0x0300, want: "assist"},
		{raw: 0x0400, want: "calibration"},
		{raw: 0x0500, want: "printing"},
		{raw: 0x0700, want: "cold_pull"},
		{raw: 0x1000, want: "self_check"},
		{raw: 0x2000, want: "debug"},
		{raw: 0x010A, want: "unknown_state_266"},
		{raw: 0x0600, want: "unknown_state_1536"},
		{raw: 0xFF00, want: "unknown_state_65280"},
	}
	for _, tt := range tests {
		if got := amsStateName(tt.raw); got != tt.want {
			t.Errorf("amsStateName(%#x) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestAMSStatusMetric(t *testing.T) {
	tests := []struct {
		name   string
		raw    int
		active string
	}{
		{name: "idle", raw: 0, active: "idle"},
		{name: "nozzle heat", raw: 258, active: "nozzle_heat"},
		{name: "printing", raw: 1280, active: "printing"},
		{name: "unknown sub-status", raw: 0x010A, active: "unknown_state_266"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := newPrinterState()
			state.merge(map[string]any{"ams_status": float64(tt.raw)})
			values := gatherAMSStatus(t, newBambuCollector("SERIAL", state))

			for _, known := range amsStatusStates {
				want := 0.0
				if known == tt.active {
					want = 1
				}
				if got := values[known]; got != want {
					t.Fatalf("state %q = %v, want %v", known, got, want)
				}
			}

			if containsString(amsStatusStates, tt.active) {
				if len(values) != len(amsStatusStates) {
					t.Fatalf("got %d series, want %d", len(values), len(amsStatusStates))
				}
				return
			}

			if got := values[tt.active]; got != 1 {
				t.Fatalf("unknown state %q = %v, want 1", tt.active, got)
			}
			if len(values) != len(amsStatusStates)+1 {
				t.Fatalf("got %d series, want %d", len(values), len(amsStatusStates)+1)
			}
		})
	}
}

func gatherAMSStatus(t *testing.T, collector *bambuCollector) map[string]float64 {
	t.Helper()
	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[string]float64)
	for _, family := range families {
		if family.GetName() != "bambu_ams_status" {
			continue
		}
		for _, metric := range family.GetMetric() {
			state := ""
			for _, label := range metric.GetLabel() {
				if label.GetName() == "state" {
					state = label.GetValue()
				}
			}
			values[state] = metric.GetGauge().GetValue()
		}
	}
	return values
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
