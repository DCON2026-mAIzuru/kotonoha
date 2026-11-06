package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/apple/kotonoha/backend/internal/llm"
	"github.com/apple/kotonoha/backend/internal/repository"
	"github.com/apple/kotonoha/backend/internal/service"
	"github.com/apple/kotonoha/backend/pkg/config"
)

func main() {
	if err := run(); err != nil {
		// DB接続文字列・会話本文・サービス応答はエラー出力へ含めない。
		fmt.Fprintf(os.Stderr, "会話記憶の取り込みに失敗しました: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	apply := flag.Bool("apply", false, "確認した成功会話ペアを取り込む（省略時は件数確認のみ）")
	userID := flag.String("user-id", "", "対象ユーザーUUID（省略時は全ユーザー）")
	limit := flag.Int("limit", 100, "1回の最大ペア数（1〜1000、再実行で続きから取り込む）")
	legacyAdjacent := flag.Bool("legacy-adjacent", false, "返信関係のない旧履歴を隣接推定する。事前にペアを確認し、会話送信を停止して使用する")
	flag.Parse()
	if *limit < 1 || *limit > 1000 {
		return errors.New("limitは1から1000で指定してください")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		return errors.New("サーバー環境変数を確認してください")
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.EmbeddingProvider), "local") {
		return errors.New("EMBEDDING_PROVIDER=localが必要です")
	}
	timeout, err := strconv.Atoi(cfg.EmbeddingTimeout)
	if err != nil || timeout <= 0 {
		return errors.New("EMBEDDING_TIMEOUT_SECONDSは正の整数で指定してください")
	}
	embedder, err := llm.NewConfiguredEmbedder(ctx, llm.EmbedderConfig{
		Provider: "local", BaseURL: cfg.EmbeddingBaseURL, Timeout: time.Duration(timeout) * time.Second,
	})
	if err != nil {
		return errors.New("ローカル埋め込みサービスとメタデータを確認してください")
	}
	store, err := repository.NewPostgresStore(ctx, cfg.SupabaseDBURL)
	if err != nil {
		return errors.New("DB接続設定と稼働状態を確認してください")
	}
	defer store.Close()
	result, err := service.BackfillConversationMemories(ctx, store, embedder, strings.TrimSpace(*userID), *limit, *apply, *legacyAdjacent)
	if err != nil {
		return errors.New("取り込み処理が失敗しました。追加migration・サービス状態を確認し、再実行してください")
	}
	fmt.Printf("space=%s candidates=%d processed=%d apply=%t\n", embedder.Space(), result.Candidates, result.Processed, *apply)
	return nil
}
