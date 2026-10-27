package main

import (
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type bambuCollector struct {
	serial string
	state  *printerState
	descs  []*prometheus.Desc

	printerInfo *prometheus.Desc

	lastReportTimestamp *prometheus.Desc
	lastReportAge       *prometheus.Desc

	nozzleTemperature       *prometheus.Desc
	nozzleTargetTemperature *prometheus.Desc
	bedTemperature          *prometheus.Desc
	bedTargetTemperature    *prometheus.Desc
	chamberTemperature      *prometheus.Desc
	nozzleKFactor           *prometheus.Desc
	fanSpeed                *prometheus.Desc

	printProgress       *prometheus.Desc
	printRemaining      *prometheus.Desc
	printLayer          *prometheus.Desc
	printLayersTotal    *prometheus.Desc
	printState          *prometheus.Desc
	printError          *prometheus.Desc
	printSpeedLevel     *prometheus.Desc
	printSpeedMagnitude *prometheus.Desc

	wifiSignal    *prometheus.Desc
	sdCardPresent *prometheus.Desc
	amsStatus     *prometheus.Desc
	lightOn       *prometheus.Desc

	hmsErrors         *prometheus.Desc
	hmsError          *prometheus.Desc
	hmsErrorTimestamp *prometheus.Desc

	amsTemperature   *prometheus.Desc
	amsHumidity      *prometheus.Desc
	amsTrayRemaining *prometheus.Desc
	amsTrayInfo      *prometheus.Desc

	externalSpoolRemaining *prometheus.Desc
	externalSpoolInfo      *prometheus.Desc
}

func newBambuCollector(serial string, state *printerState) *bambuCollector {
	collector := &bambuCollector{serial: serial, state: state}

	collector.printerInfo = collector.newDesc("bambu_printer_info",
		"Static printer information.", "nozzle_type", "nozzle_diameter", "lifecycle")
	collector.lastReportTimestamp = collector.newDesc("bambu_last_report_timestamp_seconds",
		"Unix timestamp of the last MQTT status report received from the printer.")
	collector.lastReportAge = collector.newDesc("bambu_last_report_age_seconds",
		"Seconds since the last MQTT status report was received.")

	collector.nozzleTemperature = collector.newDesc("bambu_nozzle_temperature_celsius",
		"Current nozzle temperature in degrees Celsius.")
	collector.nozzleTargetTemperature = collector.newDesc("bambu_nozzle_target_temperature_celsius",
		"Target nozzle temperature in degrees Celsius.")
	collector.bedTemperature = collector.newDesc("bambu_bed_temperature_celsius",
		"Current bed temperature in degrees Celsius.")
	collector.bedTargetTemperature = collector.newDesc("bambu_bed_target_temperature_celsius",
		"Target bed temperature in degrees Celsius.")
	collector.chamberTemperature = collector.newDesc("bambu_chamber_temperature_celsius",
		"Reported chamber temperature in degrees Celsius (may be meaningless on open-frame printers).")
	collector.nozzleKFactor = collector.newDesc("bambu_nozzle_k_factor",
		"Nozzle pressure advance (K) factor reported by the printer.")
	collector.fanSpeed = collector.newDesc("bambu_fan_speed_percent",
		"Fan speed in percent.", "fan")

	collector.printProgress = collector.newDesc("bambu_print_progress_ratio",
		"Print progress as a ratio between 0 and 1.")
	collector.printRemaining = collector.newDesc("bambu_print_remaining_seconds",
		"Estimated remaining print time in seconds.")
	collector.printLayer = collector.newDesc("bambu_print_layer",
		"Current print layer.")
	collector.printLayersTotal = collector.newDesc("bambu_print_layers_total",
		"Total layers in the current print.")
	collector.printState = collector.newDesc("bambu_print_state",
		"Current print state; one series with the state label set to 1.", "state")
	collector.printError = collector.newDesc("bambu_print_error_code",
		"Printer-reported print error code (0 means no error).")
	collector.printSpeedLevel = collector.newDesc("bambu_print_speed_level",
		"Speed level selected on the printer (1-4).")
	collector.printSpeedMagnitude = collector.newDesc("bambu_print_speed_magnitude",
		"Speed magnitude reported by the printer in percent.")

	collector.wifiSignal = collector.newDesc("bambu_wifi_signal_dbm",
		"Wi-Fi signal strength in dBm.")
	collector.sdCardPresent = collector.newDesc("bambu_sdcard_present",
		"Whether an SD card is present (1) or not (0).")
	collector.amsStatus = collector.newDesc("bambu_ams_status",
		"Raw AMS status value reported by the printer.")
	collector.lightOn = collector.newDesc("bambu_light_on",
		"Whether a printer light is on (1) or off (0).", "light")

	collector.hmsErrors = collector.newDesc("bambu_hms_errors",
		"Number of active HMS health messages.")
	collector.hmsError = collector.newDesc("bambu_hms_error",
		"Active HMS health message; one series per message.", "code", "attr", "action")
	collector.hmsErrorTimestamp = collector.newDesc("bambu_hms_error_timestamp_seconds",
		"Unix timestamp of the HMS message.", "code", "attr")

	collector.amsTemperature = collector.newDesc("bambu_ams_temperature_celsius",
		"AMS temperature in degrees Celsius.", "ams")
	collector.amsHumidity = collector.newDesc("bambu_ams_humidity_level",
		"AMS humidity index reported by the printer.", "ams")
	collector.amsTrayRemaining = collector.newDesc("bambu_ams_tray_remaining_percent",
		"Remaining filament in an AMS tray in percent.", "ams", "tray")
	collector.amsTrayInfo = collector.newDesc("bambu_ams_tray_info",
		"AMS tray filament information.", "ams", "tray", "type", "color")

	collector.externalSpoolRemaining = collector.newDesc("bambu_external_spool_remaining_percent",
		"Remaining filament on the external spool in percent.")
	collector.externalSpoolInfo = collector.newDesc("bambu_external_spool_info",
		"External spool filament information.", "type", "color")

	return collector
}

func (c *bambuCollector) newDesc(name, help string, labels ...string) *prometheus.Desc {
	desc := prometheus.NewDesc(name, help, labels, prometheus.Labels{"printer": c.serial})
	c.descs = append(c.descs, desc)
	return desc
}

func (c *bambuCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descs {
		ch <- desc
	}
}

func (c *bambuCollector) Collect(ch chan<- prometheus.Metric) {
	print, updated := c.state.snapshot()
	if len(print) == 0 {
		return
	}

	if !updated.IsZero() {
		ch <- prometheus.MustNewConstMetric(c.lastReportTimestamp, prometheus.GaugeValue, float64(updated.Unix()))
		ch <- prometheus.MustNewConstMetric(c.lastReportAge, prometheus.GaugeValue, time.Since(updated).Seconds())
	}

	emit := func(desc *prometheus.Desc, value float64, labels ...string) {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labels...)
	}
	emitIf := func(desc *prometheus.Desc, keys ...string) {
		if value, ok := floatAt(print, keys...); ok {
			emit(desc, value)
		}
	}

	emitIf(c.nozzleTemperature, "nozzle_temper")
	emitIf(c.nozzleTargetTemperature, "nozzle_target_temper")
	emitIf(c.bedTemperature, "bed_temper")
	emitIf(c.bedTargetTemperature, "bed_target_temper")
	emitIf(c.chamberTemperature, "chamber_temper")
	emitIf(c.nozzleKFactor, "k")

	for _, fan := range []struct {
		name string
		key  string
	}{
		{name: "cooling", key: "cooling_fan_speed"},
		{name: "heatbreak", key: "heatbreak_fan_speed"},
		{name: "aux", key: "big_fan1_speed"},
		{name: "chamber", key: "big_fan2_speed"},
	} {
		if value, ok := floatAt(print, fan.key); ok {
			emit(c.fanSpeed, value, fan.name)
		}
	}

	if percent, ok := floatAt(print, "mc_percent"); ok {
		emit(c.printProgress, percent/100)
	}
	if minutes, ok := floatAt(print, "mc_remaining_time"); ok {
		emit(c.printRemaining, minutes*60)
	}
	emitIf(c.printLayer, "layer_num")
	emitIf(c.printLayersTotal, "total_layer_num")
	if state, ok := stringAt(print, "gcode_state"); ok && state != "" {
		emit(c.printState, 1, strings.ToLower(state))
	}
	emitIf(c.printError, "print_error")
	emitIf(c.printSpeedLevel, "spd_lvl")
	emitIf(c.printSpeedMagnitude, "spd_mag")

	if value, ok := valueAt(print, "wifi_signal"); ok {
		if dbm, ok := wifiSignalDBm(value); ok {
			emit(c.wifiSignal, dbm)
		}
	}
	if present, ok := boolAt(print, "sdcard"); ok {
		emit(c.sdCardPresent, boolToFloat(present))
	}
	emitIf(c.amsStatus, "ams_status")

	if lights, ok := arrayAt(print, "lights_report"); ok {
		for _, item := range lights {
			light, ok := item.(map[string]any)
			if !ok {
				continue
			}
			node := stringValue(light["node"])
			if node == "" {
				continue
			}
			emit(c.lightOn, boolToFloat(strings.EqualFold(stringValue(light["mode"]), "on")), node)
		}
	}

	hms, _ := arrayAt(print, "hms")
	emit(c.hmsErrors, float64(len(hms)))
	seenHMS := make(map[string]struct{}, len(hms))
	seenHMSTimestamp := make(map[string]struct{}, len(hms))
	for _, item := range hms {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		code := stringValue(entry["code"])
		attr := stringValue(entry["attr"])
		action := stringValue(entry["action"])
		if code == "" {
			continue
		}
		key := code + "|" + attr + "|" + action
		if _, seen := seenHMS[key]; !seen {
			seenHMS[key] = struct{}{}
			emit(c.hmsError, 1, code, attr, action)
		}
		if timestamp, ok := floatValue(entry["timestamp"]); ok {
			keyTS := code + "|" + attr
			if _, seen := seenHMSTimestamp[keyTS]; !seen {
				seenHMSTimestamp[keyTS] = struct{}{}
				emit(c.hmsErrorTimestamp, timestamp, code, attr)
			}
		}
	}

	if ams, ok := objectAt(print, "ams"); ok {
		units, _ := ams["ams"].([]any)
		for unitIndex, item := range units {
			unit, ok := item.(map[string]any)
			if !ok {
				continue
			}
			amsID := stringValue(unit["id"])
			if amsID == "" {
				amsID = strconv.Itoa(unitIndex)
			}
			if value, ok := floatValue(unit["temp"]); ok {
				emit(c.amsTemperature, value, amsID)
			}
			if value, ok := floatValue(unit["humidity"]); ok {
				emit(c.amsHumidity, value, amsID)
			}
			trays, _ := unit["tray"].([]any)
			for trayIndex, trayItem := range trays {
				tray, ok := trayItem.(map[string]any)
				if !ok {
					continue
				}
				trayID := stringValue(tray["id"])
				if trayID == "" {
					trayID = strconv.Itoa(trayIndex)
				}
				if remain, ok := floatValue(tray["remain"]); ok && remain >= 0 {
					emit(c.amsTrayRemaining, remain, amsID, trayID)
				}
				trayType := stringValue(tray["tray_type"])
				if trayType != "" {
					emit(c.amsTrayInfo, 1, amsID, trayID, trayType, stringValue(tray["tray_color"]))
				}
			}
		}
	}

	if spool, ok := objectAt(print, "vt_tray"); ok {
		spoolType := stringValue(spool["tray_type"])
		if spoolType != "" {
			if remain, ok := floatValue(spool["remain"]); ok {
				emit(c.externalSpoolRemaining, remain)
			}
			emit(c.externalSpoolInfo, 1, spoolType, stringValue(spool["tray_color"]))
		}
	}

	nozzleType := stringValue(print["nozzle_type"])
	nozzleDiameter := stringValue(print["nozzle_diameter"])
	lifecycle := stringValue(print["lifecycle"])
	if nozzleType != "" || nozzleDiameter != "" || lifecycle != "" {
		emit(c.printerInfo, 1, nozzleType, nozzleDiameter, lifecycle)
	}
}

