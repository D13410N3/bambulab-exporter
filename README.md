# bambulab-exporter

Prometheus exporter for Bambu Lab printers over their local-network MQTT API.

Tested against a Bambu Lab A1 (EU). It should work with other LAN-mode printers
that expose the same `device/<serial>/report` MQTT protocol (A1 mini, P1, X1),
subject to firmware differences.

The exporter is read-only with respect to the printer: it subscribes to status
reports and sends `pushall` requests to obtain a full snapshot. It does not send
control commands.

## How it works

- Connects to the printer on TCP 8883 using TLS (the printer uses a self-signed
  certificate).
- Authenticates to MQTT as user `bblp` with the printer's LAN access code.
- Subscribes to `device/<serial>/report`.
- Publishes a `pushall` status request on connect and every `--refresh-interval`.
- Deep-merges the printer's incremental status reports.
- Exposes the merged state as Prometheus metrics.

The printer serial can be discovered from the TLS certificate CN, so normally
only the printer address and access code are required.

## Quick start

Requires Go 1.25 or newer.

```sh
go build -o bambulab-exporter .

BAMBU_PRINTER=192.0.2.10 \
BAMBU_ACCESS_CODE=12345678 \
./bambulab-exporter
```

Then:

```sh
curl http://localhost:9610/metrics
curl http://localhost:9610/health-check
```

`/health-check` returns `200` only when the MQTT session is connected and
subscribed; otherwise it returns `503`. `/healthz` is kept as an alias.

For a non-interactive deployment, keep the access code in a file:

```sh
printf '%s\n' '12345678' > /run/secrets/bambu-access-code
chmod 600 /run/secrets/bambu-access-code

BAMBU_PRINTER=192.0.2.10 \
BAMBU_ACCESS_CODE_FILE=/run/secrets/bambu-access-code \
./bambulab-exporter
```

## Configuration

