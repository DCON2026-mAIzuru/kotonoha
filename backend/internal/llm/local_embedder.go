package llm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const StoredEmbeddingDimension = 3072
const localEmbeddingDimension = 384

type LocalEmbedder struct {
	baseURL string
	space   string
	client  *http.Client
}

// メタデータの内容から空間を確認し、稼働中のモデル変更も応答ごとに検出する。
func NewLocalEmbedder(ctx context.Context, baseURL string, timeout time.Duration) (*LocalEmbedder, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" {
		return nil, errors.New("ローカル埋め込みURLは認証情報やパスを含まないloopback HTTP URLを指定してください")
	}
	ip := net.ParseIP(parsed.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return nil, errors.New("ローカル埋め込みサービスはloopback IPアドレスで指定してください")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	e := &LocalEmbedder{baseURL: parsed.String(), client: &http.Client{
		Timeout: timeout, Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
	var metadata map[string]any
	if err := e.request(ctx, http.MethodGet, "/metadata", nil, &metadata); err != nil {
		return nil, err
	}
	space, err := validateLocalEmbeddingMetadata(metadata)
	if err != nil {
		return nil, err
	}
	e.space = space
	return e, nil
}

func validateLocalEmbeddingMetadata(metadata map[string]any) (string, error) {
	text := func(key string) string { value, _ := metadata[key].(string); return value }
	if text("service") != "kotonoha-local-embeddings" || text("model_id") != "intfloat/multilingual-e5-small" ||
		!regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(text("revision")) ||
		metadata["dimension"] != float64(localEmbeddingDimension) || metadata["padding_dimension"] != float64(StoredEmbeddingDimension) ||
		metadata["normalized"] != true || metadata["max_tokens"] != float64(512) ||
		text("query_prefix") != "query: " || text("passage_prefix") != "passage: " ||
		text("pooling") != "attention-mask-mean" || text("normalization") != "l2-float32" || text("padding") != "right-zero" {
		return "", errors.New("ローカル埋め込みのモデル・次元・prefix・正規化設定が一致しません")
	}
	space := text("embedding_space")
	geometry := make(map[string]any, len(metadata)-2)
	for key, value := range metadata {
		if key != "service" && key != "embedding_space" {
			geometry[key] = value
		}
	}
	canonical, err := json.Marshal(geometry)
	if err != nil {
		return "", errors.New("埋め込み空間のメタデータを確認できません")
	}
	want := fmt.Sprintf("e5-small-v1-%x", sha256.Sum256(canonical))
	if space != want {
		return "", errors.New("埋め込み空間IDとモデル設定のハッシュが一致しません")
	}
	return space, nil
}

func (e *LocalEmbedder) Space() string { return e.space }

func (e *LocalEmbedder) Embed(ctx context.Context, req EmbeddingRequest) ([]float64, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, errors.New("埋め込み対象テキストが空です")
	}
	if len([]rune(text)) > 16000 {
		return nil, errors.New("埋め込み対象テキストが長すぎます")
	}
	var kind string
	switch req.TaskType {
	case EmbeddingTaskRetrievalQuery:
		kind = "query"
	case EmbeddingTaskRetrievalDocument:
		kind = "passage"
	default:
		return nil, errors.New("埋め込みの検索・文書種別を指定してください")
	}
	payload, _ := json.Marshal(map[string]string{"text": text, "kind": kind})
	var result struct {
		Embedding []float64 `json:"embedding"`
		Space     string    `json:"embedding_space"`
		Dimension int       `json:"dimension"`
	}
	if err := e.request(ctx, http.MethodPost, "/embed", payload, &result); err != nil {
		return nil, err
	}
	if result.Space != e.space || result.Dimension != localEmbeddingDimension || len(result.Embedding) != localEmbeddingDimension {
		return nil, errors.New("埋め込み応答の空間または次元が一致しません")
	}
	var norm float64
	for _, value := range result.Embedding {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, errors.New("埋め込みに有限でない値があります")
		}
		norm += value * value
	}
	if math.Abs(norm-1) > 0.002 {
		return nil, errors.New("埋め込みベクトルがL2正規化されていません")
	}
	// 末尾のゼロ追加は内積・ノルムを変えず、既存vector(3072)と互換にする。
	vector := make([]float64, StoredEmbeddingDimension)
	copy(vector, result.Embedding)
	return vector, nil
}

func (e *LocalEmbedder) request(ctx context.Context, method, path string, payload []byte, result any) error {
	req, err := http.NewRequestWithContext(ctx, method, e.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return errors.New("ローカル埋め込みリクエストを作成できません")
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := e.client.Do(req)
	if err != nil {
		return errors.New("ローカル埋め込みサービスへの接続に失敗しました")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("ローカル埋め込みサービスがエラーを返しました: status=%d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(result); err != nil {
		return errors.New("ローカル埋め込み応答を解析できません")
	}
	return nil
}
