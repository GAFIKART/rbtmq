package main

import (
	"context"
	"fmt"
	"log"
	"time"

	rbtmqlib "github.com/GAFIKART/rbtmq/lib"
)

// Message пример структуры сообщения
type Message struct {
	ID      string    `json:"id"`
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

func main() {
	// Конфигурация для RabbitMQ
	config := rbtmqlib.RabbitMQConfig{
		ConnectParams: rbtmqlib.ConnectParams{
			Username: "guest",
			Password: "guest",
			Host:     "localhost",
			Port:     5672,
		},
		RoutingKey: "test.messages",
	}

	// Создаем экземпляр RabbitMQ
	rabbitmq, err := rbtmqlib.NewRabbitMQ(config)
	if err != nil {
		log.Fatalf("Failed to create RabbitMQ: %v", err)
	}
	defer rabbitmq.Shutdown(context.Background())

	// Запускаем потребление сообщений
	if err := rabbitmq.StartConsuming(); err != nil {
		log.Fatalf("Failed to start consuming: %v", err)
	}

	// Получаем канал сообщений
	messages := rabbitmq.GetMessages()

	// Горутина для обработки сообщений
	go func() {
		for msg := range messages {
			log.Printf("Received message: %s", string(msg.OriginalMessage.Body))

			// Десериализуем сообщение
			var message Message
			if err := msg.UnmarshalBody(&message); err != nil {
				log.Printf("Failed to unmarshal message: %v", err)
				msg.Nack(true) // Возвращаем в очередь
				continue
			}

			log.Printf("Processed message: ID=%s, Content=%s, Time=%v",
				message.ID, message.Content, message.Time)

			// Подтверждаем обработку
			msg.Ack()
		}
	}()

	// Отправляем несколько тестовых сообщений
	for i := 1; i <= 5; i++ {
		message := Message{
			ID:      fmt.Sprintf("msg-%d", i),
			Content: fmt.Sprintf("Test message %d", i),
			Time:    time.Now(),
		}

		if err := rabbitmq.Publish(message); err != nil {
			log.Printf("Failed to publish message %d: %v", i, err)
		} else {
			log.Printf("Published message %d", i)
		}

		time.Sleep(1 * time.Second)
	}

	// Ждем обработки всех сообщений
	time.Sleep(10 * time.Second)

	log.Println("Example completed")
}
