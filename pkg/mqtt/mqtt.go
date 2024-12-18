package mqtt

import (
	"context"
	"time"
)

type MQTTClient struct {
	topic   string
	timeout time.Duration
}

// Publish publishes a message to the MQTT broker.
// implements worker.Publisher interface
func (c *MQTTClient) Publish(payload []byte, msgType string) error {
	if err := clientManager.Publish(context.Background(), c.topic, payload, msgType); err != nil {
		return err
	}

	return nil
}

func NewMQTTClient(topic string, timeout time.Duration) *MQTTClient {
	return &MQTTClient{
		topic:   topic,
		timeout: timeout,
	}
}

func Publish(ctx context.Context, topic string, payload []byte, msgType string) error {
	return clientManager.Publish(ctx, topic, payload, msgType)
}
