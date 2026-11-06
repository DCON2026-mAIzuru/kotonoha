package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// 同一オリジンで画面・API・WebSocketを提供する。認証情報やURLはログに残さない。
func newProxy(frontend, backend *url.URL) http.Handler {
	makeProxy := func(target *url.URL) *httputil.ReverseProxy {
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.FlushInterval = -1
		proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "ことの葉のサービスに接続できません", http.StatusBadGateway)
		}
		return proxy
	}
	front, back := makeProxy(frontend), makeProxy(backend)
	authTarget, _ := url.Parse("http://127.0.0.1:54321")
	auth := makeProxy(authTarget)
	authPaths := map[string]bool{"/token": true, "/user": true, "/logout": true, "/settings": true, "/health": true, "/.well-known/jwks.json": true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/supabase/") {
			path := strings.TrimPrefix(r.URL.Path, "/supabase/auth/v1")
			if !authPaths[path] {
				http.NotFound(w, r)
				return
			}
			r = r.Clone(r.Context())
			r.URL.Path, r.URL.RawPath = path, ""
			auth.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") || r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			back.ServeHTTP(w, r)
			return
		}
		front.ServeHTTP(w, r)
	})
}

func main() {
	front, _ := url.Parse("http://127.0.0.1:3000")
	back, _ := url.Parse("http://127.0.0.1:8080")
	server := &http.Server{Addr: "127.0.0.1:8787", Handler: newProxy(front, back), ReadHeaderTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Println("ことの葉の公開用プロキシを起動しました: 127.0.0.1:8787")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("プロキシを起動できませんでした。ポート8787の使用状況を確認してください")
	}
}
