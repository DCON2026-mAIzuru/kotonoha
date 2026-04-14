package handlers

import (
	"net/http"

	"github.com/apple/kotonoha/backend/internal/api/middleware"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct{}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
}

func (h *AuthHandler) Me(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "認証ユーザーが見つかりません",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}
