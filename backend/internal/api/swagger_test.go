package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/apple/kotonoha/backend/pkg/config"
	"github.com/gin-gonic/gin"
)

func TestSwaggerEndpointsAndRouteCoverage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(Dependencies{Config: &config.Config{BackendAllowedOrigin: "http://localhost:3000"}})

	for _, path := range []string{"/swagger/", "/swagger/openapi.json"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s: got %d", path, w.Code)
		}
		if path == "/swagger/" && !strings.Contains(w.Body.String(), "SwaggerUIBundle") {
			t.Fatal("Swagger UI initialization missing")
		}
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/swagger/openapi.json", nil))
	var document struct {
		OpenAPI string                                `json:"openapi"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.OpenAPI != "3.0.3" {
		t.Fatalf("unexpected OpenAPI version: %s", document.OpenAPI)
	}
	pathParameter := regexp.MustCompile(`:([^/]+)`)
	for _, route := range router.Routes() {
		if strings.HasPrefix(route.Path, "/swagger") {
			continue
		}
		path := pathParameter.ReplaceAllString(route.Path, `{$1}`)
		if _, ok := document.Paths[path][strings.ToLower(route.Method)]; !ok {
			t.Errorf("route %s %s is missing from OpenAPI", route.Method, route.Path)
		}
	}
}
