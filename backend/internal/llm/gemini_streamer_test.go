package llm

import (
	"net/url"
	"testing"
)

func TestGeminiStreamerBuildEndpointNormalizesModelResourceName(t *testing.T) {
	t.Parallel()

	streamer := NewGeminiStreamer("test-key", "models/gemini-2.5-flash", nil)

	endpoint, err := streamer.buildEndpoint()
	if err != nil {
		t.Fatalf("buildEndpoint() error = %v", err)
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}

	wantPath := "/v1beta/models/gemini-2.5-flash:streamGenerateContent"
	if parsed.Path != wantPath {
		t.Fatalf("endpoint path = %q, want %q", parsed.Path, wantPath)
	}
}
