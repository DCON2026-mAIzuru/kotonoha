package api

import (
	"strings"
	"time"

	"github.com/apple/kotonoha/backend/internal/api/handlers"
	"github.com/apple/kotonoha/backend/internal/api/middleware"
	"github.com/apple/kotonoha/backend/internal/realtime"
	"github.com/apple/kotonoha/backend/internal/service"
	pkgauth "github.com/apple/kotonoha/backend/pkg/auth"
	"github.com/apple/kotonoha/backend/pkg/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	Config                  *config.Config
	HealthService           *service.HealthService
	ProfileService          *service.ProfileService
	PushSubscriptionService *service.PushSubscriptionService
	ChatService             *service.ChatService
	SensorService           *service.SensorService
	DiagnosisService        *service.DiagnosisService
	RealtimeHub             *realtime.Hub
	JWTVerifier             *pkgauth.SupabaseJWTVerifier
}

func NewRouter(deps Dependencies) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())
	engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{deps.Config.BackendAllowedOrigin},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Sensor-Token", "X-Sensor-User-ID"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	healthHandler := handlers.NewHealthHandler(deps.HealthService)
	authHandler := handlers.NewAuthHandler()
	profileHandler := handlers.NewProfileHandler(deps.ProfileService)
	pushSubscriptionHandler := handlers.NewPushSubscriptionHandler(deps.PushSubscriptionService)
	chatHandler := handlers.NewChatHandler(deps.ChatService, deps.RealtimeHub, deps.Config.BackendAllowedOrigin)
	sensorHandler := handlers.NewSensorHandler(
		deps.SensorService,
		deps.Config.SensorSharedToken,
		strings.EqualFold(deps.Config.PresentationDemoMode, "true"),
	)
	diagnosisHandler := handlers.NewDiagnosisHandler(deps.DiagnosisService, parsePositiveInt64(deps.Config.DiagnosisMaxImageMB, 8))
	authMiddleware := middleware.RequireAuth(deps.JWTVerifier)

	engine.GET("/healthz", healthHandler.Liveness)
	engine.GET("/readyz", healthHandler.Readiness)
	engine.GET("/ws/chat", authMiddleware, chatHandler.Chat)

	apiV1 := engine.Group("/api/v1")
	{
		authRoutes := apiV1.Group("/auth")
		authRoutes.Use(authMiddleware)
		authRoutes.GET("/me", authHandler.Me)

		profileRoutes := apiV1.Group("/profile")
		profileRoutes.Use(authMiddleware)
		profileRoutes.GET("", profileHandler.Get)
		profileRoutes.PUT("", profileHandler.Upsert)

		pushRoutes := apiV1.Group("/push")
		pushRoutes.Use(authMiddleware)
		pushRoutes.GET("/subscriptions", pushSubscriptionHandler.List)
		pushRoutes.POST("/subscribe", pushSubscriptionHandler.Upsert)
		pushRoutes.POST("/demo", pushSubscriptionHandler.SendDemo)
		pushRoutes.DELETE("/subscriptions/:id", pushSubscriptionHandler.Delete)

		chatRoutes := apiV1.Group("/chat")
		chatRoutes.Use(authMiddleware)
		chatRoutes.GET("/history", chatHandler.History)
		chatRoutes.DELETE("/history", chatHandler.ClearHistory)

		sensorRoutes := apiV1.Group("/sensor")
		sensorRoutes.Use(authMiddleware)
		sensorRoutes.GET("", sensorHandler.GetCurrent)
		sensorRoutes.GET("/moisture-control", sensorHandler.GetMoistureControl)
		sensorRoutes.PATCH("/moisture-control", sensorHandler.UpdateMoistureControl)
		sensorRoutes.GET("/presentation-demo", sensorHandler.GetPresentationDemoStatus)
		sensorRoutes.POST("/presentation-demo", sensorHandler.RunPresentationDemo)

		apiV1.POST("/sensor", sensorHandler.Ingest)

		diagnosisRoutes := apiV1.Group("/diagnosis")
		diagnosisRoutes.Use(authMiddleware)
		diagnosisRoutes.POST("/images", diagnosisHandler.Create)
		diagnosisRoutes.GET("/latest", diagnosisHandler.Latest)
		diagnosisRoutes.GET("/history", diagnosisHandler.History)
		diagnosisRoutes.GET("/artifacts/:id/:kind", diagnosisHandler.Artifact)
	}

	return engine
}

func parsePositiveInt64(value string, fallback int64) int64 {
	var parsed int64
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return fallback
		}
		parsed = parsed*10 + int64(ch-'0')
	}
	if parsed <= 0 {
		return fallback
	}
	return parsed
}
