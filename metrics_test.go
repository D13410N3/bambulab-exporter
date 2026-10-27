package main

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

const sampleReport = `{
	"print": {
		"nozzle_temper": 210.5,
		"nozzle_target_temper": 210,
		"bed_temper": 60.25,
		"bed_target_temper": 60,
		"chamber_temper": 25,
		"nozzle_type": "stainless_steel",
		"nozzle_diameter": "0.4",
		"lifecycle": "product",
		"k": "0.0400",
		"cooling_fan_speed": "75",
		"mc_percent": 25,
		"mc_remaining_time": 30,
		"layer_num": 10,
		"total_layer_num": 40,
		"gcode_state": "RUNNING",
		"print_error": 0,
		"spd_lvl": 2,
		"spd_mag": 100,
		"wifi_signal": "-42dBm",
		"sdcard": true,
		"ams_status": 0,
		"lights_report": [{"node": "chamber_light", "mode": "on"}],
		"hms": [{"attr": 50335744, "code": 131073, "action": 0, "timestamp": 1700000000}],
		"ams": {
			"ams": [{
				"id": "0",
				"humidity": "3",
				"temp": "24.5",
				"tray": [
					{"id": "0", "remain": 80, "tray_type": "PLA", "tray_color": "FF0000FF"},
					{"id": "1", "remain": -1, "tray_type": "", "tray_color": "00000000"}
				]
			}]
		},
		"vt_tray": {"tray_type": "PETG", "tray_color": "000000FF", "remain": 50}
	}
}`

