package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shopally-ai/internal/adapter/gateway"
	"github.com/shopally-ai/internal/adapter/handler"
	"github.com/shopally-ai/internal/adapter/http/router"
	"github.com/shopally-ai/internal/adapter/repository"
	"github.com/shopally-ai/internal/config"
	"github.com/shopally-ai/internal/platform"
	"github.com/shopally-ai/pkg/usecase"

	"go.mongodb.org/mongo-driver/mongo"
)

// shutdownGrace bounds how long in-flight requests get to finish.
const shutdownGrace = 10 * time.Second

func main() {
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Validated here rather than in LoadConfig: an empty secret is fine for a
	// local run that never touches a protected route, but shipping one to
	// production would leave /me and /alerts permanently 401.
	if len(cfg.Auth.JWTSecret) < handler.MinTokenSecretLength {
		if cfg.IsProduction() {
			log.Fatalf("auth.jwt_secret (AUTH_JWT_SECRET) must be at least %d characters; generate one with: openssl rand -base64 32",
				handler.MinTokenSecretLength)
		}
		log.Printf("WARNING: auth.jwt_secret (AUTH_JWT_SECRET) is unset or shorter than %d characters; requests to /me and /alerts will be rejected until it is set",
			handler.MinTokenSecretLength)
	}

	// Mongo and Redis are optional at startup. An outage degrades /healthz
	// rather than preventing the API from serving search and FX; saved-item
	// requests return a storage-unavailable response until Mongo recovers.
	mongoClient := connectMongo(cfg.Mongo.URI)
	defer func() {
		if mongoClient == nil {
			return
		}
		if err := platform.Disconnect(mongoClient); err != nil {
			log.Printf("mongo disconnect: %v", err)
		}
	}()

	redisClient := connectRedis(cfg)

	// A nil cache is a supported configuration: the use case falls back to
	// calling the gateways directly.
	var cache usecase.ICachePort
	if redisClient != nil {
		cache = gateway.NewRedisCache(redisClient.Client, cfg.Redis.KeyPrefix)
	}

	// Product and intent data come from mocks for now. Both sit behind the
	// gateway interfaces, so swapping in the real HTTP adapters is a change
	// to these two lines only.
	searchUC := usecase.NewSearchProductsUseCase(
		gateway.NewMockAlibabaGateway(),
		gateway.NewMockLLMGateway(),
		cache,
	)

	fxClient := gateway.NewCachedFXClient(
		gateway.NewFXHTTPGateway(cfg.FX.APIURL, cfg.FX.APIKEY, nil),
		cache,
		time.Duration(cfg.FX.CacheTTLSeconds)*time.Second,
	)

	alertManager := usecase.NewAlertManager(repository.NewMongoAlertRepository(mongoClient, cfg.Mongo.Database, cfg.Mongo.AlertCollection))
	savedRepository := repository.NewMongoSavedRepository(mongoClient, cfg.Mongo.Database, cfg.Mongo.SavedCollection)
	compareUC := usecase.NewCompareProductsUseCase(gateway.NewMockAlibabaGateway(), cache)

	engine := router.Build(
		router.Deps{
			Health:  handler.NewHealthHandler(healthChecks(mongoClient, redisClient)),
			Search:  handler.NewSearchHandler(searchUC),
			FX:      handler.NewFXHandler(fxClient),
			Compare: handler.NewCompareHandler(compareUC),
			Alerts:  handler.NewAlertHandler(alertManager),
			Saved:   handler.NewSavedHandler(savedRepository),
		},
		router.Options{
			BasePath:       "/api/v1",
			AllowedOrigins: cfg.CORSOrigins(),
			AuthSecret:     []byte(cfg.Auth.JWTSecret),
			Production:     cfg.IsProduction(),
		},
	)

	srv := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveUntilSignal(srv, cfg.CORSOrigins())
}

// connectMongo attempts a connection and returns nil on failure rather than
// exiting, so a Mongo outage cannot take the whole API down.
func connectMongo(uri string) *mongo.Client {
	client, err := platform.Connect(uri)
	if err != nil {
		log.Printf("mongo unavailable at startup, continuing without it: %v", err)
		return nil
	}
	log.Println("mongo connected")
	return client
}

// connectRedis pings Redis and returns nil if it is unreachable.
func connectRedis(cfg *config.Config) *platform.Client {
	rdb := platform.NewRedisClient(cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.Password, cfg.Redis.DB)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx); err != nil {
		log.Printf("redis unavailable at startup, continuing without cache: %v", err)
		return nil
	}
	log.Println("redis connected")
	return rdb
}

// healthChecks builds the dependency probes for /healthz. Each closure
// re-checks live, so a dependency that recovers is reported as healthy
// without needing a restart.
func healthChecks(mongoClient *mongo.Client, redisClient *platform.Client) map[string]handler.DependencyCheck {
	return map[string]handler.DependencyCheck{
		"mongo": func(ctx context.Context) error {
			if mongoClient == nil {
				return errors.New("not connected at startup")
			}
			return mongoClient.Ping(ctx, nil)
		},
		"redis": func(ctx context.Context) error {
			if redisClient == nil {
				return errors.New("not connected at startup")
			}
			return redisClient.Client.Ping(ctx).Err()
		},
	}
}

// serveUntilSignal runs the server and drains it on SIGINT/SIGTERM so hosted
// deploys and container restarts do not sever in-flight requests.
func serveUntilSignal(srv *http.Server, origins []string) {
	errCh := make(chan error, 1)

	go func() {
		log.Printf("api listening on %s (cors allowlist: %v)", srv.Addr, origins)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		log.Fatalf("server error: %v", err)
	case sig := <-stop:
		log.Printf("received %s, shutting down", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		return
	}
	log.Println("shutdown complete")
}
