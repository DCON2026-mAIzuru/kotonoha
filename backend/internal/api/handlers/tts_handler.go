package handlers

import (
	"net/http"

	"github.com/apple/kotonoha/backend/internal/api/middleware"
	"github.com/apple/kotonoha/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type TTSHandler struct {
	voicevoxService *service.VoicevoxService
}

type voicevoxTTSRequest struct {
	Text string `json:"text"`
}

func NewTTSHandler(voicevoxService *service.VoicevoxService) *TTSHandler {
	return &TTSHandler{voicevoxService: voicevoxService}
}

func (h *TTSHandler) Voicevox(c *gin.Context) {
	if _, ok := middleware.CurrentUser(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}
	if h.voicevoxService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "VOICEVOXサービスが初期化されていません"})
		return
	}

	var request voicevoxTTSRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "text をJSONで指定してください"})
		return
	}

	audio, err := h.voicevoxService.Synthesize(c.Request.Context(), request.Text)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}

	c.Header("Content-Type", "audio/wav")
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "audio/wav", audio)
}
