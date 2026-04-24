package api

import (
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
	sensorHandler := handlers.NewSensorHandler(deps.SensorService, deps.Config.SensorSharedToken)
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

		sensorRoutes := apiV1.Group("/sensor")
		sensorRoutes.Use(authMiddleware)
		sensorRoutes.GET("", sensorHandler.GetCurrent)
		sensorRoutes.GET("/moisture-control", sensorHandler.GetMoistureControl)
		sensorRoutes.PATCH("/moisture-control", sensorHandler.UpdateMoistureControl)

		apiV1.POST("/sensor", sensorHandler.Ingest)
	}

	return engine
}
