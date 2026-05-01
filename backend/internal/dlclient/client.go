package dlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/apple/kotonoha/backend/internal/models"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type DiagnosisResult struct {
	RequestID     string                 `json:"request_id"`
	HealthScore   float64                `json:"health_score"`
	Status        string                 `json:"status"`
	StatusLabel   string                 `json:"status_label"`
	Severity      string                 `json:"severity"`
	SeverityLabel string                 `json:"severity_label"`
	Detail        string                 `json:"detail"`
	Ratios        models.DiagnosisRatios `json:"ratios"`
	Artifacts     map[string]string      `json:"artifacts"`
	RawStdout     string                 `json:"raw_stdout"`
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Diagnose(ctx context.Context, requestID string, filename string, contentType string, image []byte) (*DiagnosisResult, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("request_id", requestID); err != nil {
		return nil, err
	}

	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="%s"`, escapeQuotes(filepath.Base(filename))))
	partHeader.Set("Content-Type", contentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(image); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/diagnose", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DLサービスへの接続に失敗しました: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("DLサービスがエラーを返しました: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var result DiagnosisResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("DLサービスのレスポンス解析に失敗しました: %w", err)
	}

	return &result, nil
}

func (c *Client) DownloadArtifact(ctx context.Context, requestID string, kind string) ([]byte, error) {
	u := c.baseURL + "/v1/artifacts/" + url.PathEscape(requestID) + "/" + url.PathEscape(kind)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DL artifact の取得に失敗しました: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("DL artifact が取得できません: status=%d", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 20*1024*1024))
}

func escapeQuotes(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}
