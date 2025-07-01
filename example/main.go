package main

import (
	"context"
	"encoding/json"
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

// UserData пример структуры для пользовательских данных
type UserData struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Active   bool   `json:"active"`
}

// Библиотека предоставляет гибкие способы работы с JSON:
// 1. msg.GetBodyAsString() - получаете JSON как строку
// 2. msg.GetBodyAsBytes() - получаете JSON как байты
// 3. msg.UnmarshalBody(&struct) - парсите в структуру
// 4. json.Unmarshal(msg.GetBodyAsBytes(), &struct) - используйте любую JSON библиотеку

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
			// Получаем сырой JSON как строку
			rawJSON := msg.GetBodyAsString()
			log.Printf("📨 Received raw JSON: %s", rawJSON)

			// Проверяем, является ли это запросом (есть ли ReplyTo)
			if msg.OriginalMessage.ReplyTo != "" {
				// Обрабатываем запрос и отправляем ответ
				var request RequestMessage
				if err := msg.UnmarshalBody(&request); err != nil {
					log.Printf("❌ Failed to unmarshal request: %v", err)
					msg.Nack(true)
					continue
				}

				log.Printf("🤔 Received request: %s", request.Question)

				// Создаем ответ
				response := ResponseMessage{
					Answer: fmt.Sprintf("Answer to: %s", request.Question),
					ID:     request.ID,
				}

				// Отправляем ответ
				if err := rabbitmq.Respond(msg, response); err != nil {
					log.Printf("❌ Failed to send response: %v", err)
				} else {
					log.Printf("✅ Sent response for request ID: %s", request.ID)
				}

				msg.Ack()
			} else {
				// Обрабатываем обычные сообщения
				processMessage(msg)
			}
		}
	}()

	// Отправляем тестовые сообщения
	sendTestMessages(rabbitmq)

	// Демонстрируем request-response паттерн
	testRequestResponse(rabbitmq)

	// Ждем обработки всех сообщений
	time.Sleep(5 * time.Second)
	log.Println("🎉 Example completed")
}

// processMessage обрабатывает обычные сообщения разными способами
func processMessage(msg *rbtmqlib.DeliveryMessage) {
	log.Println("🔄 Processing message...")

	// Способ 1: Используем встроенный метод для парсинга в структуру
	var message Message
	if err := msg.UnmarshalBody(&message); err == nil {
		log.Printf("✅ Parsed as Message: ID=%s, Content=%s, Time=%v",
			message.ID, message.Content, message.Time)
		msg.Ack()
		return
	}

	// Способ 2: Пробуем парсить как пользовательские данные
	var userData UserData
	if err := msg.UnmarshalBody(&userData); err == nil {
		log.Printf("✅ Parsed as UserData: ID=%d, Username=%s, Email=%s, Active=%v",
			userData.UserID, userData.Username, userData.Email, userData.Active)
		msg.Ack()
		return
	}

	// Способ 3: Используем стандартную библиотеку JSON напрямую
	var customMessage Message
	if err := json.Unmarshal(msg.GetBodyAsBytes(), &customMessage); err == nil {
		log.Printf("✅ Parsed with standard library: ID=%s, Content=%s",
			customMessage.ID, customMessage.Content)
		msg.Ack()
		return
	}

	// Если не удалось распарсить ни одним способом
	log.Printf("❌ Failed to parse message with any method")
	log.Printf("📄 Raw message: %s", msg.GetBodyAsString())
	msg.Nack(true) // Возвращаем в очередь
}

// sendTestMessages отправляет тестовые сообщения
func sendTestMessages(rabbitmq *rbtmqlib.RabbitMQ) {
	log.Println("📤 Sending test messages...")

	// Отправляем обычные сообщения
	for i := 1; i <= 2; i++ {
		message := Message{
			ID:      fmt.Sprintf("msg-%d", i),
			Content: fmt.Sprintf("Test message %d", i),
			Time:    time.Now(),
		}

		if err := rabbitmq.Publish(message); err != nil {
			log.Printf("❌ Failed to publish message %d: %v", i, err)
		} else {
			log.Printf("✅ Published Message %d", i)
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Отправляем пользовательские данные
	for i := 1; i <= 2; i++ {
		userData := UserData{
			UserID:   i,
			Username: fmt.Sprintf("user%d", i),
			Email:    fmt.Sprintf("user%d@example.com", i),
			Active:   i%2 == 0,
		}

		if err := rabbitmq.Publish(userData); err != nil {
			log.Printf("❌ Failed to publish user data %d: %v", i, err)
		} else {
			log.Printf("✅ Published UserData %d", i)
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// testRequestResponse демонстрирует request-response паттерн
func testRequestResponse(rabbitmq *rbtmqlib.RabbitMQ) {
	log.Println("🔄 Testing Request-Response Pattern...")

	for i := 1; i <= 2; i++ {
		request := RequestMessage{
			Question: fmt.Sprintf("What is %d + %d?", i, i),
			ID:       fmt.Sprintf("req-%d", i),
		}

		log.Printf("📤 Sending request: %s", request.Question)

		// Отправляем запрос и ждем ответ (таймаут 5 секунд)
		responseBody, err := rabbitmq.PublishWithResponse(request, 5*time.Second)
		if err != nil {
			log.Printf("❌ Failed to get response for request %d: %v", i, err)
		} else {
			log.Printf("📥 Received response: %s", string(responseBody))
		}

		time.Sleep(1 * time.Second)
	}
}