| Flag | Environment variable | Default | Description |
| --- | --- | --- | --- |
| `--printer` | `BAMBU_PRINTER` | required | Printer host or `host:port`; port defaults to `8883`. |
| `--serial` | `BAMBU_SERIAL` | auto | Printer serial. Discovered from the TLS certificate CN when empty. |
| `--access-code` | `BAMBU_ACCESS_CODE` | | LAN access code. Prefer the environment or a file. |
| `--access-code-file` | `BAMBU_ACCESS_CODE_FILE` | | File containing the LAN access code. Takes precedence over `--access-code`. |
| `--listen-address` | `BAMBU_LISTEN_ADDRESS` (legacy) | built from `LISTEN_ADDR`/`LISTEN_PORT` | Full listen address override. |
| — | `LISTEN_ADDR` | `0.0.0.0` | Bind address used when `--listen-address` is not set. |
| — | `LISTEN_PORT` | `9610` | Bind port used when `--listen-address` is not set. |
| `--metrics-path` | `BAMBU_METRICS_PATH` | `/metrics` | Metrics endpoint path. |
| `--refresh-interval` | `BAMBU_REFRESH_INTERVAL` | `5m` | How often to request a full `pushall` status. |
| `--connect-timeout` | `BAMBU_CONNECT_TIMEOUT` | `10s` | Timeout for one MQTT connection attempt. |
| `--log-level` | `BAMBU_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |
| `--version` | | | Print the version and exit. |

The access code passed as a command-line flag may be visible in the process
list. Prefer `BAMBU_ACCESS_CODE_FILE` or `BAMBU_ACCESS_CODE`. The exporter warns
when the access code file is readable by group or others.

`BAMBU_LISTEN_ADDRESS` is a legacy full-address override. It is used only when
neither `LISTEN_ADDR` nor `LISTEN_PORT` is set. An explicit `--listen-address`
flag takes precedence over all environment variables.

## Metrics

All printer metrics carry a `printer` label containing the printer serial.
`bambu_exporter_build_info` is exporter-level and has `version` and `go_version`
labels. All metrics are gauges except the counters ending in `_total`.

### Exporter and connection

| Metric | Labels | Description |
| --- | --- | --- |
| `bambu_printer_connected` | `printer` | `1` while an MQTT session is active, `0` otherwise. |
| `bambu_last_report_timestamp_seconds` | `printer` | Unix timestamp of the last status report received. |
| `bambu_last_report_age_seconds` | `printer` | Seconds since the last status report. |
| `bambu_reports_total` | `printer` | Status reports received. |
| `bambu_report_errors_total` | `printer` | Status reports that could not be parsed. |
| `bambu_mqtt_reconnects_total` | `printer` | Reconnections after the first successful connection. |
| `bambu_exporter_build_info` | `version`, `go_version` | Build information. |

### Temperatures and fans

| Metric | Labels | Description |
| --- | --- | --- |
| `bambu_nozzle_temperature_celsius` | `printer` | Current nozzle temperature. |
| `bambu_nozzle_target_temperature_celsius` | `printer` | Target nozzle temperature. |
| `bambu_bed_temperature_celsius` | `printer` | Current bed temperature. |
| `bambu_bed_target_temperature_celsius` | `printer` | Target bed temperature. |
| `bambu_chamber_temperature_celsius` | `printer` | Reported chamber temperature. May be meaningless on open-frame printers such as the A1. |
| `bambu_nozzle_k_factor` | `printer` | Nozzle pressure advance (K) factor. |
| `bambu_fan_speed_percent` | `printer`, `fan` | Fan speed. `fan` is `cooling`, `heatbreak`, `aux`, or `chamber`. |

### Print job

| Metric | Labels | Description |
| --- | --- | --- |
| `bambu_print_state` | `printer`, `state` | Current state as a label (`idle`, `prepare`, `running`, `pause`, `finish`, `failed`, ...) with value `1`. |
| `bambu_print_progress_ratio` | `printer` | Print progress from `0` to `1`. |
| `bambu_print_remaining_seconds` | `printer` | Estimated remaining print time. |
| `bambu_print_layer` | `printer` | Current layer. |
| `bambu_print_layers_total` | `printer` | Total layers in the current print. |
| `bambu_print_error_code` | `printer` | Printer-reported print error code (`0` means no error). |
| `bambu_print_speed_level` | `printer` | Speed level selected on the printer. |
| `bambu_print_speed_magnitude` | `printer` | Speed magnitude in percent. |

### Printer health and accessories

| Metric | Labels | Description |
| --- | --- | --- |
| `bambu_wifi_signal_dbm` | `printer` | Wi-Fi signal strength. |
| `bambu_sdcard_present` | `printer` | `1` when an SD card is present. |
| `bambu_ams_status` | `printer`, `state` | One-hot AMS activity state: `1` for the current state, `0` for the other known states. |
| `bambu_light_on` | `printer`, `light` | `1` when the light is on, `0` when off. |
| `bambu_hms_errors` | `printer` | Number of active HMS health messages. |
| `bambu_hms_error` | `printer`, `code`, `attr`, `action` | Active HMS message, one series per message. |
| `bambu_hms_error_timestamp_seconds` | `printer`, `code`, `attr` | HMS message timestamp. |
| `bambu_printer_info` | `printer`, `nozzle_type`, `nozzle_diameter`, `lifecycle` | Static printer information. |

### AMS and external spool

These series appear only when the corresponding accessory or filament data is
reported. On an A1 without AMS the AMS unit/tray series are absent, but
`bambu_ams_status` is still reported because it is part of the print status.

### AMS status states

`bambu_ams_status` is a one-hot metric. Every known state is emitted on every
scrape with value `0`, except the current state, which has value `1`:

```text
bambu_ams_status{printer="...",state="printing"} 1
bambu_ams_status{printer="...",state="nozzle_heat"} 0
```

The raw printer value is packed as `main = (value >> 8) & 0xFF` and
`sub = value & 0xFF`. The state names are:

| State | Raw value | Meaning |
| --- | --- | --- |
| `idle` | `0x0000` | AMS idle |
| `filament_change` | `0x0100` | Filament change, no specific sub-step |
| `nozzle_heat` | `0x0102` | Heating the nozzle for a filament change |
| `cut_filament` | `0x0103` | Cutting filament |
| `pull_current_filament` | `0x0104` | Pulling the current filament |
| `push_new_filament` | `0x0105`, `0x0106` | Pushing/feeding new filament |
| `purge_old_filament` | `0x0107` | Purging old filament |
| `check_position` | `0x0108` | Checking filament position |
| `wait` | `0x0109` | Waiting |
| `check_position_again` | `0x010B` | Checking filament position again |
| `rfid_identifying` | `0x0200` | RFID identification |
| `assist` | `0x0300` | AMS assist/feed complete |
| `calibration` | `0x0400` | Calibration |
| `printing` | `0x0500` | Printing. This main status is not in BambuStudio's public enum, but is observed while the printer is running a print. |
| `cold_pull` | `0x0700` | Cold pull |
| `self_check` | `0x1000` | Self-check |
| `debug` | `0x2000` | Debug |

For an unknown raw value, the exporter emits one series named
`unknown_state_<raw>` with value `1`, where `<raw>` is the decimal
`ams_status` value. All known states are `0` in that case. The old unlabeled
raw `bambu_ams_status` series is no longer emitted; use the `state` label
instead.

BambuStudio distinguishes slightly different filament-loading sub-steps for
`0x0105` and `0x0106` depending on whether an AMS or the external spool is
loading (push new filament vs. confirm extruded). `ams_status` alone does not
carry the loading path, so both are exposed as `push_new_filament`.

| Metric | Labels | Description |
| --- | --- | --- |
| `bambu_ams_temperature_celsius` | `printer`, `ams` | AMS temperature. |
| `bambu_ams_humidity_level` | `printer`, `ams` | AMS humidity index reported by the printer. |
| `bambu_ams_tray_remaining_percent` | `printer`, `ams`, `tray` | Remaining filament in an AMS tray. |
| `bambu_ams_tray_info` | `printer`, `ams`, `tray`, `type`, `color` | AMS tray filament information. |
| `bambu_external_spool_remaining_percent` | `printer` | Remaining filament on the external spool. |
| `bambu_external_spool_info` | `printer`, `type`, `color` | External spool filament information. |

## Prometheus

```yaml
scrape_configs:
  - job_name: bambulab
    static_configs:
      - targets: ["192.0.2.20:9610"]
