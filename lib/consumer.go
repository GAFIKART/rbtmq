package rbtmqlib

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// consumer слушает сообщения из очереди RabbitMQ
type consumer struct {
	connector *connector
	queueName string
	ctx       context.Context
	cancel    context.CancelFunc
	Messages  chan *DeliveryMessage
	startOnce sync.Once
	started   bool
	wg        sync.WaitGroup
}

// newConsumerWithConnector создаёт новый экземпляр consumer с существующим connector
func newConsumerWithConnector(params ConsumerParams, connector *connector) (*consumer, error) {
	ch, err := validateChannelFromConnector(connector)
	if err != nil {
		return nil, err
	}

	_, queueName, err := setupExchangeAndQueue(ch, params.RoutingKey, true, "", "")
	if err != nil {
		return nil, err
	}

	applyDefaultConsumerParams(&params)

	if err := ch.Qos(params.PrefetchCount, 0, false); err != nil {
		return nil, fmt.Errorf("failed to set QoS: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	consumer := &consumer{
		connector: connector,
		queueName: queueName,
		ctx:       ctx,
		cancel:    cancel,
		Messages:  make(chan *DeliveryMessage),
	}

	return consumer, nil
}

// consume запускает слушатель сообщений
func (c *consumer) consume() error {
	if c.started {
		return fmt.Errorf("consumer is already running")
	}

	c.startOnce.Do(func() {
		c.started = true
		c.wg.Add(1)
		go c.consumeLoop()
	})

	return nil
}

// consumeLoop основной цикл потребления сообщений
func (c *consumer) consumeLoop() {
	defer c.wg.Done()
	defer close(c.Messages)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if err := c.consumeMessages(); err != nil {
				log.Printf("Consumer error: %v, restarting in 2 seconds...", err)
				time.Sleep(2 * time.Second)
				continue
			}
		}
	}
}

// consumeMessages выполняет потребление сообщений с обработкой ошибок
func (c *consumer) consumeMessages() error {
	ch := c.connector.getChannel()
	if ch == nil {
		return fmt.Errorf("channel not available")
	}

	// Проверяем состояние канала
	if ch.IsClosed() {
		return fmt.Errorf("channel is closed")
	}

	msgs, err := ch.Consume(
		c.queueName, // queue
		"",          // consumer
		false,       // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	log.Printf("Started consuming from queue: %s", c.queueName)

	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("message channel closed")
			}

			// Проверяем, что consumer все еще активен
			select {
			case c.Messages <- newDeliveryMessage(msg):
			case <-c.ctx.Done():
				return nil
			}

		case <-c.ctx.Done():
			return nil
		}
	}
}

// shutdown корректно завершает работу consumer
func (c *consumer) shutdown(ctx context.Context) error {
	logShutdown("consumer")
	c.cancel()
	c.wg.Wait()
	return nil
}
