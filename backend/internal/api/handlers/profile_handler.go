package handlers

import (
	"net/http"

	"github.com/apple/kotonoha/backend/internal/api/middleware"
	"github.com/apple/kotonoha/backend/internal/models"
	"github.com/apple/kotonoha/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type ProfileHandler struct {
	profileService *service.ProfileService
}

func NewProfileHandler(profileService *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{
		profileService: profileService,
	}
}

func (h *ProfileHandler) Get(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}

	profile, err := h.profileService.Get(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "プロフィールの取得に失敗しました"})
		return
	}

	if profile == nil {
		c.JSON(http.StatusOK, gin.H{
			"profile": nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"profile": profile,
	})
}

func (h *ProfileHandler) Upsert(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}

	var input models.UpsertBonsaiProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "リクエストの形式が不正です"})
		return
	}
	input.UserID = user.ID

	profile, err := h.profileService.Upsert(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"profile": profile,
	})
}
