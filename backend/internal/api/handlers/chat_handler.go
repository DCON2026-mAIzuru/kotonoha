package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/apple/kotonoha/backend/internal/api/middleware"
	"github.com/apple/kotonoha/backend/internal/realtime"
	"github.com/apple/kotonoha/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	wsWriteWait      = 10 * time.Second
	wsPongWait       = 60 * time.Second
	wsPingPeriod     = (wsPongWait * 9) / 10
	wsMaxMessageSize = 4096
)

type wsChatRequest struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ChatHandler struct {
	chatService *service.ChatService
	realtimeHub *realtime.Hub
	upgrader    websocket.Upgrader
}

func NewChatHandler(chatService *service.ChatService, realtimeHub *realtime.Hub, allowedOrigin string) *ChatHandler {
	normalizedAllowedOrigin := strings.TrimSpace(allowedOrigin)

	return &ChatHandler{
		chatService: chatService,
		realtimeHub: realtimeHub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := strings.TrimSpace(r.Header.Get("Origin"))
				if origin == "" || normalizedAllowedOrigin == "" {
					return true
				}

				return origin == normalizedAllowedOrigin
			},
		},
	}
}

func (h *ChatHandler) History(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}

	entries, err := h.chatService.ListHistory(c.Request.Context(), user.ID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"history": entries})
}

func (h *ChatHandler) ClearHistory(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}

	result, err := h.chatService.ClearHistory(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": result})
}

func (h *ChatHandler) Chat(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocketアップグレードに失敗しました: %v", err)
		return
	}
	defer conn.Close()

	conn.SetReadLimit(wsMaxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	streamCtx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	var writerMu sync.Mutex
	writeJSON := func(payload realtime.Message) error {
		writerMu.Lock()
		defer writerMu.Unlock()

		_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
		return conn.WriteJSON(payload)
	}
	unregister := h.realtimeHub.Register(user.ID, writeJSON)
	defer unregister()

	done := make(chan struct{})
	defer close(done)

	go func() {
		ticker := time.NewTicker(wsPingPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-streamCtx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				writerMu.Lock()
				_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				err := conn.WriteMessage(websocket.PingMessage, nil)
				writerMu.Unlock()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var request wsChatRequest
		if err := json.Unmarshal(raw, &request); err != nil {
			_ = writeJSON(realtime.Message{
				Type:    "error",
				Message: "JSONの形式が不正です",
				Source:  "chat",
			})
			continue
		}

		if request.Type != "message" {
			_ = writeJSON(realtime.Message{
				Type:    "error",
				Message: "type=message のみ対応しています",
				Source:  "chat",
			})
			continue
		}

		meta, err := h.chatService.PrepareStreamMeta(streamCtx, user.ID)
		if err != nil {
			_ = writeJSON(realtime.Message{
				Type:    "error",
				Message: err.Error(),
				Source:  "chat",
			})
			continue
		}
		if err := writeJSON(realtime.Message{
			Type:   "reply_start",
			Source: "chat",
			Typing: meta.Typing,
		}); err != nil {
			return
		}

		err = h.chatService.StreamReply(streamCtx, service.StreamChatInput{
			UserID: user.ID,
			Text:   request.Text,
		}, func(text string) error {
			return writeJSON(realtime.Message{
				Type:   "reply_chunk",
				Text:   text,
				Source: "chat",
			})
		})
		if err != nil {
			_ = writeJSON(realtime.Message{
				Type:    "error",
				Message: err.Error(),
				Source:  "chat",
			})
			continue
		}

		if err := writeJSON(realtime.Message{Type: "reply_done", Source: "chat"}); err != nil {
			return
		}
	}
}
