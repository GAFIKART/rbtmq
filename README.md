# RBTMQ - Простая библиотека для RabbitMQ

Простая, надежная и эффективная библиотека для работы с RabbitMQ в Go. Создана специально для микросервисной архитектуры с независимыми слушателями и отправщиками сообщений.

## 🚀 Особенности

- **Простота** - минимальный API, только необходимые функции
- **Надежность** - автоматическое переподключение и обработка ошибок
- **Эффективность** - один connector для publisher и consumer
- **Микросервисы** - идеально подходит для независимых сервисов
- **Производительность** - оптимизирована для высоких нагрузок
- **Request-Response** - поддержка паттерна запрос-ответ
- **Простота использования** - всего 7 основных функций

## 📦 Установка

```bash
go get github.com/GAFIKART/rbtmq
```

## 🔧 Быстрый старт

### Простой пример

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    rbtmqlib "github.com/GAFIKART/rbtmq/lib"
)

// Message структура сообщения
type Message struct {
    ID      string    `json:"id"`
    Content string    `json:"content"`
    Time    time.Time `json:"time"`
}

func main() {
    // Конфигурация
    config := rbtmqlib.RabbitMQConfig{
        ConnectParams: rbtmqlib.ConnectParams{
            Username: "guest",
            Password: "guest",
            Host:     "localhost",
            Port:     5672,
        },
        RoutingKey: "test.messages",
    }

    // Создание экземпляра
    rabbitmq, err := rbtmqlib.NewRabbitMQ(config)
    if err != nil {
        log.Fatalf("Failed to create RabbitMQ: %v", err)
    }
    defer rabbitmq.Shutdown(context.Background())

    // Запуск слушателя
    if err := rabbitmq.StartConsuming(); err != nil {
        log.Fatalf("Failed to start consuming: %v", err)
    }

    // Получение сообщений
    messages := rabbitmq.GetMessages()

    // Обработка сообщений
    go func() {
        for msg := range messages {
            var message Message
            if err := msg.UnmarshalBody(&message); err != nil {
                log.Printf("Failed to unmarshal: %v", err)
                msg.Nack(true)
                continue
            }

            log.Printf("Processed: ID=%s, Content=%s", message.ID, message.Content)
            msg.Ack()
        }
    }()

    // Отправка сообщений
    for i := 1; i <= 5; i++ {
        message := Message{
            ID:      fmt.Sprintf("msg-%d", i),
            Content: fmt.Sprintf("Test message %d", i),
            Time:    time.Now(),
        }

        if err := rabbitmq.Publish(message); err != nil {
            log.Printf("Failed to publish: %v", err)
        } else {
            log.Printf("Published message %d", i)
        }
    }

    time.Sleep(10 * time.Second)
}
```

### Request-Response паттерн

```go
// Отправка запроса и ожидание ответа
request := RequestMessage{Question: "What is 2+2?", ID: "req-1"}
responseBody, err := rabbitmq.PublishWithResponse(request, 10*time.Second)
if err != nil {
    log.Printf("Failed to get response: %v", err)
} else {
    log.Printf("Response: %s", string(responseBody))
}

// Обработка запросов и отправка ответов
go func() {
    for msg := range messages {
        if msg.OriginalMessage.ReplyTo != "" {
            // Это запрос, требующий ответа
            var request RequestMessage
            if err := msg.UnmarshalBody(&request); err != nil {
                msg.Nack(true)
                continue
            }

            // Создаем ответ
            response := ResponseMessage{
                Answer: fmt.Sprintf("Answer to: %s", request.Question),
                ID:     request.ID,
            }

            // Отправляем ответ
            if err := rabbitmq.Respond(msg, response); err != nil {
                log.Printf("Failed to send response: %v", err)
            }

            msg.Ack()
        }
    }
}()
```

## 📚 API Документация

### Основные типы

#### ConnectParams
```go
type ConnectParams struct {
    Username string        // Имя пользователя
    Password string        // Пароль
    Host     string        // Хост RabbitMQ
    Port     int           // Порт (обычно 5672)
    Heartbeat int          // Интервал heartbeat в секундах
    ConnectionTimeout time.Duration // Таймаут подключения
}
```

#### RabbitMQConfig
```go
type RabbitMQConfig struct {
    ConnectParams         // Параметры подключения
    RoutingKey string     // Routing key для обмена сообщениями
}
```

#### DeliveryMessage
```go
type DeliveryMessage struct {
    OriginalMessage amqp091.Delivery // Оригинальное сообщение RabbitMQ
    ReceivedAt      time.Time        // Время получения
}
```

### Основные функции

#### NewRabbitMQ(config)
Создает новый экземпляр RabbitMQ с общим connector для publisher и consumer.

```go
rabbitmq, err := rbtmqlib.NewRabbitMQ(config)
if err != nil {
    log.Fatal(err)
}
```

#### Publish(msg)
Отправляет сообщение в RabbitMQ. Сообщение автоматически сериализуется в JSON.

```go
message := MyMessage{ID: "123", Data: "test"}
err := rabbitmq.Publish(message)
```

#### PublishWithResponse(msg, timeout...)
Отправляет сообщение и ожидает ответ. Поддерживает опциональный таймаут (по умолчанию 30 секунд).

```go
// С таймаутом по умолчанию (30 секунд)
response, err := rabbitmq.PublishWithResponse(request)

