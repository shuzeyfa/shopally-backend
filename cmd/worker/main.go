package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shopally-ai/internal/adapter/gateway"
	"github.com/shopally-ai/internal/adapter/repository"
	"github.com/shopally-ai/internal/config"
	"github.com/shopally-ai/internal/platform"
	"github.com/shopally-ai/pkg/usecase"
)

func main() {
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

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

	alertRepo := repository.NewMongoAlertRepository(nil, cfg.Mongo.Database, cfg.Mongo.AlertCollection)
	productGateway := gateway.NewMockAlibabaGateway()
	evaluator := usecase.NewAlertEvaluator(alertRepo, productGateway)

	switchableCache := gateway.NewSwitchableCache(nil)
	var lastRedis *platform.Client
	redisReady := make(chan struct{}, 1)

	syncRuntime := func() {
		alertRepo.Rebind(conns.MongoClient(), cfg.Mongo.Database, cfg.Mongo.AlertCollection)

		rc := conns.RedisClient()
		if rc == nil {
			return
		}
		if rc == lastRedis {
			return
		}
		lastRedis = rc
		switchableCache.Replace(gateway.NewRedisCache(rc.Client, cfg.Redis.KeyPrefix))
		log.Println("redis cache active")
		redisReady <- struct{}{}
	}
	conns.StartBackgroundRetry(ctx, 5*time.Second, syncRuntime)

	fxHTTP := gateway.NewFXHTTPGateway(cfg.FX.APIURL, cfg.FX.APIKEY, nil)
	ttl := time.Duration(cfg.FX.CacheTTLSeconds) * time.Second
	fx := gateway.NewCachedFXClient(fxHTTP, switchableCache, ttl)

	warm := func() {
		if conns.RedisClient() == nil {
			return
		}
		warmCtx, warmCancel := context.WithTimeout(ctx, 10*time.Second)
		defer warmCancel()
		if rate, err := fx.GetRate(warmCtx, "USD", "ETB"); err != nil {
			log.Printf("worker warm fx error: %v", err)
		} else {
			log.Printf("worker warm fx USD->ETB: %.6f", rate)
		}
	}

	fxTicker := time.NewTicker(30 * time.Minute)
	alertTicker := time.NewTicker(time.Minute)
	defer fxTicker.Stop()
	defer alertTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-redisReady:
			warm()
		case <-fxTicker.C:
			warm()
		case <-alertTicker.C:
			if conns.MongoClient() == nil {
				continue
			}
			evalCtx, evalCancel := context.WithTimeout(ctx, 20*time.Second)
			count, err := evaluator.Evaluate(evalCtx)
			evalCancel()
			if err != nil {
				log.Printf("worker evaluate alerts error: %v", err)
			} else if count > 0 {
				log.Printf("worker triggered %d price alerts", count)
			}
		}
	}
}
