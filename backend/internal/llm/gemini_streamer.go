package llm

import (
	"bufio"
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

type GeminiAPIError struct {
	StatusCode int
	Status     string
	Message    string
	Body       string
}

func (e *GeminiAPIError) Error() string {
	switch {
	case e == nil:
		return ""
	case e.Status != "" && e.Message != "":
		return fmt.Sprintf("Geminiがエラーを返しました: status=%d api_status=%s message=%s body=%s", e.StatusCode, e.Status, e.Message, e.Body)
	case e.Message != "":
		return fmt.Sprintf("Geminiがエラーを返しました: status=%d message=%s body=%s", e.StatusCode, e.Message, e.Body)
	default:
		return fmt.Sprintf("Geminiがエラーを返しました: status=%d body=%s", e.StatusCode, e.Body)
	}
}

type GeminiStreamer struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewGeminiStreamer(apiKey string, model string, httpClient *http.Client) *GeminiStreamer {
	client := httpClient
	if client == nil {
		client = &http.Client{
			Timeout: 120 * time.Second,
		}
	}

	return &GeminiStreamer{
		apiKey:     strings.TrimSpace(apiKey),
		model:      strings.TrimSpace(model),
		httpClient: client,
	}
}

func (s *GeminiStreamer) StreamChat(ctx context.Context, req ChatRequest, onChunk func(text string) error) error {
	if strings.TrimSpace(req.Message) == "" {
		return errors.New("送信メッセージが空です")
	}

	endpoint, err := s.buildEndpoint()
	if err != nil {
		return err
	}

	payload, err := s.buildRequestBody(req)
	if err != nil {
		return fmt.Errorf("Geminiリクエストの組み立てに失敗しました: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("Geminiリクエストの作成に失敗しました: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := s.httpClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("Geminiへの接続に失敗しました: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return buildGeminiAPIError(response.StatusCode, raw)
	}

	return s.scanSSE(response.Body, onChunk)
}

func buildGeminiAPIError(statusCode int, rawBody []byte) error {
	body := strings.TrimSpace(string(rawBody))

	var payload struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return &GeminiAPIError{
			StatusCode: statusCode,
			Body:       body,
		}
	}

	return &GeminiAPIError{
		StatusCode: statusCode,
		Status:     strings.TrimSpace(payload.Error.Status),
		Message:    strings.TrimSpace(payload.Error.Message),
		Body:       body,
	}
}

func (s *GeminiStreamer) buildEndpoint() (string, error) {
	if s.apiKey == "" {
		return "", errors.New("GEMINI_API_KEY が設定されていません")
	}

	if s.model == "" {
		return "", errors.New("GEMINI_MODEL が設定されていません")
	}

	endpoint, err := url.Parse(
		fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent", url.PathEscape(s.model)),
	)
	if err != nil {
		return "", fmt.Errorf("Geminiエンドポイントの構築に失敗しました: %w", err)
	}

	query := endpoint.Query()
	query.Set("alt", "sse")
	query.Set("key", s.apiKey)
	endpoint.RawQuery = query.Encode()

	return endpoint.String(), nil
}

func (s *GeminiStreamer) buildRequestBody(req ChatRequest) ([]byte, error) {
	type part struct {
		Text string `json:"text"`
	}

	type content struct {
		Role  string `json:"role,omitempty"`
		Parts []part `json:"parts"`
	}

	payload := map[string]any{
		"contents": []content{
			{
				Role:  "user",
				Parts: []part{{Text: req.Message}},
			},
		},
	}

	if strings.TrimSpace(req.SystemPrompt) != "" {
		payload["systemInstruction"] = content{
			Parts: []part{{Text: req.SystemPrompt}},
		}
	}

	return json.Marshal(payload)
}

func (s *GeminiStreamer) scanSSE(reader io.Reader, onChunk func(text string) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	hasAnyChunk := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}

		if data == "[DONE]" {
			break
		}

		var event geminiStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return fmt.Errorf("Geminiストリーム応答の解析に失敗しました: %w", err)
		}

		if event.Error != nil {
			return fmt.Errorf("Geminiストリームエラー: %s", event.Error.Message)
		}

		for _, candidate := range event.Candidates {
			for _, p := range candidate.Content.Parts {
				if p.Text == "" {
					continue
				}

				hasAnyChunk = true
				if err := onChunk(p.Text); err != nil {
					return err
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("Geminiストリーム読み取りに失敗しました: %w", err)
	}

	if !hasAnyChunk {
		return errors.New("Geminiから空の応答が返されました")
	}

	return nil
}

type geminiStreamEvent struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}
