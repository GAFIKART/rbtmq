package rbtmqlib

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// RabbitMQ основной интерфейс для работы с RabbitMQ
type RabbitMQ struct {
	publisher *publisher
	consumer  *consumer
	connector *connector
	mu        sync.RWMutex
}

// RabbitMQConfig конфигурация для RabbitMQ
type RabbitMQConfig struct {
	ConnectParams
	RoutingKey string
}

// NewRabbitMQ создает новый экземпляр RabbitMQ с общим connector
func NewRabbitMQ(config RabbitMQConfig) (*RabbitMQ, error) {
	if config.RoutingKey == "" {
		return nil, fmt.Errorf("routing key is required")
	}

	connector, err := newConnector(config.ConnectParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create connector: %w", err)
	}

	publisherParams := PublisherParams{
		ConnectParams: config.ConnectParams,
		RoutingKey:    config.RoutingKey,
	}

	publisher, err := newPublisherWithConnector(publisherParams, connector)
	if err != nil {
		connector.close()
		return nil, fmt.Errorf("failed to create publisher: %w", err)
	}

	consumerParams := ConsumerParams{
		ConnectParams: config.ConnectParams,
		RoutingKey:    config.RoutingKey,
	}

	consumer, err := newConsumerWithConnector(consumerParams, connector)
	if err != nil {
		connector.close()
		publisher.shutdown(context.Background())
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	return &RabbitMQ{
		publisher: publisher,
		consumer:  consumer,
		connector: connector,
	}, nil
}

// Publish отправляет сообщение
func (r *RabbitMQ) Publish(msg any) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.publisher == nil {
		return fmt.Errorf("publisher is not initialized")
	}

	return r.publisher.publish(msg)
}

// StartConsuming запускает потребление сообщений
func (r *RabbitMQ) StartConsuming() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.consumer == nil {
		return fmt.Errorf("consumer is not initialized")
	}

	return r.consumer.consume()
}

// GetMessages возвращает канал сообщений
func (r *RabbitMQ) GetMessages() <-chan *DeliveryMessage {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.consumer == nil {
		closedChan := make(chan *DeliveryMessage)
		close(closedChan)
		return closedChan
	}
	return r.consumer.Messages
}

// Shutdown корректно завершает работу RabbitMQ
func (r *RabbitMQ) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Printf("Shutting down RabbitMQ...")

	if r.consumer != nil {
		r.consumer.shutdown(ctx)
	}

	if r.publisher != nil {
		r.publisher.shutdown(ctx)
	}

	if r.connector != nil {
		r.connector.close()
	}

	log.Printf("RabbitMQ shutdown completed")
	return nil
}