func (e *exporter) registerMetrics(reg prometheus.Registerer) {
	labels := prometheus.Labels{"printer": e.serial}

	reg.MustRegister(newBambuCollector(e.serial, e.state))
	reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "bambu_printer_connected",
		Help:        "Whether the exporter has an active MQTT session with the printer (1) or not (0).",
		ConstLabels: labels,
	}, func() float64 {
		return boolToFloat(e.connected.Load())
	}))
	reg.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name:        "bambu_reports_total",
		Help:        "Total MQTT status reports received from the printer.",
		ConstLabels: labels,
	}, func() float64 {
		return float64(e.reports.Load())
	}))
	reg.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name:        "bambu_report_errors_total",
		Help:        "Total MQTT status reports that could not be parsed.",
		ConstLabels: labels,
	}, func() float64 {
		return float64(e.reportErrors.Load())
	}))
	reg.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name:        "bambu_mqtt_reconnects_total",
		Help:        "Total MQTT reconnections after the first successful connection.",
		ConstLabels: labels,
	}, func() float64 {
		return float64(e.reconnects.Load())
	}))

	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bambu_exporter_build_info",
		Help: "Build information for bambulab-exporter.",
	}, []string{"version", "go_version"})
	buildInfo.WithLabelValues(e.version, runtime.Version()).Set(1)
	reg.MustRegister(buildInfo)
}

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
