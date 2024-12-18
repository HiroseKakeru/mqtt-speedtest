package mqtt

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"golang.org/x/sync/errgroup"
)

var (
	clientManager *ClientManager
	initOnce      sync.Once
	ClientCount   int
	IsClientReady bool
)

const (
	maxClient       = 1
	maxMsgQueueSize = 1000
	maxMsgPerClient = 1
	tickerInterval  = 5 * time.Second
)

func Init() *ClientManager {
	// The ClientManager is initialized only once.
	initOnce.Do(func() {
		var err error
		clientManager, err = NewClientManager(maxClient)
		if err != nil {
			//log.Fatal("failed to initialize MQTT client manager: ", err)
			slog.Error("failed to initialize MQTT client manager", "error message", err)
		}
	})

	return clientManager
}

type ClientManager struct {
	size           int
	clients        chan Client
	msgQueue       chan Message
	stop           chan bool
	workingClients *sync.WaitGroup
}

func NewClientManager(size int) (*ClientManager, error) {
	clientManager := &ClientManager{
		size:           size,
		clients:        make(chan Client, size),
		msgQueue:       make(chan Message, maxMsgQueueSize),
		stop:           make(chan bool),
		workingClients: new(sync.WaitGroup),
	}

	url := os.Getenv("BROKER_URL")

	eg, egCtx := errgroup.WithContext(context.Background())
	defer egCtx.Done()
	for i := 0; i < size; i++ {
		i := i
		eg.Go(func() error {
			return func(url string, i int) error {
				cm, err := NewClient(context.Background(), url, i)
				if err != nil {
					slog.Error("new client connect failed", "error", err)
				}
				ClientCount++
				client := Client{
					id:                ClientCount,
					connectionManager: cm,
					msgQueue:          make(chan []Message, maxMsgPerClient),
					stop:              make(chan bool),
					wg:                clientManager.workingClients,
				}
				client.Start()

				clientManager.clients <- client

				return err
			}(url, i)
		})
	}

	if err := eg.Wait(); err != nil {
		slog.Error("failed to connect MQTT clients", "error", err)
		if ClientCount == 0 {
			return nil, err
		}
	}

	// Run the message processing goroutine
	clientManager.Run()

	clientManager.size = ClientCount
	IsClientReady = true
	return clientManager, nil
}

func (cm *ClientManager) PushClient(c Client) {
	defer cm.workingClients.Done()
	cm.clients <- c
}

func (cm *ClientManager) PopClient() Client {
	defer cm.workingClients.Add(1)
	c := <-cm.clients
	return c
}

type Message struct {
	Ctx     context.Context
	Topic   string
	Payload []byte
	MsgType string
}

func (cm *ClientManager) Run() {
	go func() {
		bulkMsg := make([]Message, 0, maxMsgPerClient)
		for {
			select {
			case msg := <-cm.msgQueue:
				bulkMsg = append(bulkMsg, msg)
				cm.SendBulkToClient(&bulkMsg)
			case <-cm.stop:
				// Close the message queue and wait for all messages to be processed
				close(cm.msgQueue)

				for msg := range cm.msgQueue {
					bulkMsg = append(bulkMsg, msg)
					if len(bulkMsg) == maxMsgPerClient {
						cm.SendBulkToClient(&bulkMsg)
					}
				}

				if len(bulkMsg) > 0 {
					cm.SendBulkToClient(&bulkMsg)
				}
				return
			}
		}
	}()
}

func (cm *ClientManager) SendBulkToClient(bulkMsg *[]Message) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("recovered panic", "error", r)
		}
	}()
	client := cm.PopClient()
	defer cm.PushClient(client)

	cpBulk := make([]Message, len(*bulkMsg))
	_ = copy(cpBulk, *bulkMsg)

	client.PushMessage(cpBulk)

	// Clear the bulk message
	*bulkMsg = (*bulkMsg)[:0]
}

func (cm *ClientManager) Publish(ctx context.Context, topic string, payload []byte, msgType string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("recovered panic", "error", r)
			err = errors.New("MQTT publish failed")
		}
	}()

	cm.msgQueue <- Message{
		Ctx:     ctx,
		Topic:   topic,
		Payload: payload,
		MsgType: msgType,
	}

	return err
}

func (cm *ClientManager) Close() {
	cm.stop <- true
	// Make sure all clients done before closing
	cm.workingClients.Wait()

	close(cm.clients)
	for c := range cm.clients {
		c.Stop()
	}
}

type Client struct {
	id                int
	connectionManager *autopaho.ConnectionManager
	msgQueue          chan []Message
	stop              chan bool
	wg                *sync.WaitGroup
}

func (c *Client) Start() {
	go func() {
		for {
			select {
			case bulkMsg := <-c.msgQueue:
				c.wg.Add(1)
				func() {
					defer c.wg.Done()
					c.SendInBulk(bulkMsg)
				}()
			case <-c.stop:
				slog.Debug("stopping client", "client_id", c.id)
				c.wg.Add(1)
				func() {
					defer c.wg.Done()
					err := c.connectionManager.Disconnect(context.Background())
					if err != nil {
						slog.Error("disconnect MQTT client failed", "error", err)
					}
				}()
				return
			}
		}
	}()
}

func (c *Client) Stop() {
	go func() {
		close(c.msgQueue)
		c.stop <- true
	}()
}

func (c *Client) SendInBulk(bulk []Message) {
	wg := sync.WaitGroup{}
	for _, msg := range bulk {
		wg.Add(1)
		go func(m Message) {
			defer wg.Done()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := MQTTPublish(ctx, c.connectionManager, m.Topic, m.Payload, m.MsgType)
			if err != nil {
				slog.Error("failed to publish message", "error", err)
			}
		}(msg)
	}
	wg.Wait()
}

func (c *Client) PushMessage(bulk []Message) {
	c.msgQueue <- bulk
}