func TestBambuCollector(t *testing.T) {
	state := newPrinterState()
	document := decodeJSON(t, sampleReport)
	printObject, ok := document["print"].(map[string]any)
	if !ok {
		t.Fatal("sample report has no print object")
	}
	state.merge(printObject)

	collector := newBambuCollector("SERIAL", state)

	expected := `# HELP bambu_ams_humidity_level AMS humidity index reported by the printer.
# TYPE bambu_ams_humidity_level gauge
bambu_ams_humidity_level{ams="0",printer="SERIAL"} 3
# HELP bambu_ams_status Raw AMS status value reported by the printer.
# TYPE bambu_ams_status gauge
bambu_ams_status{printer="SERIAL"} 0
# HELP bambu_ams_temperature_celsius AMS temperature in degrees Celsius.
# TYPE bambu_ams_temperature_celsius gauge
bambu_ams_temperature_celsius{ams="0",printer="SERIAL"} 24.5
# HELP bambu_ams_tray_info AMS tray filament information.
# TYPE bambu_ams_tray_info gauge
bambu_ams_tray_info{ams="0",color="FF0000FF",printer="SERIAL",tray="0",type="PLA"} 1
# HELP bambu_ams_tray_remaining_percent Remaining filament in an AMS tray in percent.
# TYPE bambu_ams_tray_remaining_percent gauge
bambu_ams_tray_remaining_percent{ams="0",printer="SERIAL",tray="0"} 80
# HELP bambu_bed_target_temperature_celsius Target bed temperature in degrees Celsius.
# TYPE bambu_bed_target_temperature_celsius gauge
bambu_bed_target_temperature_celsius{printer="SERIAL"} 60
# HELP bambu_bed_temperature_celsius Current bed temperature in degrees Celsius.
# TYPE bambu_bed_temperature_celsius gauge
bambu_bed_temperature_celsius{printer="SERIAL"} 60.25
# HELP bambu_chamber_temperature_celsius Reported chamber temperature in degrees Celsius (may be meaningless on open-frame printers).
# TYPE bambu_chamber_temperature_celsius gauge
bambu_chamber_temperature_celsius{printer="SERIAL"} 25
# HELP bambu_external_spool_info External spool filament information.
# TYPE bambu_external_spool_info gauge
bambu_external_spool_info{color="000000FF",printer="SERIAL",type="PETG"} 1
# HELP bambu_external_spool_remaining_percent Remaining filament on the external spool in percent.
# TYPE bambu_external_spool_remaining_percent gauge
bambu_external_spool_remaining_percent{printer="SERIAL"} 50
# HELP bambu_fan_speed_percent Fan speed in percent.
# TYPE bambu_fan_speed_percent gauge
bambu_fan_speed_percent{fan="cooling",printer="SERIAL"} 75
# HELP bambu_hms_error Active HMS health message; one series per message.
# TYPE bambu_hms_error gauge
bambu_hms_error{action="0",attr="50335744",code="131073",printer="SERIAL"} 1
# HELP bambu_hms_error_timestamp_seconds Unix timestamp of the HMS message.
# TYPE bambu_hms_error_timestamp_seconds gauge
bambu_hms_error_timestamp_seconds{attr="50335744",code="131073",printer="SERIAL"} 1700000000
# HELP bambu_hms_errors Number of active HMS health messages.
# TYPE bambu_hms_errors gauge
bambu_hms_errors{printer="SERIAL"} 1
# HELP bambu_light_on Whether a printer light is on (1) or off (0).
# TYPE bambu_light_on gauge
bambu_light_on{light="chamber_light",printer="SERIAL"} 1
# HELP bambu_nozzle_k_factor Nozzle pressure advance (K) factor reported by the printer.
# TYPE bambu_nozzle_k_factor gauge
bambu_nozzle_k_factor{printer="SERIAL"} 0.04
# HELP bambu_nozzle_target_temperature_celsius Target nozzle temperature in degrees Celsius.
# TYPE bambu_nozzle_target_temperature_celsius gauge
bambu_nozzle_target_temperature_celsius{printer="SERIAL"} 210
# HELP bambu_nozzle_temperature_celsius Current nozzle temperature in degrees Celsius.
# TYPE bambu_nozzle_temperature_celsius gauge
bambu_nozzle_temperature_celsius{printer="SERIAL"} 210.5
# HELP bambu_printer_info Static printer information.
# TYPE bambu_printer_info gauge
bambu_printer_info{lifecycle="product",nozzle_diameter="0.4",nozzle_type="stainless_steel",printer="SERIAL"} 1
# HELP bambu_print_error_code Printer-reported print error code (0 means no error).
# TYPE bambu_print_error_code gauge
bambu_print_error_code{printer="SERIAL"} 0
# HELP bambu_print_layer Current print layer.
# TYPE bambu_print_layer gauge
bambu_print_layer{printer="SERIAL"} 10
# HELP bambu_print_layers_total Total layers in the current print.
# TYPE bambu_print_layers_total gauge
bambu_print_layers_total{printer="SERIAL"} 40
# HELP bambu_print_progress_ratio Print progress as a ratio between 0 and 1.
# TYPE bambu_print_progress_ratio gauge
bambu_print_progress_ratio{printer="SERIAL"} 0.25
# HELP bambu_print_remaining_seconds Estimated remaining print time in seconds.
# TYPE bambu_print_remaining_seconds gauge
bambu_print_remaining_seconds{printer="SERIAL"} 1800
# HELP bambu_print_speed_level Speed level selected on the printer (1-4).
# TYPE bambu_print_speed_level gauge
bambu_print_speed_level{printer="SERIAL"} 2
# HELP bambu_print_speed_magnitude Speed magnitude reported by the printer in percent.
# TYPE bambu_print_speed_magnitude gauge
bambu_print_speed_magnitude{printer="SERIAL"} 100
# HELP bambu_print_state Current print state; one series with the state label set to 1.
# TYPE bambu_print_state gauge
bambu_print_state{printer="SERIAL",state="running"} 1
# HELP bambu_sdcard_present Whether an SD card is present (1) or not (0).
# TYPE bambu_sdcard_present gauge
bambu_sdcard_present{printer="SERIAL"} 1
# HELP bambu_wifi_signal_dbm Wi-Fi signal strength in dBm.
# TYPE bambu_wifi_signal_dbm gauge
bambu_wifi_signal_dbm{printer="SERIAL"} -42
`

	metricNames := []string{
		"bambu_ams_humidity_level",
		"bambu_ams_status",
		"bambu_ams_temperature_celsius",
		"bambu_ams_tray_info",
		"bambu_ams_tray_remaining_percent",
		"bambu_bed_target_temperature_celsius",
		"bambu_bed_temperature_celsius",
		"bambu_chamber_temperature_celsius",
		"bambu_external_spool_info",
		"bambu_external_spool_remaining_percent",
		"bambu_fan_speed_percent",
		"bambu_hms_error",
		"bambu_hms_error_timestamp_seconds",
		"bambu_hms_errors",
		"bambu_light_on",
		"bambu_nozzle_k_factor",
		"bambu_nozzle_target_temperature_celsius",
		"bambu_nozzle_temperature_celsius",
		"bambu_printer_info",
		"bambu_print_error_code",
		"bambu_print_layer",
		"bambu_print_layers_total",
		"bambu_print_progress_ratio",
		"bambu_print_remaining_seconds",
		"bambu_print_speed_level",
		"bambu_print_speed_magnitude",
		"bambu_print_state",
		"bambu_sdcard_present",
		"bambu_wifi_signal_dbm",
	}

	if err := testutil.CollectAndCompare(collector, strings.NewReader(expected), metricNames...); err != nil {
		t.Fatal(err)
	}
}
