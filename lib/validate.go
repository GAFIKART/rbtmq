package rbtmqlib

import (
	"fmt"
)

// validateConnectParams валидирует параметры подключения
func validateConnectParams(params ConnectParams) error {
	if params.Username == "" {
		return fmt.Errorf("username is required")
	}
	if params.Password == "" {
		return fmt.Errorf("password is required")
	}
	if params.Host == "" {
		return fmt.Errorf("host is required")
	}
	if params.Port <= 0 || params.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

// validateConsumerParams валидирует параметры consumer
func validateConsumerParams(params ConsumerParams) error {
	if err := validateConnectParams(params.ConnectParams); err != nil {
		return err
	}
	if params.RoutingKey == "" {
		return fmt.Errorf("routingKey is required")
	}
	return nil
}

// validatePublisherParams валидирует параметры Publisher
func validatePublisherParams(params PublisherParams) error {
	if err := validateConnectParams(params.ConnectParams); err != nil {
		return err
	}
	if params.RoutingKey == "" {
		return fmt.Errorf("routingKey is required")
	}
	return nil
}

// validateMessageSize валидирует размер сообщения
func validateMessageSize(body []byte) error {
	if len(body) > MaxMessageSize {
		return fmt.Errorf("message size %d exceeds maximum allowed size %d", len(body), MaxMessageSize)
	}
	return nil
}
