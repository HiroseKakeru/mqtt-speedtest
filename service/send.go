package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/HiroseKakeru/mqtt-speedtest/pkg/mqtt"
	"github.com/HiroseKakeru/mqtt-speedtest/pkg/worker"
	"golang.org/x/sync/errgroup"
)

var (
	payloadSize = 1000
	limit       = 25
)

var (
	publishTimeout       = 3 * time.Minute
	publishTopicVideoFmt = "shinonome/speedtest/%d/data"
)

type OfferJob struct {
	publisher worker.Publisher
	payload   []byte
}

func (j *OfferJob) Send() error {
	return j.publisher.Publish(j.payload, "[]bytes")
}

func (j *OfferJob) PayloadOfferLength() int {
	if j.payload == nil {
		return 0
	}
	return len(j.payload)
}

func (j *OfferJob) PayloadSize() int {
	if j.payload == nil {
		return 0
	}
	return len(j.payload)
}

func SendSpeedTest(ctx context.Context, payload []byte) error {
	payloadCh := make(chan worker.Sender, payloadSize)
	dispatcher := worker.NewDispatcher()
	dispatcher.Run(payloadCh)

	eg, _ := errgroup.WithContext(ctx)
	for i := 0; i < limit; i++ {
		num := i
		eg.Go(func() error {
			topic := fmt.Sprintf(publishTopicVideoFmt, num)
			slog.Info("send", "Time", time.Now().Format("15:04:05.999999-07"), "topic", topic)
			mqttclient := mqtt.NewMQTTClient(topic, publishTimeout)
			payloadCh <- &OfferJob{
				publisher: mqttclient,
				payload:   payload,
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		fmt.Printf("error: %v", err)
	}

	close(payloadCh)

	dispatcher.Wait()
	return nil
}
