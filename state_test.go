package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func decodeJSON(t *testing.T, input string) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(input))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	return value
}

func TestPrinterStateMerge(t *testing.T) {
	state := newPrinterState()
	state.merge(decodeJSON(t, `{
		"nozzle_temper": 20,
		"bed_temper": 60,
		"ams": {"ams_exist_bits": "0", "tray_now": "254"}
	}`))
	state.merge(decodeJSON(t, `{
		"nozzle_temper": 210,
		"ams": {"tray_now": "0"}
	}`))

	snapshot, updated := state.snapshot()
	if updated.IsZero() {
		t.Fatal("updated timestamp was not set")
	}
	if got := stringValue(snapshot["nozzle_temper"]); got != "210" {
		t.Fatalf("nozzle_temper = %q, want 210", got)
	}
	if _, ok := snapshot["bed_temper"]; !ok {
		t.Fatal("bed_temper was lost during merge")
	}
	ams, ok := snapshot["ams"].(map[string]any)
	if !ok {
		t.Fatalf("ams has type %T", snapshot["ams"])
	}
	if ams["ams_exist_bits"] != "0" {
		t.Fatalf("ams_exist_bits = %v", ams["ams_exist_bits"])
	}
	if ams["tray_now"] != "0" {
		t.Fatalf("tray_now = %v", ams["tray_now"])
	}
}

func TestPrinterStateSnapshotIsCopy(t *testing.T) {
	state := newPrinterState()
	state.merge(decodeJSON(t, `{"nozzle_temper": 20, "ams": {"tray_now": "0"}}`))

	snapshot, _ := state.snapshot()
	snapshot["nozzle_temper"] = json.Number("999")
	ams := snapshot["ams"].(map[string]any)
	ams["tray_now"] = "changed"

	fresh, _ := state.snapshot()
	if got := stringValue(fresh["nozzle_temper"]); got != "20" {
		t.Fatalf("stored nozzle_temper = %q, want 20", got)
	}
	if got := fresh["ams"].(map[string]any)["tray_now"]; got != "0" {
		t.Fatalf("stored tray_now = %v, want 0", got)
	}
}

func TestValueHelpers(t *testing.T) {
	if got, ok := floatValue(json.Number("21.5")); !ok || got != 21.5 {
		t.Fatalf("floatValue(json.Number) = %v, %v", got, ok)
	}
	if got, ok := floatValue("75"); !ok || got != 75 {
		t.Fatalf("floatValue(string) = %v, %v", got, ok)
	}
	if got, ok := boolValue("true"); !ok || !got {
		t.Fatalf("boolValue(string) = %v, %v", got, ok)
	}
	if got, ok := boolValue(json.Number("0")); !ok || got {
		t.Fatalf("boolValue(json.Number) = %v, %v", got, ok)
	}
	if _, ok := boolValue("maybe"); ok {
		t.Fatal("boolValue accepted an invalid value")
	}
}

func TestWifiSignalDBm(t *testing.T) {
	tests := []struct {
		input any
		want  float64
		ok    bool
	}{
		{input: "-35dBm", want: -35, ok: true},
		{input: "-42", want: -42, ok: true},
		{input: json.Number("-50"), want: -50, ok: true},
		{input: "nope", ok: false},
	}
	for _, tt := range tests {
		got, ok := wifiSignalDBm(tt.input)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("wifiSignalDBm(%v) = %v, %v; want %v, %v", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}