```

If you run multiple exporters for multiple printers, keep the `printer` label or
add a `printer_name` relabel so the series remain distinguishable.

## Docker

```sh
docker build -t bambulab-exporter .

docker run --rm -p 9610:9610 \
  -e BAMBU_PRINTER=192.0.2.10 \
  -e BAMBU_ACCESS_CODE_FILE=/run/secrets/bambu-access-code \
  -v "$PWD/access-code:/run/secrets/bambu-access-code:ro" \
  bambulab-exporter
```

Do not bake the access code into the image.

## A1 notes

The exporter was verified against an A1 on the local network. Observed A1
behavior:

- The TLS certificate CN is the printer serial.
- `gcode_state`, temperatures, fan speeds, print progress, layers, Wi-Fi signal,
  HMS messages, lights, and the external spool are reported.
- `chamber_temper` is present but does not represent a real chamber sensor on the
  open-frame A1; treat it with caution.
- Without AMS, `ams.ams` is empty and no `bambu_ams_*` series are emitted.
- The camera stream (TCP 6000) and FTPS (TCP 990) are not used.

## Development

```sh
make test
make race
make vet
make build
```

`make build` injects the version from `VERSION` into the binary.

A local, gitignored `build.sh <tag>` builds and pushes
`zot.onlineteam.ru/bambulab-exporter:<tag>` and `:latest`.

## Security notes

- The access code is a secret. Do not commit it or pass it on the command line in
  shared environments.
- The HTTP endpoints are unauthenticated and, by default, listen on
  `0.0.0.0:9610`. Restrict access to the exporter port at the network level.
- The printer uses a self-signed TLS certificate, so the exporter does not verify
  the CA. The MQTT access code is the authentication mechanism. Keep the exporter
  on a trusted network.
- The exporter does not log the access code or full status payloads.

## Limitations

- One printer per exporter process.
- Metrics are a snapshot of the printer state; there is no historical data.
- Field availability and semantics vary by model and firmware.
- This project is not affiliated with Bambu Lab.
