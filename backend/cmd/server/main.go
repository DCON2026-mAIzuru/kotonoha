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

	"github.com/apple/kotonoha/backend/internal/api"
	"github.com/apple/kotonoha/backend/internal/repository"
	"github.com/apple/kotonoha/backend/internal/service"
	pkgauth "github.com/apple/kotonoha/backend/pkg/auth"
	"github.com/apple/kotonoha/backend/pkg/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("設定の読み込みに失敗しました: %v", err)
	}

	store, err := repository.NewPostgresStore(ctx, cfg.SupabaseDBURL)
	if err != nil {
		log.Fatalf("Supabase接続の初期化に失敗しました: %v", err)
	}
	defer store.Close()

	verifier, err := pkgauth.NewSupabaseJWTVerifier(cfg.SupabaseURL, cfg.SupabasePublishableKey, cfg.SupabaseJWTSecret)
	if err != nil {
		log.Fatalf("JWT検証器の初期化に失敗しました: %v", err)
	}

	deps := api.Dependencies{
		Config:                  cfg,
		HealthService:           service.NewHealthService(store),
		ProfileService:          service.NewProfileService(store),
		PushSubscriptionService: service.NewPushSubscriptionService(store),
		JWTVerifier:             verifier,
	}

	engine := api.NewRouter(deps)
	server := &http.Server{
		Addr:              ":" + cfg.BackendPort,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("バックエンドを起動しました: port=%s", cfg.BackendPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("サーバーの起動に失敗しました: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("シャットダウンを開始します")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("サーバーの停止に失敗しました: %v", err)
		os.Exit(1)
	}

	log.Println("サーバーを停止しました")
}
