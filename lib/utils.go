package rbtmqlib

import (
	"fmt"
	"strings"

	"github.com/rabbitmq/amqp091-go"
)

// setupExchangeAndQueue настраивает exchange и queue для consumer
func setupExchangeAndQueue(ch *amqp091.Channel, routingKey string, durable bool, queueName, exchangeName string) (string, string, error) {
	if ch == nil {
		return "", "", fmt.Errorf("channel is nil")
	}

	if exchangeName == "" {
		exchangeName = extractExchangeName(routingKey)
	}

	if queueName == "" {
		queueName = generateQueueName(routingKey)
	}

	if err := declareExchange(ch, exchangeName, durable); err != nil {
		return "", "", err
	}

	_, err := ch.QueueDeclare(
		queueName, // name
		durable,   // durable
		false,     // auto-delete
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to declare queue: %w", err)
	}

	err = ch.QueueBind(
		queueName,    // queue name
		routingKey,   // routing key
		exchangeName, // exchange
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to bind queue: %w", err)
	}

	return exchangeName, queueName, nil
}

// declareExchange объявляет exchange
func declareExchange(ch *amqp091.Channel, exchangeName string, durable bool) error {
	return ch.ExchangeDeclare(
		exchangeName, // name
		"direct",     // type
		durable,      // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
}

// extractExchangeName извлекает имя exchange из routing key
func extractExchangeName(routingKey string) string {
	parts := strings.Split(routingKey, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return "default"
}

// generateQueueName генерирует имя queue на основе routing key
func generateQueueName(routingKey string) string {
	queueName := strings.ReplaceAll(routingKey, ".", "_")
	if len(queueName) > 100 {
		queueName = queueName[:100]
	}
	return sanitizeQueueName(queueName)
}

// sanitizeQueueName очищает имя queue от недопустимых символов
func sanitizeQueueName(name string) string {
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, name)
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	result = strings.Trim(result, "_")
	if result == "" {
		result = "default_queue"
	}
	return result
}
