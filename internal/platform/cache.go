package platform

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	*redis.Client
}

func NewRedisClient(host, port, password string, db int, useTLS ...bool) *Client {
	options := &redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       db,
	}
	if len(useTLS) > 0 && useTLS[0] {
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return &Client{
		redis.NewClient(options),
	}
}

func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.Client.Ping(ctx).Err()
}
