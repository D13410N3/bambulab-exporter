package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const mqttUsername = "bblp"

type exporter struct {
	cfg     config
	serial  string
	log     *slog.Logger
	state   *printerState
	version string

	client mqtt.Client

	connected     atomic.Bool
	subscribed    atomic.Bool
	everConnected atomic.Bool
	stopping      atomic.Bool
	reports       atomic.Uint64
	reportErrors  atomic.Uint64
	reconnects    atomic.Uint64
	sequence      atomic.Uint64
}

func newExporter(cfg config, serial, version string, logger *slog.Logger) *exporter {
	return &exporter{
		cfg:     cfg,
		serial:  serial,
		version: version,
		log:     logger,
		state:   newPrinterState(),
	}
}

func (e *exporter) reportTopic() string {
	return "device/" + e.serial + "/report"
}

func (e *exporter) requestTopic() string {
	return "device/" + e.serial + "/request"
}

func (e *exporter) run(ctx context.Context) {
	options := mqtt.NewClientOptions()
	options.AddBroker("ssl://" + e.cfg.printerAddr)
	options.SetClientID(clientID())
	options.SetUsername(mqttUsername)
	options.SetPassword(e.cfg.accessCode)
	options.SetTLSConfig(&tls.Config{
		// Bambu printers use a self-signed certificate. The LAN access code is
		// the authentication mechanism; no CA is available to verify the peer.
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	})
	options.SetCleanSession(true)
	options.SetAutoReconnect(true)
	options.SetConnectRetry(true)
	options.SetConnectRetryInterval(5 * time.Second)
	options.SetConnectTimeout(e.cfg.connectTimeout)
	options.SetKeepAlive(30 * time.Second)
	options.SetPingTimeout(10 * time.Second)
	options.SetMaxReconnectInterval(60 * time.Second)
	options.SetOnConnectHandler(e.onConnect)
	options.SetConnectionLostHandler(e.onConnectionLost)

	e.client = mqtt.NewClient(options)
	token := e.client.Connect()
	if !token.WaitTimeout(e.cfg.connectTimeout) {
		e.log.Info("initial MQTT connection is still in progress", "timeout", e.cfg.connectTimeout)
	} else if err := token.Error(); err != nil {
		e.log.Warn("initial MQTT connection failed; will keep retrying", "error", err)
	}

	go e.connectionWatchdog(ctx)
	go e.refreshLoop(ctx)

	<-ctx.Done()
	e.stopping.Store(true)
	e.client.Disconnect(250)
}

func (e *exporter) onConnect(client mqtt.Client) {
	if e.everConnected.Swap(true) {
		e.reconnects.Add(1)
	}
	e.connected.Store(true)
	e.subscribed.Store(false)
	e.log.Info("MQTT connected", "printer", e.cfg.printerAddr, "serial", e.serial)

	token := client.Subscribe(e.reportTopic(), 0, e.handleMessage)
	if !token.WaitTimeout(10 * time.Second) {
		e.log.Warn("MQTT subscribe timed out")
	} else if err := token.Error(); err != nil {
		e.log.Error("MQTT subscribe failed", "error", err)
	} else {
		e.subscribed.Store(true)
	}

	e.requestPushAll()
}

func (e *exporter) onConnectionLost(_ mqtt.Client, err error) {
	e.connected.Store(false)
	e.subscribed.Store(false)
	if e.stopping.Load() {
		e.log.Debug("MQTT connection closed")
		return
	}
	e.log.Warn("MQTT connection lost; reconnecting", "error", err)
}

func (e *exporter) connectionWatchdog(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !e.connected.Load() {
				e.log.Warn("MQTT is not connected; retrying in background", "printer", e.cfg.printerAddr)
			}
		}
	}
}

func (e *exporter) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(e.cfg.refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.requestPushAll()
		}
	}
}

func (e *exporter) requestPushAll() {
	if e.client == nil || !e.client.IsConnected() {
		return
	}
	sequence := e.sequence.Add(1)
	payload := fmt.Sprintf(`{"pushing":{"sequence_id":"%d","command":"pushall"}}`, sequence)
	token := e.client.Publish(e.requestTopic(), 0, false, payload)
	go func() {
		if !token.WaitTimeout(5 * time.Second) {
			e.log.Debug("pushall publish timed out")
			return
		}
		if err := token.Error(); err != nil {
			e.log.Warn("pushall publish failed", "error", err)
		}
	}()
}

func (e *exporter) handleMessage(_ mqtt.Client, message mqtt.Message) {
	if message.Topic() != e.reportTopic() {
		return
	}
	var document map[string]any
	decoder := json.NewDecoder(bytes.NewReader(message.Payload()))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		e.reportErrors.Add(1)
		e.log.Debug("could not parse printer report", "error", err)
		return
	}
	printObject, ok := document["print"].(map[string]any)
	if !ok {
		e.reportErrors.Add(1)
		return
	}
	e.state.merge(printObject)
	e.reports.Add(1)
}

func clientID() string {
	var random [4]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("bambulab-exporter-%d", time.Now().UnixNano())
	}
	return "bambulab-exporter-" + hex.EncodeToString(random[:])
}
