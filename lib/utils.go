package rbtmqlib

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

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

// serializeMessage сериализует сообщение в JSON и валидирует размер
func serializeMessage(msg any) ([]byte, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := validateMessageSize(body); err != nil {
		return nil, err
	}

	return body, nil
}

// createPublishing создает структуру amqp091.Publishing с базовыми параметрами
func createPublishing(body []byte, correlationID, replyTo string) amqp091.Publishing {
	publishing := amqp091.Publishing{
		ContentType: "application/json",
		Body:        body,
	}

	if correlationID != "" {
		publishing.CorrelationId = correlationID
	}

	if replyTo != "" {
		publishing.ReplyTo = replyTo
	}

	return publishing
}

// validateChannel проверяет доступность канала
func validateChannel(ch *amqp091.Channel) error {
	if ch == nil {
		return fmt.Errorf("channel not available")
	}

	// Проверяем, не закрыт ли канал
	if ch.IsClosed() {
		return fmt.Errorf("channel is closed")
	}

	return nil
}

// validateConnector проверяет, что connector не nil
func validateConnector(connector *connector) error {
	if connector == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	return nil
}

// validateChannelFromConnector проверяет доступность канала из connector
func validateChannelFromConnector(connector *connector) (*amqp091.Channel, error) {
	if err := validateConnector(connector); err != nil {
		return nil, err
	}

	ch := connector.getChannel()
	if ch == nil {
		return nil, fmt.Errorf("failed to get channel - connection may be down")
	}

	// Дополнительная проверка состояния канала
	if err := validateChannel(ch); err != nil {
		return nil, fmt.Errorf("channel validation failed: %w", err)
	}

	return ch, nil
}

// createTemporaryQueue создает временную очередь для ответов
func createTemporaryQueue(ch *amqp091.Channel) (*amqp091.Queue, error) {
	if err := validateChannel(ch); err != nil {
		return nil, err
	}

	// Попытки создания временной очереди с повторными попытками
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		queue, err := ch.QueueDeclare(
			"",    // name
			false, // durable
			true,  // delete when unused
			true,  // exclusive
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			if attempt < maxRetries-1 {
				log.Printf("Failed to declare reply queue, retrying in 500ms... (attempt %d/%d): %v", attempt+1, maxRetries, err)
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("failed to declare a reply queue after %d attempts: %w", maxRetries, err)
		}

		return &queue, nil
	}

	return nil, fmt.Errorf("failed to create temporary queue after %d attempts", maxRetries)
}

// logShutdown логирует процесс завершения работы компонента
func logShutdown(componentName string) {
	log.Printf("Shutting down %s...", componentName)
	log.Printf("%s shutdown completed", componentName)
}

// logShutdownError логирует ошибку при завершении работы компонента
func logShutdownError(componentName string, err error) {
	log.Printf("error shutting down %s: %v", componentName, err)
}

// decodeBase64IfNeeded декодирует base64 если данные закодированы
func decodeBase64IfNeeded(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	// Пытаемся декодировать как base64
	str := string(data)
	decoded, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		// Если не base64, возвращаем исходные данные
		return data, nil
	}

	return decoded, nil
}
