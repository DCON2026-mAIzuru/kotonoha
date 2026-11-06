package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apple/kotonoha/backend/pkg/config"
	"github.com/gin-gonic/gin"
)

func TestRequestLogsOmitQueryCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	previous := gin.DefaultWriter
	gin.DefaultWriter = &logs
	t.Cleanup(func() { gin.DefaultWriter = previous })
	router := NewRouter(Dependencies{Config: &config.Config{BackendAllowedOrigin: "http://localhost:3000"}})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/unknown?token=private-test-token", nil))
	if strings.Contains(logs.String(), "private-test-token") || strings.Contains(logs.String(), "?token") {
		t.Fatalf("request query leaked into logs: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "GET /unknown") {
		t.Fatal("request path missing from access log")
	}
}
