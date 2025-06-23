package rbtmqlib

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// publisher отправляет сообщения в RabbitMQ
type publisher struct {
	connector    *connector
	exchangeName string
	routingKey   string
}

// newPublisherWithConnector создаёт новый экземпляр publisher с существующим connector
func newPublisherWithConnector(params PublisherParams, connector *connector) (*publisher, error) {
	ch, err := validateChannelFromConnector(connector)
	if err != nil {
		return nil, err
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
	if err := validateChannel(ch); err != nil {
		return err
	}

	body, err := serializeMessage(msg)
	if err != nil {
		return err
	}

	err = ch.Publish(
		p.exchangeName,
		p.routingKey,
		false, // mandatory
		false, // immediate
		createPublishing(body, "", ""),
	)

	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// publishWithResponse отправляет сообщение и ожидает ответ
func (p *publisher) publishWithResponse(msg any, timeout time.Duration) ([]byte, error) {
	ch := p.connector.getChannel()
	if err := validateChannel(ch); err != nil {
		return nil, err
	}

	// 1. Создание временной очереди для ответов
	replyQueue, err := createTemporaryQueue(ch)
	if err != nil {
		return nil, err
	}

	// 2. Создание потребителя для очереди ответов
	msgs, err := ch.Consume(
		replyQueue.Name, // queue
		"",              // consumer
		true,            // auto-ack
		true,            // exclusive
		false,           // no-local
		false,           // no-wait
		nil,             // args
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register a consumer for reply queue: %w", err)
	}

	// 3. Генерация Correlation ID
	correlationID := uuid.New().String()

	body, err := serializeMessage(msg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 4. Публикация сообщения с Correlation ID и ReplyTo
	err = ch.PublishWithContext(
		ctx,
		p.exchangeName, // exchange
		p.routingKey,   // routing key
		false,          // mandatory
		false,          // immediate
		createPublishing(body, correlationID, replyQueue.Name),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to publish message: %w", err)
	}

	// 5. Ожидание ответа
	for {
		select {
		case d := <-msgs:
			if d.CorrelationId == correlationID {
				return d.Body, nil
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for response timed out")
		}
	}
}

// respond отправляет ответ на полученное сообщение
func (p *publisher) respond(originalDelivery *DeliveryMessage, responsePayload any) error {
	originalMsg := originalDelivery.OriginalMessage
	if originalMsg.ReplyTo == "" {
		return fmt.Errorf("message does not have a ReplyTo field")
	}

	ch := p.connector.getChannel()
	if err := validateChannel(ch); err != nil {
		return err
	}

	body, err := serializeMessage(responsePayload)
	if err != nil {
		return err
	}

	err = ch.Publish(
		"",                  // exchange: default exchange
		originalMsg.ReplyTo, // routing key: the reply queue name
		false,               // mandatory
		false,               // immediate
		createPublishing(body, originalMsg.CorrelationId, ""),
	)

	if err != nil {
		return fmt.Errorf("failed to publish reply: %w", err)
	}

	return nil
}

// shutdown корректно завершает работу publisher
func (p *publisher) shutdown(ctx context.Context) error {
	logShutdown("publisher")
	return nil
}
