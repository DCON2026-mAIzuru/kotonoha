package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GeminiEmbedder struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewGeminiEmbedder(apiKey string, model string, httpClient *http.Client) *GeminiEmbedder {
	client := httpClient
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &GeminiEmbedder{
		apiKey:     strings.TrimSpace(apiKey),
		model:      strings.TrimSpace(model),
		httpClient: client,
	}
}

func (e *GeminiEmbedder) Embed(ctx context.Context, req EmbeddingRequest) ([]float64, error) {
	if strings.TrimSpace(req.Text) == "" {
		return nil, errors.New("埋め込み対象テキストが空です")
	}

	endpoint, err := e.buildEndpoint()
	if err != nil {
		return nil, err
	}

	payload, err := e.buildRequestBody(req)
	if err != nil {
		return nil, fmt.Errorf("埋め込みリクエストの組み立てに失敗しました: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("埋め込みリクエストの作成に失敗しました: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("x-goog-api-key", e.apiKey)

	response, err := e.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("埋め込みAPIへの接続に失敗しました: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return nil, fmt.Errorf("埋め込みAPIがエラーを返しました: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(raw)))
	}

	var result struct {
		Embedding struct {
			Values []float64 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("埋め込みAPIの応答解析に失敗しました: %w", err)
	}

	if len(result.Embedding.Values) == 0 {
		return nil, errors.New("埋め込みAPIから空ベクトルが返されました")
	}

	return result.Embedding.Values, nil
}

func (e *GeminiEmbedder) buildEndpoint() (string, error) {
	if e.apiKey == "" {
		return "", errors.New("GEMINI_API_KEY が設定されていません")
	}
	if e.model == "" {
		return "", errors.New("GEMINI_EMBEDDING_MODEL が設定されていません")
	}

	endpoint, err := url.Parse(
		fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:embedContent", url.PathEscape(normalizeGeminiModelName(e.model))),
	)
	if err != nil {
		return "", fmt.Errorf("埋め込みエンドポイントの構築に失敗しました: %w", err)
	}

	return endpoint.String(), nil
}

func (e *GeminiEmbedder) buildRequestBody(req EmbeddingRequest) ([]byte, error) {
	type part struct {
		Text string `json:"text"`
	}

	type content struct {
		Parts []part `json:"parts"`
	}

	payload := map[string]any{
		"model": "models/" + normalizeGeminiModelName(e.model),
		"content": content{
			Parts: []part{{Text: req.Text}},
		},
	}

	if req.TaskType != "" {
		payload["taskType"] = req.TaskType
	}
	if strings.TrimSpace(req.Title) != "" {
		payload["title"] = strings.TrimSpace(req.Title)
	}

	return json.Marshal(payload)
}
