package rbtmqlib

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
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

	publisher, err := newPublisherWithConnector(PublisherParams(config), connector)
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

// PublishWithResponse отправляет сообщение и ожидает ответ
func (r *RabbitMQ) PublishWithResponse(msg any, timeout ...time.Duration) (json.RawMessage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.publisher == nil {
		return nil, fmt.Errorf("publisher is not initialized")
	}

	finalTimeout := 30 * time.Second
	if len(timeout) > 0 {
		finalTimeout = timeout[0]
	}

	return r.publisher.publishWithResponse(msg, finalTimeout)
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
		if err := r.consumer.shutdown(ctx); err != nil {
			logShutdownError("consumer", err)
		}
	}

	if r.publisher != nil {
		if err := r.publisher.shutdown(ctx); err != nil {
			logShutdownError("publisher", err)
		}
	}

	if r.connector != nil {
		r.connector.close()
		// Ждем завершения работы connector
		r.connector.waitForShutdown()
	}

	log.Printf("RabbitMQ shutdown completed")
	return nil
}

// Respond отправляет ответ на полученное сообщение
func (r *RabbitMQ) Respond(originalDelivery *DeliveryMessage, responsePayload any) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.publisher == nil {
		return fmt.Errorf("publisher is not initialized")
	}

	return r.publisher.respond(originalDelivery, responsePayload)
}
