// Package redisclient replica redis_client.py: una unica instancia de
// go-redis compartida en toda la aplicacion.
package redisclient

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

var _client *redis.Client

// Get retorna la instancia global del cliente Redis.
func Get() *redis.Client {
	if _client == nil {
		panic("Redis not initialized. Call Init() from main first.")
	}
	return _client
}

// Init crea el cliente Redis global a partir de un DSN.
func Init(dsn string) (*redis.Client, error) {
	opts, err := redis.ParseURL(dsn)
	if err != nil {
		return nil, err
	}
	_client = redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := _client.Ping(ctx).Err(); err != nil {
		_client.Close()
		_client = nil
		return nil, err
	}
	return _client, nil
}

// Close cierra el cliente Redis global.
func Close() {
	if _client != nil {
		_ = _client.Close()
		_client = nil
	}
}