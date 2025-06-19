package rbtmqlib

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/rabbitmq/amqp091-go"
)

// publisher отправляет сообщения в RabbitMQ
type publisher struct {
	connector    *connector
	exchangeName string
	routingKey   string
}

// newPublisherWithConnector создаёт новый экземпляр publisher с существующим connector
func newPublisherWithConnector(params PublisherParams, connector *connector) (*publisher, error) {
	if connector == nil {
		return nil, fmt.Errorf("connector cannot be nil")
	}

	ch := connector.getChannel()
	if ch == nil {
		return nil, fmt.Errorf("failed to get channel")
	}

	exchangeName := extractExchangeName(params.RoutingKey)

	if err := declareExchange(ch, exchangeName, true); err != nil {
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	applyDefaultPublisherParams(&params)

	publisher := &publisher{
		connector:    connector,
		exchangeName: exchangeName,
		routingKey:   params.RoutingKey,
	}

	return publisher, nil
}

// publish отправляет сообщение
func (p *publisher) publish(msg any) error {
	ch := p.connector.getChannel()
	if ch == nil {
		return fmt.Errorf("channel not available")
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := validateMessageSize(body); err != nil {
		return err
	}

	err = ch.Publish(
		p.exchangeName,
		p.routingKey,
		false, // mandatory
		false, // immediate
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// shutdown корректно завершает работу publisher
func (p *publisher) shutdown(ctx context.Context) error {
	log.Printf("Shutting down publisher...")
	log.Printf("Publisher shutdown completed")
	return nil
}
