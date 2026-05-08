// Package config loads service configuration from env vars + .env file.
package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	HTTPAddr string `envconfig:"HTTP_ADDR" default:":8090"`

	JWTSecret string        `envconfig:"JWT_SECRET" default:"dev-only-change-me"`
	JWTTTL    time.Duration `envconfig:"JWT_TTL" default:"24h"`

	PostgresHost     string `envconfig:"POSTGRES_HOST" default:"localhost"`
	PostgresPort     int    `envconfig:"POSTGRES_PORT" default:"5433"`
	PostgresUser     string `envconfig:"POSTGRES_USER" default:"realtimehub"`
	PostgresPassword string `envconfig:"POSTGRES_PASSWORD" default:"realtimehub"`
	PostgresDB       string `envconfig:"POSTGRES_DB" default:"realtimehub"`

	RedisAddr string `envconfig:"REDIS_ADDR" default:"localhost:6380"`

	OTLPEndpoint string `envconfig:"OTEL_EXPORTER_OTLP_ENDPOINT" default:""`

	HubShards    int           `envconfig:"HUB_SHARDS" default:"16"`
	WriteBuffer  int           `envconfig:"WRITE_BUFFER" default:"64"`
	PingInterval time.Duration `envconfig:"PING_INTERVAL" default:"25s"`
	PongTimeout  time.Duration `envconfig:"PONG_TIMEOUT" default:"60s"`

	CORSOrigins []string `envconfig:"CORS_ORIGINS" default:"http://localhost:5173,http://localhost:4173"`
}

func Load() (Config, error) {
	var c Config
	err := envconfig.Process("", &c)
	return c, err
}
