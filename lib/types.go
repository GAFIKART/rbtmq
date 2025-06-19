package rbtmqlib

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// ConnectParams параметры подключения к RabbitMQ
type ConnectParams struct {
	Username string
	Password string
	Host     string
	Port     int
	// Heartbeat интервал сердцебиения в секундах
	Heartbeat int
	// ConnectionTimeout таймаут подключения
	ConnectionTimeout time.Duration
}

// PublisherParams параметры Publisher
type PublisherParams struct {
	ConnectParams
	RoutingKey string
}

// ConsumerParams параметры Consumer
type ConsumerParams struct {
	ConnectParams
	RoutingKey    string
	PrefetchCount int
}

// DeliveryMessage обёртка для сообщений RabbitMQ
type DeliveryMessage struct {
	OriginalMessage amqp091.Delivery
	ReceivedAt      time.Time
}

// newDeliveryMessage создаёт новое сообщение
func newDeliveryMessage(msg amqp091.Delivery) *DeliveryMessage {
	return &DeliveryMessage{
		OriginalMessage: msg,
		ReceivedAt:      time.Now(),
	}
}

// Ack подтверждает обработку сообщения
func (dm *DeliveryMessage) Ack() error {
	return dm.OriginalMessage.Ack(false)
}

// Nack отклоняет сообщение (requeue — вернуть в очередь)
func (dm *DeliveryMessage) Nack(requeue bool) error {
	return dm.OriginalMessage.Nack(false, requeue)
}

// UnmarshalBody десериализует тело сообщения в структуру
func (dm *DeliveryMessage) UnmarshalBody(v any) error {
	if len(dm.OriginalMessage.Body) == 0 {
		return fmt.Errorf("message body is empty")
	}
	return json.Unmarshal(dm.OriginalMessage.Body, v)
}
