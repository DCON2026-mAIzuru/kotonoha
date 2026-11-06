package main

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestProxyRoutesAndPreservesAuthentication(t *testing.T) {
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "frontend") }))
	defer front.Close()
	back := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-only" || r.URL.Query().Get("token") != "test-token" {
			t.Error("認証情報が転送されていません")
		}
		_, _ = io.WriteString(w, "backend")
	}))
	defer back.Close()
	f, _ := url.Parse(front.URL)
	b, _ := url.Parse(back.URL)
	for _, path := range []string{"/", "/_next/static/chunk.js", "/api/v1/profile", "/ws/chat", "/healthz", "/readyz"} {
		r := httptest.NewRequest(http.MethodGet, path+"?token=test-token", nil)
		r.Header.Set("Authorization", "Bearer test-only")
		w := httptest.NewRecorder()
		newProxy(f, b).ServeHTTP(w, r)
		want := "backend"
		if path == "/" || path == "/_next/static/chunk.js" {
			want = "frontend"
		}
		if w.Code != http.StatusOK || w.Body.String() != want {
			t.Errorf("%s: %d %q", path, w.Code, w.Body.String())
		}
	}
}

func TestProxyUpgradesBidirectionalConnection(t *testing.T) {
	back := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws/chat" || r.URL.Query().Get("token") != "test-token" {
			t.Error("WebSocketの接続先または認証トークンが失われました")
		}
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
		_ = rw.Flush()
		message := make([]byte, 4)
		if _, err := io.ReadFull(rw, message); err != nil {
			t.Error(err)
			return
		}
		_, _ = rw.Write(message)
		_ = rw.Flush()
	}))
	defer back.Close()
	upstream, _ := url.Parse(back.URL)
	proxy := httptest.NewServer(newProxy(upstream, upstream))
	defer proxy.Close()
	address, _ := url.Parse(proxy.URL)
	conn, err := net.Dial("tcp", address.Host)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = io.WriteString(conn, "GET /ws/chat?token=test-token HTTP/1.1\r\nHost: example.test\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status=%d", response.StatusCode)
	}
	_, _ = io.WriteString(conn, "ping")
	message := make([]byte, 4)
	if _, err := io.ReadFull(reader, message); err != nil {
		t.Fatal(err)
	}
	if string(message) != "ping" {
		t.Fatalf("双方向転送が失敗しました: %q", message)
	}
}
