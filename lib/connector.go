package rbtmqlib

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// connector управляет соединением с RabbitMQ
type connector struct {
	params           ConnectParams
	conn             *amqp091.Connection
	channel          *amqp091.Channel
	mu               sync.RWMutex
	isClosed         bool
	reconnectChan    chan struct{}
	reconnectMutex   sync.Mutex
	isReconnecting   bool
	connectionErrors chan error
	stopChan         chan struct{}
	wg               sync.WaitGroup
}

// newConnector создаёт новый экземпляр connector
func newConnector(params ConnectParams) (*connector, error) {
	if err := validateConnectParams(params); err != nil {
		return nil, fmt.Errorf("invalid connection parameters: %w", err)
	}

	connector := &connector{
		params:           params,
		reconnectChan:    make(chan struct{}, 1),
		connectionErrors: make(chan error, 10),
		stopChan:         make(chan struct{}),
	}

	applyDefaultConnectParams(&connector.params)

	if err := connector.connect(); err != nil {
		return nil, err
	}

	// Запускаем мониторинг соединения
	connector.startConnectionMonitor()

	return connector, nil
}

// connect устанавливает соединение с RabbitMQ
func (c *connector) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Закрываем существующие соединения
	c.closeConnections()

	url := fmt.Sprintf("amqp://%s:%s@%s:%d/", c.params.Username, c.params.Password, c.params.Host, c.params.Port)

	config := amqp091.Config{
		Heartbeat: time.Duration(c.params.Heartbeat) * time.Second,
		Dial:      amqp091.DefaultDial(c.params.ConnectionTimeout),
	}

	conn, err := amqp091.DialConfig(url, config)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	c.conn = conn

	if err := c.createChannel(); err != nil {
		c.conn.Close()
		c.conn = nil
		return fmt.Errorf("failed to create channel: %w", err)
	}

	c.isClosed = false
	log.Printf("Connected to RabbitMQ at %s:%d", c.params.Host, c.params.Port)
	return nil
}

// createChannel создает канал для соединения
func (c *connector) createChannel() error {
	if c.conn == nil {
		return fmt.Errorf("connection is nil")
	}

	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to create channel: %w", err)
	}

	c.channel = ch
	return nil
}

// getChannel возвращает канал соединения с автоматическим переподключением
func (c *connector) getChannel() *amqp091.Channel {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.isClosed {
		return nil
	}

	// Проверяем состояние канала
	if c.channel != nil && !c.channel.IsClosed() {
		return c.channel
	}

	// Если канал закрыт, запускаем переподключение
	if c.channel != nil && c.channel.IsClosed() {
		log.Printf("Channel is closed, triggering reconnection...")
		c.triggerReconnect()
		return nil
	}

	return c.channel
}

// triggerReconnect запускает процесс переподключения
func (c *connector) triggerReconnect() {
	c.reconnectMutex.Lock()
	defer c.reconnectMutex.Unlock()

	if c.isReconnecting {
		return
	}

	c.isReconnecting = true
	select {
	case c.reconnectChan <- struct{}{}:
	default:
	}
}

// startConnectionMonitor запускает мониторинг состояния соединения
func (c *connector) startConnectionMonitor() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.monitorConnection()
	}()
}

// monitorConnection мониторит состояние соединения и выполняет переподключение
func (c *connector) monitorConnection() {
	for {
		select {
		case <-c.reconnectChan:
			c.performReconnect()
		case <-c.stopChan:
			return
		}
	}
}

// performReconnect выполняет переподключение
func (c *connector) performReconnect() {
	c.reconnectMutex.Lock()
	defer c.reconnectMutex.Unlock()

	if !c.isReconnecting {
		return
	}

	log.Printf("Performing reconnection to RabbitMQ...")

	// Попытки переподключения с экспоненциальной задержкой
	maxRetries := 5
	baseDelay := time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := c.connect(); err != nil {
			delay := time.Duration(attempt+1) * baseDelay
			log.Printf("Reconnection attempt %d failed: %v, retrying in %v...", attempt+1, err, delay)
			time.Sleep(delay)
			continue
		}

		log.Printf("Successfully reconnected to RabbitMQ")
		c.isReconnecting = false
		return
	}

	log.Printf("Failed to reconnect after %d attempts", maxRetries)
	c.isReconnecting = false
}

// closeConnections закрывает соединения без блокировки
func (c *connector) closeConnections() {
	if c.channel != nil {
		c.channel.Close()
		c.channel = nil
	}

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// close закрывает соединение
func (c *connector) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isClosed {
		return
	}

	log.Printf("Closing connector...")
	c.isClosed = true

	// Останавливаем мониторинг
	close(c.stopChan)

	c.closeConnections()

	log.Printf("Connector closed")
}

// waitForShutdown ждет завершения работы connector
func (c *connector) waitForShutdown() {
	c.wg.Wait()
}
