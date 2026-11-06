package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OpenAICompatibleStreamer talks to an existing chat-completions server.
// Credentials and the endpoint URL are used only by the backend.
type OpenAICompatibleStreamer struct {
	endpoint   string
	apiKey     string
	model      string
	maxTokens  int
	httpClient *http.Client
}

func NewOpenAICompatibleStreamer(baseURL, apiKey, model string, maxTokens int, timeout time.Duration) (*OpenAICompatibleStreamer, error) {
	endpoint, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || endpoint == nil || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, errors.New("LLM_BASE_URL に認証情報・クエリを含まない API ベース URL を設定してください")
	}
	if endpoint.Scheme != "https" {
		host := endpoint.Hostname()
		ip := net.ParseIP(host)
		if endpoint.Scheme != "http" || (host != "localhost" && (ip == nil || !ip.IsLoopback())) {
			return nil, errors.New("LLM_BASE_URL は HTTPS、またはローカル転送先の HTTP を使用してください")
		}
	}
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("LLM_MODEL に稼働中サーバーのモデル ID を設定してください")
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/chat/completions"
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &OpenAICompatibleStreamer{
		endpoint: endpoint.String(), apiKey: strings.TrimSpace(apiKey), model: strings.TrimSpace(model), maxTokens: maxTokens,
		httpClient: &http.Client{
			Timeout: timeout,
			// Never forward a server credential to a redirect target.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

func (s *OpenAICompatibleStreamer) StreamChat(ctx context.Context, req ChatRequest, onChunk func(string) error) error {
	if strings.TrimSpace(req.Message) == "" {
		return errors.New("送信メッセージが空です")
	}
	if onChunk == nil {
		return errors.New("ストリーム受信ハンドラーがありません")
	}
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	messages := make([]message, 0, 2)
	if strings.TrimSpace(req.SystemPrompt) != "" {
		messages = append(messages, message{Role: "system", Content: req.SystemPrompt})
	}
	messages = append(messages, message{Role: "user", Content: req.Message})
	body, err := json.Marshal(struct {
		Model     string    `json:"model"`
		Messages  []message `json:"messages"`
		Stream    bool      `json:"stream"`
		MaxTokens int       `json:"max_tokens"`
	}{s.model, messages, true, s.maxTokens})
	if err != nil {
		return errors.New("LLM リクエストを作成できませんでした")
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("LLM リクエストを作成できませんでした")
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "text/event-stream")
	if s.apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+s.apiKey)
	}
	response, err := s.httpClient.Do(httpRequest)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Network errors may contain the private endpoint URL.
		return errors.New("LLM サーバーへ接続できません。接続先と稼働状態を確認してください")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("LLM サーバーがエラーを返しました: HTTP %d", response.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "text/event-stream" {
		return errors.New("LLM サーバーがストリーミング応答を返しませんでした")
	}
	return scanChatCompletionSSE(response.Body, onChunk)
}

func scanChatCompletionSSE(reader io.Reader, onChunk func(string) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var data []string
	hasText := false
	consume := func() (bool, error) {
		if len(data) == 0 {
			return false, nil
		}
		payload := strings.Join(data, "\n")
		data = nil
		if strings.TrimSpace(payload) == "[DONE]" {
			if !hasText {
				return false, errors.New("LLM から空の応答が返されました")
			}
			return true, nil
		}
		var event struct {
			Choices []struct {
				Index int `json:"index"`
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return false, errors.New("LLM ストリーム応答の形式が不正です")
		}
		if len(event.Error) > 0 && string(event.Error) != "null" {
			return false, errors.New("LLM ストリームの生成中にエラーが発生しました")
		}
		for _, choice := range event.Choices {
			if choice.Index != 0 || choice.Delta.Content == "" {
				continue
			}
			hasText = true
			if err := onChunk(choice.Delta.Content); err != nil {
				return false, err
			}
		}
		return false, nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			done, err := consume()
			if done || err != nil {
				return err
			}
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if scanner.Err() != nil {
		return errors.New("LLM ストリームの読み取りに失敗しました")
	}
	if done, err := consume(); done || err != nil {
		return err
	}
	return errors.New("LLM ストリームが完了する前に切断されました")
}
