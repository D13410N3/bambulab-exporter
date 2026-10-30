package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestPrintErrorMessage(t *testing.T) {
	tests := []struct {
		code    int64
		hexCode string
		message string
	}{
		{
			code:    83902467,
			hexCode: "05004003",
			message: "Printing stopped because the printer was unable to parse the file. Please resend your print job.",
		},
		{
			code:    83935248,
			hexCode: "0500C010",
			message: "MicroSD Card read/write exception: please reinsert or replace the MicroSD Card.",
		},
		{
			code:    83902476,
			hexCode: "0500400C",
			message: "Please insert a MicroSD card and restart the print job.",
		},
		{
			code:    318734352,
			hexCode: "12FF8010",
			message: "Please check whether the external spool holder or filament is stuck.",
		},
		{
			code:    318750726,
			hexCode: "12FFC006",
			message: "Please feed filament into the PTFE tube until it can not be pushed any farther.",
		},
		{
			code:    318734343,
			hexCode: "12FF8007",
			message: "Check nozzle. Select 'Done' if filament was extruded, otherwise push filament forward slightly and select 'Retry.'",
		},
		{
			code:    1,
			hexCode: "00000001",
			message: "Unknown print error",
		},
	}

	for _, tt := range tests {
		hexCode, message := printErrorMessage(tt.code)
		if hexCode != tt.hexCode {
			t.Errorf("printErrorMessage(%d) hex = %q, want %q", tt.code, hexCode, tt.hexCode)
		}
		if message != tt.message {
			t.Errorf("printErrorMessage(%d) message = %q, want %q", tt.code, message, tt.message)
		}
	}
}

func TestPrintErrorInfoMetric(t *testing.T) {
	state := newPrinterState()
	state.merge(map[string]any{"print_error": float64(83902476)})
	values := gatherPrintErrorInfo(t, newBambuCollector("SERIAL", state))

	if len(values) != 1 {
		t.Fatalf("got %d series, want 1: %v", len(values), values)
	}
	if got := values["0500400C"]; got != "Please insert a MicroSD card and restart the print job." {
		t.Fatalf("message = %q", got)
	}

	state = newPrinterState()
	state.merge(map[string]any{"print_error": float64(0)})
	if values := gatherPrintErrorInfo(t, newBambuCollector("SERIAL", state)); len(values) != 0 {
		t.Fatalf("got %d series for a zero print_error, want 0", len(values))
	}
}

func gatherPrintErrorInfo(t *testing.T, collector *bambuCollector) map[string]string {
	t.Helper()
	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}

	values := make(map[string]string)
	for _, family := range families {
		if family.GetName() != "bambu_print_error_info" {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := make(map[string]string, len(metric.GetLabel()))
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			values[labels["code"]] = labels["message"]
		}
	}
	return values
}
