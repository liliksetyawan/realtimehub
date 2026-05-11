// RealtimeHub server.
//
// Boot sequence: load config → init logger → connect postgres + run
// migrations → connect redis + start subscriber → wire JWT auth, hub,
// use cases, REST routes on a single nbio engine → block until
// SIGINT/SIGTERM → drain.
package main

import (
	"context"
	"fmt"
	stdhttp "net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"

	"github.com/liliksetyawan/realtimehub/internal/adapter/auth"
	httpadapter "github.com/liliksetyawan/realtimehub/internal/adapter/http"
	"github.com/liliksetyawan/realtimehub/internal/adapter/postgres"
	redisadapter "github.com/liliksetyawan/realtimehub/internal/adapter/redis"
	wsadapter "github.com/liliksetyawan/realtimehub/internal/adapter/websocket"
	"github.com/liliksetyawan/realtimehub/internal/app/usecase"
	"github.com/liliksetyawan/realtimehub/internal/config"
	"github.com/liliksetyawan/realtimehub/internal/observability"
	"github.com/liliksetyawan/realtimehub/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := zerolog.New(os.Stdout).With().
		Timestamp().Str("service", "realtimehub").Logger()

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Tracing ---
	shutdownTracing, err := observability.InitTracing(rootCtx, "realtimehub", cfg.OTLPEndpoint)
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(ctx)
	}()
	if cfg.OTLPEndpoint != "" {
		logger.Info().Str("endpoint", cfg.OTLPEndpoint).Msg("tracing enabled")
	}

	// --- Postgres + migrations ---
	pool, err := postgres.Connect(rootCtx, postgres.Config{
		Host: cfg.PostgresHost, Port: cfg.PostgresPort,
		User: cfg.PostgresUser, Password: cfg.PostgresPassword,
		Database: cfg.PostgresDB,
	})
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer pool.Close()
	if err := postgres.Migrate(rootCtx, pool, migrations.FS); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	logger.Info().Msg("migrations up to date")

	// --- Auth ---
	jwtAuth := auth.NewJWT(cfg.JWTSecret, cfg.JWTTTL)

	// History adapter feeds the WS server CurrentSeq + SinceSeq so the
	// welcome message and resume replay both work.
	notifRepo := postgres.NewNotificationRepo(pool)
	history := postgres.NewHistoryAdapter(notifRepo)

	// --- WebSocket server (epoll engine) ---
	server := wsadapter.NewServer(wsadapter.ServerConfig{
		Addr:         cfg.HTTPAddr,
		WriteBuffer:  cfg.WriteBuffer,
		PingInterval: cfg.PingInterval,
		PongTimeout:  cfg.PongTimeout,
		HubShards:    cfg.HubShards,
	}, jwtAuth, history, logger)

	// --- Redis (cross-node fan-out) ---
	redisClient, err := redisadapter.NewClient(cfg.RedisAddr)
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	defer redisClient.Close()

	// Every node both publishes and subscribes — even single-node deploys
	// route through Redis, so multi-node deploys need no code changes.
	subscriber := redisadapter.NewSubscriber(redisClient, server.Hub(), logger)
	if err := subscriber.Start(rootCtx); err != nil {
		return fmt.Errorf("redis subscribe: %w", err)
	}

	// --- Use cases ---
	publisher := redisadapter.NewPublisher(redisClient, logger)
	sendUC := usecase.NewSendNotification(notifRepo, publisher, logger)
	listUC := usecase.NewListNotifications(notifRepo)
	markUC := usecase.NewMarkRead(notifRepo)

	// --- REST routes on the nbio mux ---
	authH := httpadapter.NewAuthHandler(jwtAuth, logger)
	notifH := httpadapter.NewNotificationHandler(sendUC, listUC, markUC, logger)
	usersH := httpadapter.NewUsersHandler()
	mux := server.Mux()
	cors := func(h stdhttp.Handler) stdhttp.Handler { return httpadapter.CORS(cfg.CORSOrigins, h) }

	// Public
	mux.Handle("/v1/auth/login", cors(stdhttp.HandlerFunc(authH.Login)))
	mux.Handle("/v1/me", cors(stdhttp.HandlerFunc(authH.Me)))

	// Authenticated
	mux.Handle("/v1/notifications", cors(httpadapter.RequireAuth(jwtAuth, stdhttp.HandlerFunc(notifH.ListMine))))
	mux.Handle("/v1/notifications/", cors(httpadapter.RequireAuth(jwtAuth, stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if !strings.HasSuffix(r.URL.Path, "/read") {
			stdhttp.NotFound(w, r)
			return
		}
		notifH.MarkRead(w, r)
	}))))

	// Admin
	mux.Handle("/v1/admin/notifications", cors(httpadapter.RequireAdmin(jwtAuth, stdhttp.HandlerFunc(notifH.AdminSend))))
	mux.Handle("/v1/admin/users", cors(httpadapter.RequireAdmin(jwtAuth, stdhttp.HandlerFunc(usersH.List))))

	if err := server.Start(rootCtx); err != nil {
		return fmt.Errorf("server start: %w", err)
	}
	logger.Info().Str("addr", cfg.HTTPAddr).Int("shards", cfg.HubShards).Msg("realtimehub up")

	<-rootCtx.Done()
	logger.Info().Msg("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Stop(shutdownCtx)
	logger.Info().Msg("bye")
	return nil
}
