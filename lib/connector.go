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
	params   ConnectParams
	conn     *amqp091.Connection
	channel  *amqp091.Channel
	mu       sync.RWMutex
	isClosed bool
}

// newConnector создаёт новый экземпляр connector
func newConnector(params ConnectParams) (*connector, error) {
	if err := validateConnectParams(params); err != nil {
		return nil, fmt.Errorf("invalid connection parameters: %w", err)
	}

	connector := &connector{
		params: params,
	}

	applyDefaultConnectParams(&connector.params)

	if err := connector.connect(); err != nil {
		return nil, err
	}

	return connector, nil
}

// connect устанавливает соединение с RabbitMQ
func (c *connector) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
	}
	if c.channel != nil {
		c.channel.Close()
	}

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

// getChannel возвращает канал соединения
func (c *connector) getChannel() *amqp091.Channel {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.channel == nil || c.isClosed {
		return nil
	}

	return c.channel
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

	if c.channel != nil {
		c.channel.Close()
		c.channel = nil
	}

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	log.Printf("Connector closed")
}
