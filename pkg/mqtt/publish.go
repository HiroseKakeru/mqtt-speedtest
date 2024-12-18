package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"net/url"
	"os"
	"time"

	"github.com/HiroseKakeru/mqtt-speedtest/pkg/converter"
	"github.com/eclipse/paho.golang/autopaho"

	"github.com/eclipse/paho.golang/paho"
)

var (
	connectTimeoutSecond           = 7 * time.Second
	packetTimeoutSecond            = 300 * time.Second
	connectRetryDelaySecond        = 10 * time.Second
	keepAliveSecond         uint16 = 0
	sessionExpiryInterval   uint32 = 0
	qos                     byte   = 1
	payloadFormat           byte   = 0
	randomNumDigits                = 1000
)

func NewClient(ctx context.Context, brokerURL string, i int) (*autopaho.ConnectionManager, error) {
	randomNumStr := fmt.Sprint(rand.Intn(randomNumDigits))
	clientID := os.Getenv("MQTT_CLIENT_ID") + fmt.Sprint(i) + randomNumStr

	server, err := url.Parse(brokerURL)
	if err != nil {
		return nil, err
	}

	cliCfg := autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{server},
		KeepAlive:                     keepAliveSecond,
		ConnectRetryDelay:             connectRetryDelaySecond,
		ConnectTimeout:                connectTimeoutSecond,
		CleanStartOnInitialConnection: true,
		SessionExpiryInterval:         sessionExpiryInterval,
		OnConnectionUp:                func(*autopaho.ConnectionManager, *paho.Connack) { slog.Info("mqtt connection up", "worker_id", i) },
		OnConnectError:                func(err error) { slog.Info("error", "whilst attempting connection", err) },
		ClientConfig: paho.ClientConfig{
			ClientID:      clientID,
			PacketTimeout: packetTimeoutSecond,
			OnClientError: func(err error) { slog.Info("OnClientError", "error", err) },
			OnServerDisconnect: func(d *paho.Disconnect) {
				if d.Properties != nil {
					slog.Info("server requested disconnect", d.Properties.ReasonString, d.ReasonCode)
				} else {
					slog.Info("server requested disconnect", "reason code", d.ReasonCode)
				}
			},
		},
	}

	cm, err := autopaho.NewConnection(ctx, cliCfg)
	if err != nil {
		slog.Info("new connection", "error", err)
		return nil, err
	}
	if err := cm.AwaitConnection(ctx); err != nil {
		slog.Info("publisher done", "error", err)
	}
	return cm, nil
}

func MQTTPublish(ctx context.Context, cm *autopaho.ConnectionManager, topic string, payload []byte, msgType string) error {
	now := time.Now().In(converter.JPLoc)
	dayAfterTomorrowMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 23, 59, 50, 0, now.Location())
	duration := dayAfterTomorrowMidnight.Sub(now)
	messageExpiry := uint32(duration.Seconds())

	res, err := cm.Publish(ctx, &paho.Publish{
		QoS:    qos,
		Retain: true,
		Topic:  topic,
		Properties: &paho.PublishProperties{
			ContentType:   fmt.Sprintf("application/x-protobuf;msgType=%s", msgType),
			PayloadFormat: &payloadFormat,
			MessageExpiry: &messageExpiry,
		},
		Payload: payload,
	})

	if err != nil {
		return err
	}
	slog.Info("published message", "topic", topic, "response", res)
	return nil
}
