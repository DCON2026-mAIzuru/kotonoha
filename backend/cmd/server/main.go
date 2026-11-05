package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/apple/kotonoha/backend/internal/api"
	"github.com/apple/kotonoha/backend/internal/dlclient"
	"github.com/apple/kotonoha/backend/internal/llm"
	"github.com/apple/kotonoha/backend/internal/realtime"
	"github.com/apple/kotonoha/backend/internal/repository"
	"github.com/apple/kotonoha/backend/internal/service"
	pushtransport "github.com/apple/kotonoha/backend/internal/webpush"
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

	llmStreamer, err := llm.NewConfiguredStreamer(llm.StreamerConfig{
		Provider: cfg.LLMProvider, BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMAPIKey, Model: cfg.LLMModel,
		MaxTokens: parsePositiveInt(cfg.LLMMaxTokens, 1024), Timeout: time.Duration(parsePositiveInt(cfg.LLMTimeout, 120)) * time.Second,
		GeminiAPIKey: cfg.GeminiAPIKey, GeminiModel: cfg.GeminiModel, GeminiModelPriority: cfg.GeminiModelPriority,
	})
	if err != nil {
		log.Fatalf("会話モデルの設定に失敗しました: %v", err)
	}
	llmEmbedder, err := llm.NewConfiguredEmbedder(cfg.EmbeddingProvider, cfg.GeminiAPIKey, cfg.GeminiEmbeddingModel)
	if err != nil {
		log.Fatalf("埋め込みモデルの設定に失敗しました: %v", err)
	}
	if llmEmbedder == nil {
		log.Println("長期記憶のベクトル検索・保存は無効です。会話履歴の保存は継続します")
	}
	if strings.EqualFold(strings.TrimSpace(cfg.LLMProvider), "disabled") {
		log.Println("LLM_PROVIDER=disabled: LLMが未接続です。Colabの復旧と接続設定が必要です。会話生成はエラーを返します")
	}
	if cfg.LLMProvider == "fallback" || cfg.EmbeddingProvider == "fallback" {
		log.Println("開発用の代替応答・埋め込みが選択されています。実際のモデルによる生成ではありません")
	}
	realtimeHub := realtime.NewHub()
	pushSender := pushtransport.NewSender(cfg.VapidPublicKey, cfg.VapidPrivateKey, cfg.VapidSubject)
	dlTimeoutSeconds := parsePositiveInt(cfg.DLServiceTimeout, 90)
	voicevoxTimeoutSeconds := parsePositiveInt(cfg.VoicevoxTimeout, 30)
	diagnosisMaxImageMB := int64(parsePositiveInt(cfg.DiagnosisMaxImageMB, 8))
	dlClient := dlclient.New(cfg.DLServiceURL, time.Duration(dlTimeoutSeconds)*time.Second)
	voicevoxService := service.NewVoicevoxService(
		cfg.VoicevoxEngineURL,
		cfg.VoicevoxSpeakerID,
		cfg.VoicevoxSpeakerName,
		cfg.VoicevoxStyleName,
		cfg.VoicevoxSpeedScale,
		time.Duration(voicevoxTimeoutSeconds)*time.Second,
	)

	deps := api.Dependencies{
		Config:                  cfg,
		HealthService:           service.NewHealthService(store),
		ProfileService:          service.NewProfileService(store),
		PushSubscriptionService: service.NewPushSubscriptionService(store, pushSender),
		ChatService:             service.NewChatService(store, llmStreamer, llmEmbedder),
		SensorService:           service.NewSensorService(store, llmStreamer, realtimeHub, pushSender),
		DiagnosisService:        service.NewDiagnosisService(store, dlClient, diagnosisMaxImageMB, cfg.DiagnosisArtifactDir),
		VoicevoxService:         voicevoxService,
		RealtimeHub:             realtimeHub,
		JWTVerifier:             verifier,
	}

	engine := api.NewRouter(deps)
	server := &http.Server{
		Addr:              net.JoinHostPort(cfg.BackendHost, cfg.BackendPort),
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

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
