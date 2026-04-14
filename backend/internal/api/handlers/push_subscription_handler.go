package handlers

import (
	"net/http"

	"github.com/apple/kotonoha/backend/internal/api/middleware"
	"github.com/apple/kotonoha/backend/internal/models"
	"github.com/apple/kotonoha/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type PushSubscriptionHandler struct {
	pushSubscriptionService *service.PushSubscriptionService
}

func NewPushSubscriptionHandler(pushSubscriptionService *service.PushSubscriptionService) *PushSubscriptionHandler {
	return &PushSubscriptionHandler{
		pushSubscriptionService: pushSubscriptionService,
	}
}

func (h *PushSubscriptionHandler) List(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}

	subscriptions, err := h.pushSubscriptionService.List(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Push購読一覧の取得に失敗しました"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subscriptions": subscriptions,
	})
}

func (h *PushSubscriptionHandler) Upsert(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}

	var input models.UpsertPushSubscriptionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "リクエストの形式が不正です"})
		return
	}
	input.UserID = user.ID

	subscription, err := h.pushSubscriptionService.Upsert(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subscription": subscription,
	})
}

func (h *PushSubscriptionHandler) Delete(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}

	subscriptionID := c.Param("id")
	if err := h.pushSubscriptionService.Delete(c.Request.Context(), user.ID, subscriptionID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deleted": true,
	})
}
