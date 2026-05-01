package handlers

import (
	"io"
	"net/http"
	"strconv"

	"github.com/apple/kotonoha/backend/internal/api/middleware"
	"github.com/apple/kotonoha/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type DiagnosisHandler struct {
	diagnosisService *service.DiagnosisService
	maxImageBytes    int64
}

func NewDiagnosisHandler(diagnosisService *service.DiagnosisService, maxImageMB int64) *DiagnosisHandler {
	if maxImageMB <= 0 {
		maxImageMB = 8
	}
	return &DiagnosisHandler{
		diagnosisService: diagnosisService,
		maxImageBytes:    maxImageMB * 1024 * 1024,
	}
}

func (h *DiagnosisHandler) Create(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}
	if h.diagnosisService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "診断サービスが初期化されていません"})
		return
	}

	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "image ファイルを指定してください"})
		return
	}
	defer file.Close()

	payload, err := io.ReadAll(io.LimitReader(file, h.maxImageBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "画像ファイルを読み取れません"})
		return
	}
	if int64(len(payload)) > h.maxImageBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"message": "画像ファイルが大きすぎます"})
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = service.DetectContentType(payload)
	}

	diagnosis, err := h.diagnosisService.Run(c.Request.Context(), service.RunDiagnosisInput{
		UserID:      user.ID,
		Filename:    header.Filename,
		ContentType: contentType,
		Image:       payload,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"diagnosis": diagnosis})
}

func (h *DiagnosisHandler) Latest(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}

	diagnosis, err := h.diagnosisService.Latest(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"diagnosis": diagnosis})
}

func (h *DiagnosisHandler) History(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	diagnoses, err := h.diagnosisService.History(c.Request.Context(), user.ID, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"diagnoses": diagnoses})
}

func (h *DiagnosisHandler) Artifact(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}

	path, err := h.diagnosisService.ArtifactPath(c.Request.Context(), user.ID, c.Param("id"), c.Param("kind"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	c.File(path)
}
