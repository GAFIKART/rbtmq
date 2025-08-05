package rbtmqlib

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
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
	// Попытки отправки с повторными попытками
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		ch := p.connector.getChannel()
		if err := validateChannel(ch); err != nil {
			if attempt < maxRetries-1 {
				log.Printf("Channel not available, retrying in 1 second... (attempt %d/%d)", attempt+1, maxRetries)
				time.Sleep(time.Second)
				continue
			}
			return fmt.Errorf("failed to get channel after %d attempts: %w", maxRetries, err)
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
			if attempt < maxRetries-1 {
				log.Printf("Failed to publish message, retrying in 1 second... (attempt %d/%d): %v", attempt+1, maxRetries, err)
				time.Sleep(time.Second)
				continue
			}
			return fmt.Errorf("failed to publish message after %d attempts: %w", maxRetries, err)
		}

		return nil
	}

	return fmt.Errorf("failed to publish message after %d attempts", maxRetries)
}

// publishWithResponse отправляет сообщение и ожидает ответ
func (p *publisher) publishWithResponse(msg any, timeout time.Duration) ([]byte, error) {
	// Попытки отправки с повторными попытками
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		ch := p.connector.getChannel()
		if err := validateChannel(ch); err != nil {
			if attempt < maxRetries-1 {
				log.Printf("Channel not available for response, retrying in 1 second... (attempt %d/%d)", attempt+1, maxRetries)
				time.Sleep(time.Second)
				continue
			}
			return nil, fmt.Errorf("failed to get channel after %d attempts: %w", maxRetries, err)
		}

		// 1. Создание временной очереди для ответов
		replyQueue, err := createTemporaryQueue(ch)
		if err != nil {
			if attempt < maxRetries-1 {
				log.Printf("Failed to create reply queue, retrying in 1 second... (attempt %d/%d): %v", attempt+1, maxRetries, err)
				time.Sleep(time.Second)
				continue
			}
			return nil, fmt.Errorf("failed to create reply queue after %d attempts: %w", maxRetries, err)
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
			if attempt < maxRetries-1 {
				log.Printf("Failed to register consumer for reply queue, retrying in 1 second... (attempt %d/%d): %v", attempt+1, maxRetries, err)
				time.Sleep(time.Second)
				continue
			}
			return nil, fmt.Errorf("failed to register a consumer for reply queue after %d attempts: %w", maxRetries, err)
		}

		// 3. Генерация Correlation ID
		correlationID := uuid.New().String()

		body, err := serializeMessage(msg)
		if err != nil {
			return nil, err
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)

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
			cancel()
			if attempt < maxRetries-1 {
				log.Printf("Failed to publish message with response, retrying in 1 second... (attempt %d/%d): %v", attempt+1, maxRetries, err)
				time.Sleep(time.Second)
				continue
			}
			return nil, fmt.Errorf("failed to publish message after %d attempts: %w", maxRetries, err)
		}

		// 5. Ожидание ответа
		response, err := p.waitForResponse(msgs, correlationID, ctx)
		cancel()

		if err != nil {
			if attempt < maxRetries-1 {
				log.Printf("Failed to get response, retrying in 1 second... (attempt %d/%d): %v", attempt+1, maxRetries, err)
				time.Sleep(time.Second)
				continue
			}
			return nil, err
		}

		// 6. Декодируем base64 если необходимо
		decodedResponse, err := decodeBase64IfNeeded(response)
		if err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return decodedResponse, nil
	}

	return nil, fmt.Errorf("failed to publish message with response after %d attempts", maxRetries)
}

// waitForResponse ожидает ответ с указанным correlation ID
func (p *publisher) waitForResponse(msgs <-chan amqp091.Delivery, correlationID string, ctx context.Context) ([]byte, error) {
	for {
		select {
		case d, ok := <-msgs:
			if !ok {
				return nil, fmt.Errorf("message channel closed")
			}
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

	// Попытки отправки ответа с повторными попытками
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		ch := p.connector.getChannel()
		if err := validateChannel(ch); err != nil {
			if attempt < maxRetries-1 {
				log.Printf("Channel not available for response, retrying in 1 second... (attempt %d/%d)", attempt+1, maxRetries)
				time.Sleep(time.Second)
				continue
			}
			return fmt.Errorf("failed to get channel after %d attempts: %w", maxRetries, err)
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
			if attempt < maxRetries-1 {
				log.Printf("Failed to publish reply, retrying in 1 second... (attempt %d/%d): %v", attempt+1, maxRetries, err)
				time.Sleep(time.Second)
				continue
			}
			return fmt.Errorf("failed to publish reply after %d attempts: %w", maxRetries, err)
		}

		return nil
	}

	return fmt.Errorf("failed to publish reply after %d attempts", maxRetries)
}

// shutdown корректно завершает работу publisher
func (p *publisher) shutdown(ctx context.Context) error {
	logShutdown("publisher")
	return nil
}
