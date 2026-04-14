package middleware

import (
	"net/http"
	"strings"

	"github.com/apple/kotonoha/backend/internal/models"
	pkgauth "github.com/apple/kotonoha/backend/pkg/auth"
	"github.com/gin-gonic/gin"
)

const currentUserContextKey = "auth_user"

func RequireAuth(verifier *pkgauth.SupabaseJWTVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "認証トークンがありません",
			})
			return
		}

		user, err := verifier.Verify(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "認証トークンを検証できませんでした",
			})
			return
		}

		c.Set(currentUserContextKey, user)
		c.Next()
	}
}

func CurrentUser(c *gin.Context) (*models.AuthUser, bool) {
	value, exists := c.Get(currentUserContextKey)
	if !exists {
		return nil, false
	}

	user, ok := value.(*models.AuthUser)
	return user, ok
}

func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		const prefix = "Bearer "
		if strings.HasPrefix(authHeader, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
		}
	}

	return strings.TrimSpace(c.Query("token"))
}
