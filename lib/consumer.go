package rbtmqlib

import (
	"context"
	"fmt"
	"log"
	"sync"
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
	if connector == nil {
		return nil, fmt.Errorf("connector cannot be nil")
	}

	ch := connector.getChannel()
	if ch == nil {
		return nil, fmt.Errorf("failed to get channel from connector")
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

	ch := c.connector.getChannel()
	if ch == nil {
		log.Printf("Channel not available")
		return
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
		log.Printf("Failed to start consuming: %v", err)
		return
	}

	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				return
			}
			c.Messages <- newDeliveryMessage(msg)

		case <-c.ctx.Done():
			return
		}
	}
}

// shutdown корректно завершает работу consumer
func (c *consumer) shutdown(ctx context.Context) error {
	log.Printf("Shutting down consumer...")
	c.cancel()
	c.wg.Wait()
	log.Printf("Consumer shutdown completed")
	return nil
}
