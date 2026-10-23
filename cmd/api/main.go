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

	"github.com/redis/go-redis/v9"

	"github.com/shopally-ai/internal/adapter/gateway"
	"github.com/shopally-ai/internal/adapter/handler"
	"github.com/shopally-ai/internal/adapter/http/router"
	"github.com/shopally-ai/internal/adapter/repository"
	"github.com/shopally-ai/internal/config"
	"github.com/shopally-ai/internal/platform"
	"github.com/shopally-ai/pkg/usecase"
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

	ctx, stopDeps := context.WithCancel(context.Background())
	defer stopDeps()

	conns := platform.NewServiceConnections(
		cfg.Mongo.URI,
		cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.Password,
		cfg.Redis.DB, cfg.Redis.TLS,
	)
	defer func() {
		if err := conns.Close(); err != nil {
			log.Printf("dependency shutdown: %v", err)
		}
	}()

	alertRepo := repository.NewMongoAlertRepository(conns.MongoClient(), cfg.Mongo.Database, cfg.Mongo.AlertCollection)
	savedRepo := repository.NewMongoSavedRepository(conns.MongoClient(), cfg.Mongo.Database, cfg.Mongo.SavedCollection)

	switchableCache := gateway.NewSwitchableCache(nil)
	var lastRedis *redis.Client

	syncRuntime := func() {
		alertRepo.Rebind(conns.MongoClient(), cfg.Mongo.Database, cfg.Mongo.AlertCollection)
		savedRepo.Rebind(conns.MongoClient(), cfg.Mongo.Database, cfg.Mongo.SavedCollection)

		rc := conns.RedisClient()
		if rc == nil {
			return
		}
		if rc.Client == lastRedis {
			return
		}
		lastRedis = rc.Client
		switchableCache.Replace(gateway.NewRedisCache(rc.Client, cfg.Redis.KeyPrefix))
		log.Println("redis cache active")
	}

	conns.StartBackgroundRetry(ctx, 5*time.Second, syncRuntime)

	searchUC := usecase.NewSearchProductsUseCase(
		gateway.NewMockAlibabaGateway(),
		gateway.NewMockLLMGateway(),
		switchableCache,
	)

	fxClient := gateway.NewCachedFXClient(
		gateway.NewFXHTTPGateway(cfg.FX.APIURL, cfg.FX.APIKEY, nil),
		switchableCache,
		time.Duration(cfg.FX.CacheTTLSeconds)*time.Second,
	)

	alertManager := usecase.NewAlertManager(alertRepo)
	compareUC := usecase.NewCompareProductsUseCase(gateway.NewMockAlibabaGateway(), switchableCache)

	engine := router.Build(
		router.Deps{
			Health:  handler.NewHealthHandler(healthChecks(conns)),
			Search:  handler.NewSearchHandler(searchUC),
			FX:      handler.NewFXHandler(fxClient),
			Compare: handler.NewCompareHandler(compareUC),
			Alerts:  handler.NewAlertHandler(alertManager),
			Saved:   handler.NewSavedHandler(savedRepo),
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

	serveUntilSignal(srv, cfg.CORSOrigins(), stopDeps)
}

// healthChecks builds the dependency probes for /healthz. Each closure
// re-checks live, so a dependency that recovers is reported as healthy
// without needing a restart.
func healthChecks(conns *platform.ServiceConnections) map[string]handler.DependencyCheck {
	return map[string]handler.DependencyCheck{
		"mongo": conns.PingMongo,
		"redis": conns.PingRedis,
	}
}

// serveUntilSignal runs the server and drains it on SIGINT/SIGTERM so hosted
// deploys and container restarts do not sever in-flight requests.
func serveUntilSignal(srv *http.Server, origins []string, stopDeps context.CancelFunc) {
	errCh := make(chan error, 1)

	go func() {
		log.Printf("api listening on %s (cors allowlist: %v)", srv.Addr, origins)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	select {
	case err := <-errCh:
		log.Fatalf("server error: %v", err)
	case sig := <-stop:
		log.Printf("received %s, shutting down", sig)
	}

	stopDeps()

	ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		return
	}
	log.Println("shutdown complete")
}
