package redis

import (
	"context"
	"fmt"

	"github.com/redis/rueidis"
)

// NewClient builds a rueidis client with sane defaults for pub-sub +
// general use. rueidis auto-pipelines and auto-reconnects.
func NewClient(addr string) (rueidis.Client, error) {
	c, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{addr},
		// We're using the client for both PSUBSCRIBE (long-lived) and PUBLISH
		// (request/response). rueidis's pipelining handles this transparently.
	})
	if err != nil {
		return nil, fmt.Errorf("new rueidis client: %w", err)
	}
	if err := c.Do(context.Background(), c.B().Ping().Build()).Error(); err != nil {
		c.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return c, nil
}
