package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/apple/kotonoha/backend/internal/api/middleware"
	"github.com/apple/kotonoha/backend/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	sensorTokenHeader  = "X-Sensor-Token"
	sensorUserIDHeader = "X-Sensor-User-ID"
)

type SensorHandler struct {
	sensorService     *service.SensorService
	sensorSharedToken string
}

func NewSensorHandler(sensorService *service.SensorService, sensorSharedToken string) *SensorHandler {
	return &SensorHandler{
		sensorService:     sensorService,
		sensorSharedToken: strings.TrimSpace(sensorSharedToken),
	}
}

func (h *SensorHandler) GetCurrent(c *gin.Context) {
	if h.sensorService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "センサーサービスが初期化されていません"})
		return
	}

	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}

	sensorData, err := h.sensorService.GetCurrent(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sensor_data": sensorData,
	})
}

func (h *SensorHandler) GetMoistureControl(c *gin.Context) {
	if h.sensorService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "センサーサービスが初期化されていません"})
		return
	}

	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}

	control, err := h.sensorService.GetMoistureControl(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"moisture_control": control,
	})
}

func (h *SensorHandler) UpdateMoistureControl(c *gin.Context) {
	if h.sensorService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "センサーサービスが初期化されていません"})
		return
	}

	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証ユーザーが見つかりません"})
		return
	}

	var payload struct {
		Paused      bool     `json:"paused"`
		ManualValue *float64 `json:"manual_value"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "リクエストの形式が不正です"})
		return
	}

	control, err := h.sensorService.UpdateMoistureControl(c.Request.Context(), user.ID, service.UpdateMoistureControlInput{
		Paused:      payload.Paused,
		ManualValue: payload.ManualValue,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"moisture_control": control,
	})
}

func (h *SensorHandler) Ingest(c *gin.Context) {
	if h.sensorService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "センサーサービスが初期化されていません"})
		return
	}
	if h.sensorSharedToken == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "SENSOR_SHARED_TOKEN が設定されていません"})
		return
	}
	if strings.TrimSpace(c.GetHeader(sensorTokenHeader)) != h.sensorSharedToken {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "センサートークンを検証できませんでした"})
		return
	}

	var input service.IngestSensorInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "リクエストの形式が不正です"})
		return
	}
	input.UserID = strings.TrimSpace(c.GetHeader(sensorUserIDHeader))
	if input.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "X-Sensor-User-ID がありません"})
		return
	}

	result, err := h.sensorService.Ingest(c.Request.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSensorReadingOutOfOrder):
			c.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, result)
}
