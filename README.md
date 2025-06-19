# RBTMQ - Простая библиотека для RabbitMQ

Простая, надежная и эффективная библиотека для работы с RabbitMQ в Go. Создана специально для микросервисной архитектуры с независимыми слушателями и отправщиками сообщений.

## 🚀 Особенности

- **Простота** - минимальный API, только необходимые функции
- **Надежность** - автоматическое переподключение и обработка ошибок
- **Эффективность** - один connector для publisher и consumer
- **Микросервисы** - идеально подходит для независимых сервисов
- **Производительность** - оптимизирована для высоких нагрузок
- **Простота использования** - всего 5 основных функций

## 📦 Установка

```bash
go get github.com/GAFIKART/rbtmq/lib
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

## 🏗️ Архитектура

### Микросервисная архитектура

Библиотека идеально подходит для микросервисной архитектуры:

```go
// Сервис A - отправляет сообщения
configA := rbtmqlib.RabbitMQConfig{
    ConnectParams: rbtmqlib.ConnectParams{
        Username: "user", Password: "pass", Host: "localhost", Port: 5672,
    },
    RoutingKey: "orders.new",
}
rabbitmqA, _ := rbtmqlib.NewRabbitMQ(configA)
rabbitmqA.Publish(Order{ID: "123", Amount: 100})

// Сервис B - получает сообщения
configB := rbtmqlib.RabbitMQConfig{
    ConnectParams: rbtmqlib.ConnectParams{
        Username: "user", Password: "pass", Host: "localhost", Port: 5672,
    },
    RoutingKey: "orders.new",
}
rabbitmqB, _ := rbtmqlib.NewRabbitMQ(configB)
rabbitmqB.StartConsuming()
messages := rabbitmqB.GetMessages()
```

### Независимые слушатели

Каждый микросервис может независимо слушать свои сообщения:

```go
// Сервис уведомлений
notificationConsumer := rbtmqlib.NewRabbitMQ(rbtmqlib.RabbitMQConfig{
    ConnectParams: connectParams,
    RoutingKey: "notifications.email",
})

// Сервис аналитики  
analyticsConsumer := rbtmqlib.NewRabbitMQ(rbtmqlib.RabbitMQConfig{
    ConnectParams: connectParams,
    RoutingKey: "analytics.events",
})
```

## ⚡ Производительность

### Настройки по умолчанию

```go
const (
    DefaultPrefetchCount = 10                    // Сообщений без подтверждения
    MaxMessageSize       = 10 * 1024 * 1024      // Максимальный размер (10MB)
)

var (
    DefaultConnectionTimeout = 30 * time.Second   // Таймаут подключения
    DefaultHeartbeat        = 30                 // Интервал heartbeat
)
```

### Оптимизация

- **Один connector** на экземпляр RabbitMQ
- **Небуферизованный канал** сообщений (использует буферизацию RabbitMQ)
- **Автоматическое переподключение** при разрыве соединения
- **Минимальные блокировки** для высокой производительности

## 🔒 Надежность

### Автоматическое переподключение

Библиотека автоматически обрабатывает разрывы соединения:

```go
// При разрыве соединения библиотека автоматически:
// 1. Обнаруживает разрыв
// 2. Пытается переподключиться
// 3. Восстанавливает работу
```

### Обработка ошибок

```go
// Отправка сообщений
if err := rabbitmq.Publish(message); err != nil {
    log.Printf("Failed to publish: %v", err)
    // Обработка ошибки
}

// Получение сообщений
for msg := range messages {
    if err := processMessage(msg); err != nil {
        msg.Nack(true)  // Вернуть в очередь для повторной обработки
    } else {
        msg.Ack()       // Подтвердить успешную обработку
    }
}
```

## 📋 Примеры использования

### Отправка сообщений

```go
// Простая отправка
message := map[string]interface{}{
    "type": "order_created",
    "data": map[string]interface{}{
        "order_id": "123",
        "amount": 100.50,
    },
}
err := rabbitmq.Publish(message)

// Отправка структуры
type OrderEvent struct {
    Type      string    `json:"type"`
    OrderID   string    `json:"order_id"`
    Amount    float64   `json:"amount"`
    Timestamp time.Time `json:"timestamp"`
}

event := OrderEvent{
    Type:      "order_created",
    OrderID:   "123",
    Amount:    100.50,
    Timestamp: time.Now(),
}
err := rabbitmq.Publish(event)
```

### Получение сообщений

```go
// Простая обработка
for msg := range messages {
    log.Printf("Received: %s", string(msg.OriginalMessage.Body))
    msg.Ack()
}

// Обработка с десериализацией
type MyMessage struct {
    ID   string `json:"id"`
    Data string `json:"data"`
}

for msg := range messages {
    var message MyMessage
    if err := msg.UnmarshalBody(&message); err != nil {
        log.Printf("Failed to unmarshal: %v", err)
        msg.Nack(true)  // Вернуть в очередь
        continue
    }
    
    log.Printf("Processed: ID=%s, Data=%s", message.ID, message.Data)
    msg.Ack()
}
```

### Graceful Shutdown

```go
// Создание контекста с таймаутом
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Корректное завершение
if err := rabbitmq.Shutdown(ctx); err != nil {
    log.Printf("Shutdown error: %v", err)
}
```

## 🐳 Docker

### Запуск RabbitMQ

```bash
docker run -d --name rabbitmq \
  -p 5672:5672 \
  -p 15672:15672 \
  rabbitmq:3-management
```

### Подключение

```go
config := rbtmqlib.RabbitMQConfig{
    ConnectParams: rbtmqlib.ConnectParams{
        Username: "guest",
        Password: "guest",
        Host:     "localhost",
        Port:     5672,
    },
    RoutingKey: "my.queue",
}
```

## 🧪 Тестирование

### Запуск примера

```bash
cd example
go run main.go
```

### Проверка работы

1. Запустите RabbitMQ
2. Запустите пример
3. Проверьте логи - должны появиться сообщения об отправке и получении

## 📝 Логирование

Библиотека использует стандартный `log` пакет для логирования:

```
2024/01/01 12:00:00 Connected to RabbitMQ at localhost:5672
2024/01/01 12:00:01 Published message 1
2024/01/01 12:00:01 Received message: {"id":"msg-1","content":"Test message 1","time":"2024-01-01T12:00:01Z"}
2024/01/01 12:00:01 Processed message: ID=msg-1, Content=Test message 1
```

## 🆘 Поддержка

Если у вас есть вопросы или проблемы:

1. Проверьте [Issues](../../issues)
2. Создайте новый Issue с описанием проблемы
3. Укажите версию Go и RabbitMQ

