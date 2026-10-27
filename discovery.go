package main

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strings"
	"time"
)

// discoverSerial connects to the printer's TLS endpoint and uses the
// certificate CN as the device serial. Bambu printers issue a self-signed
// certificate with the serial number in the CN.
func discoverSerial(ctx context.Context, address string, timeout time.Duration) (string, error) {
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dialer := &tls.Dialer{
		Config: &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		},
		NetDialer: &net.Dialer{Timeout: timeout},
	}
	conn, err := dialer.DialContext(dialCtx, "tcp", address)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return "", errors.New("unexpected connection type")
	}
	certificates := tlsConn.ConnectionState().PeerCertificates
	if len(certificates) == 0 {
		return "", errors.New("printer presented no certificate")
	}
	serial := strings.TrimSpace(certificates[0].Subject.CommonName)
	if serial == "" {
		return "", errors.New("printer certificate has an empty CN")
	}
	return serial, nil
}
