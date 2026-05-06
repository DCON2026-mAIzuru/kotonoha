package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	BackendPort            string
	BackendAllowedOrigin   string
	SupabaseURL            string
	SupabasePublishableKey string
	SupabaseSecretKey      string
	SupabaseDBURL          string
	SupabaseJWTSecret      string
	SensorSharedToken      string
	PresentationDemoMode   string
	GeminiAPIKey           string
	GeminiModel            string
	GeminiModelPriority    string
	GeminiEmbeddingModel   string
	VapidPublicKey         string
	VapidPrivateKey        string
	VapidSubject           string
	DLServiceURL           string
	DLServiceTimeout       string
	DiagnosisMaxImageMB    string
	DiagnosisArtifactDir   string
	VoicevoxEngineURL      string
	VoicevoxSpeakerID      string
	VoicevoxSpeakerName    string
	VoicevoxStyleName      string
	VoicevoxSpeedScale     string
	VoicevoxTimeout        string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		BackendPort:            getEnv("BACKEND_PORT", "8080"),
		BackendAllowedOrigin:   getEnv("BACKEND_ALLOWED_ORIGIN", "http://localhost:3000"),
		SupabaseURL:            os.Getenv("SUPABASE_URL"),
		SupabasePublishableKey: os.Getenv("SUPABASE_PUBLISHABLE_KEY"),
		SupabaseSecretKey:      os.Getenv("SUPABASE_SECRET_KEY"),
		SupabaseDBURL:          os.Getenv("SUPABASE_DB_URL"),
		SupabaseJWTSecret:      os.Getenv("SUPABASE_JWT_SECRET"),
		SensorSharedToken:      os.Getenv("SENSOR_SHARED_TOKEN"),
		PresentationDemoMode:   getEnv("PRESENTATION_DEMO_MODE", getEnv("NEXT_PUBLIC_PRESENTATION_DEMO_MODE", "false")),
		GeminiAPIKey:           os.Getenv("GEMINI_API_KEY"),
		GeminiModel:            getEnv("GEMINI_MODEL", "gemini-2.5-flash"),
		GeminiModelPriority:    os.Getenv("GEMINI_MODEL_PRIORITY"),
		GeminiEmbeddingModel:   getEnv("GEMINI_EMBEDDING_MODEL", "gemini-embedding-001"),
		VapidPublicKey:         os.Getenv("VAPID_PUBLIC_KEY"),
		VapidPrivateKey:        os.Getenv("VAPID_PRIVATE_KEY"),
		VapidSubject:           os.Getenv("VAPID_SUBJECT"),
		DLServiceURL:           getEnv("DL_SERVICE_URL", "http://127.0.0.1:8090"),
		DLServiceTimeout:       getEnv("DL_SERVICE_TIMEOUT_SECONDS", "90"),
		DiagnosisMaxImageMB:    getEnv("DIAGNOSIS_MAX_IMAGE_MB", "8"),
		DiagnosisArtifactDir:   getEnv("DIAGNOSIS_ARTIFACT_DIR", "/tmp/kotonoha-diagnosis-artifacts"),
		VoicevoxEngineURL:      getEnv("VOICEVOX_ENGINE_URL", "http://127.0.0.1:50021"),
		VoicevoxSpeakerID:      getEnv("VOICEVOX_SPEAKER_ID", "13"),
		VoicevoxSpeakerName:    getEnv("VOICEVOX_SPEAKER_NAME", "青山龍星"),
		VoicevoxStyleName:      getEnv("VOICEVOX_STYLE_NAME", "ノーマル"),
		VoicevoxSpeedScale:     getEnv("VOICEVOX_SPEED_SCALE", "1.12"),
		VoicevoxTimeout:        getEnv("VOICEVOX_TIMEOUT_SECONDS", "30"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	required := map[string]string{
		"SUPABASE_URL":             c.SupabaseURL,
		"SUPABASE_PUBLISHABLE_KEY": c.SupabasePublishableKey,
		"SUPABASE_SECRET_KEY":      c.SupabaseSecretKey,
		"SUPABASE_DB_URL":          c.SupabaseDBURL,
	}

	var missing []string
	for key, value := range required {
		if value == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("必須の環境変数が不足しています: %v", missing)
	}

	return nil
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
