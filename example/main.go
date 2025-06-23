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

// RequestMessage структура для запроса
type RequestMessage struct {
	Question string `json:"question"`
	ID       string `json:"id"`
}

// ResponseMessage структура для ответа
type ResponseMessage struct {
	Answer string `json:"answer"`
	ID     string `json:"id"`
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

			// Проверяем, является ли это запросом (есть ли ReplyTo)
			if msg.OriginalMessage.ReplyTo != "" {
				// Обрабатываем запрос и отправляем ответ
				var request RequestMessage
				if err := msg.UnmarshalBody(&request); err != nil {
					log.Printf("Failed to unmarshal request: %v", err)
					msg.Nack(true)
					continue
				}

				log.Printf("Received request: %s", request.Question)

				// Создаем ответ
				response := ResponseMessage{
					Answer: fmt.Sprintf("Answer to: %s", request.Question),
					ID:     request.ID,
				}

				// Отправляем ответ
				if err := rabbitmq.Respond(msg, response); err != nil {
					log.Printf("Failed to send response: %v", err)
				} else {
					log.Printf("Sent response for request ID: %s", request.ID)
				}

				msg.Ack()
			} else {
				// Обычное сообщение
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
		}
	}()

	// Отправляем несколько тестовых сообщений
	for i := 1; i <= 3; i++ {
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

	// Демонстрируем request-response паттерн
	log.Println("=== Testing Request-Response Pattern ===")

	// Отправляем запросы и ждем ответы
	for i := 1; i <= 3; i++ {
		request := RequestMessage{
			Question: fmt.Sprintf("What is %d + %d?", i, i),
			ID:       fmt.Sprintf("req-%d", i),
		}

		log.Printf("Sending request: %s", request.Question)

		// Отправляем запрос и ждем ответ (таймаут 5 секунд)
		responseBody, err := rabbitmq.PublishWithResponse(request, 5*time.Second)
		if err != nil {
			log.Printf("Failed to get response for request %d: %v", i, err)
		} else {
			log.Printf("Received response: %s", string(responseBody))
		}

		time.Sleep(1 * time.Second)
	}

	// Ждем обработки всех сообщений
	time.Sleep(5 * time.Second)

	log.Println("Example completed")
}