// С указанным таймаутом
response, err := rabbitmq.PublishWithResponse(request, 10*time.Second)
```

#### StartConsuming()
Запускает слушатель сообщений.

```go
err := rabbitmq.StartConsuming()
```

#### GetMessages()
Возвращает канал для получения сообщений.

```go
messages := rabbitmq.GetMessages()
for msg := range messages {
    // Обработка сообщения
}
```

#### Respond(originalDelivery, responsePayload)
Отправляет ответ на полученное сообщение (для request-response паттерна).

```go
response := ResponseMessage{Answer: "42"}
err := rabbitmq.Respond(originalMessage, response)
```

#### Shutdown(ctx)
Корректно завершает работу RabbitMQ.

```go
err := rabbitmq.Shutdown(context.Background())
```

### Методы DeliveryMessage

#### Ack()
Подтверждает успешную обработку сообщения.

```go
msg.Ack()
```

#### Nack(requeue)
Отклоняет сообщение. `requeue=true` возвращает сообщение в очередь.

```go
msg.Nack(true)  // Вернуть в очередь
msg.Nack(false) // Удалить из очереди
```

#### UnmarshalBody(v)
Десериализует тело сообщения в структуру.

```go
var message MyMessage
err := msg.UnmarshalBody(&message)
```

## 🔄 Request-Response паттерн

Библиотека поддерживает паттерн запрос-ответ, который позволяет:

1. **Отправлять запросы** с помощью `PublishWithResponse()`
2. **Обрабатывать запросы** в consumer и отправлять ответы через `Respond()`
3. **Получать ответы** автоматически в том же вызове

### Как это работает:

1. **Отправитель** вызывает `PublishWithResponse()` с сообщением
2. Библиотека создает временную очередь для ответов
3. Сообщение отправляется с заголовками `CorrelationId` и `ReplyTo`
4. **Получатель** обрабатывает сообщение и вызывает `Respond()`
5. Ответ отправляется в временную очередь отправителя
6. **Отправитель** получает ответ и временная очередь удаляется

### Преимущества:

- **Простота** - один вызов для отправки и получения ответа
- **Автоматическое управление** - временные очереди создаются и удаляются автоматически
- **Таймауты** - защита от бесконечного ожидания
- **Корреляция** - автоматическое сопоставление запросов и ответов

## ⚙️ Конфигурация

### Параметры по умолчанию

```go
DefaultConnectionTimeout = 30 * time.Second
DefaultHeartbeat         = 30
DefaultPrefetchCount     = 10
MaxMessageSize           = 10 * 1024 * 1024 // 10MB
```

### Настройка параметров

```go
config := rbtmqlib.RabbitMQConfig{
    ConnectParams: rbtmqlib.ConnectParams{
        Username:           "guest",
        Password:           "guest",
        Host:               "localhost",
        Port:               5672,
        Heartbeat:          60,                    // 60 секунд
        ConnectionTimeout:   60 * time.Second,      // 60 секунд
    },
    RoutingKey: "my.service.messages",
}
```

## 🚀 Производительность

- **Один connector** для publisher и consumer
- **Автоматическое переподключение** при потере соединения
- **Эффективная сериализация** JSON
- **Оптимизированные очереди** для request-response
- **Минимальные накладные расходы**

## 🔧 Примеры использования

### Микросервис обработки заказов

```go
// Сервис заказов
orderService := rbtmqlib.NewRabbitMQ(orderConfig)
defer orderService.Shutdown(context.Background())

// Обработка заказов
go func() {
    for msg := range orderService.GetMessages() {
        var order Order
        if err := msg.UnmarshalBody(&order); err != nil {
            msg.Nack(true)
            continue
        }

        // Обработка заказа
        result := processOrder(order)
        msg.Ack()
    }
}()

// Отправка заказа
order := Order{ID: "123", Items: []string{"item1", "item2"}}
err := orderService.Publish(order)
```

### Request-Response для API

```go
// API сервис
apiService := rbtmqlib.NewRabbitMQ(apiConfig)

// Обработка API запросов
go func() {
    for msg := range apiService.GetMessages() {
        if msg.OriginalMessage.ReplyTo != "" {
            var request APIRequest
            if err := msg.UnmarshalBody(&request); err != nil {
                msg.Nack(true)
                continue
            }

            // Обработка запроса
            response := handleAPIRequest(request)
            
            // Отправка ответа
            apiService.Respond(msg, response)
            msg.Ack()
        }
    }
}()

// Клиент API
request := APIRequest{Method: "GET", Path: "/users/123"}
response, err := apiService.PublishWithResponse(request, 5*time.Second)
```

## 📝 Лицензия

MIT License

