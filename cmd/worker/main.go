package main

import (
	"context"
	"log"
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

	mongoClient, err := platform.Connect(cfg.Mongo.URI)
	if err != nil {
		log.Printf("mongo unavailable; alert evaluation will retry on future worker restart: %v", err)
	} else {
		defer platform.Disconnect(mongoClient)
	}
	alertRepo := repository.NewMongoAlertRepository(mongoClient, cfg.Mongo.Database, cfg.Mongo.AlertCollection)
	productGateway := gateway.NewMockAlibabaGateway()
	evaluator := usecase.NewAlertEvaluator(alertRepo, productGateway)

	rc := platform.NewRedisClient(cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.Password, cfg.Redis.DB)
	redisReady := false
	if err := rc.Ping(context.Background()); err != nil {
		log.Printf("redis unavailable; will retry in background: %v", err)
	} else {
		redisReady = true
	}
	cache := gateway.NewRedisCache(rc.Client, cfg.Redis.KeyPrefix)

	fxHTTP := gateway.NewFXHTTPGateway(cfg.FX.APIURL, cfg.FX.APIKEY, nil)
	ttl := time.Duration(cfg.FX.CacheTTLSeconds) * time.Second
	fx := gateway.NewCachedFXClient(fxHTTP, cache, ttl)

	// Optional: pre-warm a common FX pair periodically
	warm := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if rate, err := fx.GetRate(ctx, "USD", "ETB"); err != nil {
			log.Printf("worker warm fx error: %v", err)
		} else {
			log.Printf("worker warm fx USD->ETB: %.6f", rate)
		}
	}

	if redisReady {
		warm()
	}
	fxTicker := time.NewTicker(30 * time.Minute)
	alertTicker := time.NewTicker(time.Minute)
	redisTicker := time.NewTicker(5 * time.Second)
	defer fxTicker.Stop()
	defer alertTicker.Stop()
	defer redisTicker.Stop()
	for {
		select {
		case <-fxTicker.C:
			if redisReady {
				warm()
			}
		case <-alertTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			count, err := evaluator.Evaluate(ctx)
			cancel()
			if err != nil {
				log.Printf("worker evaluate alerts error: %v", err)
			} else if count > 0 {
				log.Printf("worker triggered %d price alerts", count)
			}
		case <-redisTicker.C:
			if !redisReady {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				err := rc.Ping(ctx)
				cancel()
				if err != nil {
					log.Printf("redis still unavailable: %v", err)
				} else {
					redisReady = true
					log.Println("redis connection recovered")
					warm()
				}
			}
		}
	}
}
