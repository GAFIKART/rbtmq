package rbtmqlib

import "time"

// Константы по умолчанию
const (
	DefaultPrefetchCount = 10
	MaxMessageSize       = 10 * 1024 * 1024 // максимальный размер сообщения (10MB)
)

// Настраиваемые константы
var (
	DefaultConnectionTimeout = 30 * time.Second
	DefaultHeartbeat         = 30
)

// applyDefaultConnectParams применяет дефолтные значения к параметрам подключения
func applyDefaultConnectParams(params *ConnectParams) {
	if params.ConnectionTimeout == 0 {
		params.ConnectionTimeout = DefaultConnectionTimeout
	}
	if params.Heartbeat == 0 {
		params.Heartbeat = DefaultHeartbeat
	}
}

// applyDefaultConsumerParams применяет дефолтные значения к параметрам consumer
func applyDefaultConsumerParams(params *ConsumerParams) {
	applyDefaultConnectParams(&params.ConnectParams)
	if params.PrefetchCount == 0 {
		params.PrefetchCount = DefaultPrefetchCount
	}
}

// applyDefaultPublisherParams применяет дефолтные значения к параметрам publisher
func applyDefaultPublisherParams(params *PublisherParams) {
	applyDefaultConnectParams(&params.ConnectParams)
}
