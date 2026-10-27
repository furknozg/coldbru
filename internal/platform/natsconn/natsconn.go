// Package natsconn opens the NATS connection and JetStream context services share (D-053).
package natsconn

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/furknozg/coldbru/internal/platform/health"
)

// KeyNATSURL is the config key holding the NATS server URL.
const KeyNATSURL = "NATS_URL"

const (
	reconnectWait  = 2 * time.Second
	connectTimeout = 5 * time.Second
)

// Connect dials url and returns the connection with a JetStream context. name
// identifies the client in server logs. After the first successful connection
// the client reconnects forever; disconnects and reconnects are logged.
func Connect(url, name string, logger *slog.Logger) (*nats.Conn, jetstream.JetStream, error) {
	nc, err := nats.Connect(url,
		nats.Name(name),
		nats.Timeout(connectTimeout),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(reconnectWait),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			logger.Warn("nats disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			logger.Info("nats reconnected", "url", c.ConnectedUrlRedacted())
		}),
		nats.ClosedHandler(func(*nats.Conn) {
			logger.Info("nats connection closed")
		}),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to nats: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("create jetstream context: %w", err)
	}
	return nc, js, nil
}

// Check reports ready only while nc is connected.
func Check(nc *nats.Conn) health.Check {
	return func(context.Context) error {
		if st := nc.Status(); st != nats.CONNECTED {
			return errors.New("nats status " + st.String())
		}
		return nil
	}
}
