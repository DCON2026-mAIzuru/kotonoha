package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/apple/kotonoha/backend/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

type SupabaseJWTVerifier struct {
	secret         []byte
	authUserURL    string
	publishableKey string
	httpClient     *http.Client
}

type SupabaseClaims struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

func NewSupabaseJWTVerifier(projectURL string, publishableKey string, secret string) (*SupabaseJWTVerifier, error) {
	if projectURL == "" {
		return nil, errors.New("SUPABASE_URL が設定されていません")
	}

	if publishableKey == "" {
		return nil, errors.New("SUPABASE_PUBLISHABLE_KEY が設定されていません")
	}

	return &SupabaseJWTVerifier{
		secret:         []byte(secret),
		authUserURL:    strings.TrimRight(projectURL, "/") + "/auth/v1/user",
		publishableKey: publishableKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}, nil
}

func (v *SupabaseJWTVerifier) Verify(ctx context.Context, tokenString string) (*models.AuthUser, error) {
	if len(v.secret) > 0 {
		user, err := v.verifyWithSharedSecret(tokenString)
		if err == nil {
			return user, nil
		}
	}

	return v.verifyWithAuthServer(ctx, tokenString)
}

func (v *SupabaseJWTVerifier) verifyWithSharedSecret(tokenString string) (*models.AuthUser, error) {
	token, err := jwt.ParseWithClaims(tokenString, &SupabaseClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("想定外の署名方式です: %s", token.Method.Alg())
		}

		return v.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, fmt.Errorf("JWTの検証に失敗しました: %w", err)
	}

	claims, ok := token.Claims.(*SupabaseClaims)
	if !ok || !token.Valid {
		return nil, errors.New("JWTクレームを読み取れません")
	}

	if claims.Subject == "" {
		return nil, errors.New("JWTにユーザーIDが含まれていません")
	}

	return &models.AuthUser{
		ID:    claims.Subject,
		Email: claims.Email,
		Role:  claims.Role,
	}, nil
}

type authServerUserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (v *SupabaseJWTVerifier) verifyWithAuthServer(ctx context.Context, tokenString string) (*models.AuthUser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, v.authUserURL, nil)
	if err != nil {
		return nil, fmt.Errorf("認証サーバー向けリクエストを作成できません: %w", err)
	}

	request.Header.Set("apikey", v.publishableKey)
	request.Header.Set("Authorization", "Bearer "+tokenString)

	response, err := v.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("認証サーバーへの問い合わせに失敗しました: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("認証サーバーがJWTを受け付けませんでした: status=%d", response.StatusCode)
	}

	var user authServerUserResponse
	if err := json.NewDecoder(response.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("認証サーバーの応答を読み取れません: %w", err)
	}

	if user.ID == "" {
		return nil, errors.New("認証サーバー応答にユーザーIDが含まれていません")
	}

	return &models.AuthUser{
		ID:    user.ID,
		Email: user.Email,
		Role:  user.Role,
	}, nil
}
