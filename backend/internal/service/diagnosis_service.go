package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/apple/kotonoha/backend/internal/dlclient"
	"github.com/apple/kotonoha/backend/internal/models"
)

const (
	defaultDiagnosisHistoryLimit = 20
	maxDiagnosisHistoryLimit     = 50
)

type diagnosisStore interface {
	CreateBonsaiDiagnosis(ctx context.Context, input models.CreateBonsaiDiagnosisInput) (*models.BonsaiDiagnosis, error)
	GetLatestBonsaiDiagnosis(ctx context.Context, userID string) (*models.BonsaiDiagnosis, error)
	ListBonsaiDiagnoses(ctx context.Context, userID string, limit int) ([]models.BonsaiDiagnosis, error)
	GetBonsaiDiagnosisByID(ctx context.Context, userID string, id string) (*models.BonsaiDiagnosis, error)
}

type diagnosisDLClient interface {
	Diagnose(ctx context.Context, requestID string, filename string, contentType string, image []byte) (*dlclient.DiagnosisResult, error)
	DownloadArtifact(ctx context.Context, requestID string, kind string) ([]byte, error)
}

type DiagnosisService struct {
	store       diagnosisStore
	dlClient    diagnosisDLClient
	maxImageMB  int64
	artifactDir string
}

type RunDiagnosisInput struct {
	UserID      string
	Filename    string
	ContentType string
	Image       []byte
}

func NewDiagnosisService(store diagnosisStore, dlClient diagnosisDLClient, maxImageMB int64, artifactDir string) *DiagnosisService {
	if maxImageMB <= 0 {
		maxImageMB = 8
	}
	if artifactDir == "" {
		artifactDir = "/tmp/kotonoha-diagnosis-artifacts"
	}
	return &DiagnosisService{
		store:       store,
		dlClient:    dlClient,
		maxImageMB:  maxImageMB,
		artifactDir: artifactDir,
	}
}

func (s *DiagnosisService) Run(ctx context.Context, input RunDiagnosisInput) (*models.BonsaiDiagnosis, error) {
	if s == nil || s.store == nil || s.dlClient == nil {
		return nil, errors.New("診断サービスが設定されていません")
	}

	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return nil, errors.New("ユーザーIDが空です")
	}
	if len(input.Image) == 0 {
		return nil, errors.New("画像ファイルが空です")
	}
	if int64(len(input.Image)) > s.maxImageMB*1024*1024 {
		return nil, fmt.Errorf("画像は %dMB 以下にしてください", s.maxImageMB)
	}
	if !allowedDiagnosisContentType(input.ContentType) {
		return nil, errors.New("jpg/png/webp 画像を指定してください")
	}

	requestID, err := randomID()
	if err != nil {
		return nil, err
	}

	result, err := s.dlClient.Diagnose(ctx, requestID, input.Filename, input.ContentType, input.Image)
	if err != nil {
		return nil, err
	}

	artifactPaths, err := s.saveArtifacts(ctx, requestID, result.Artifacts)
	if err != nil {
		return nil, err
	}

	diagnosis, err := s.store.CreateBonsaiDiagnosis(ctx, models.CreateBonsaiDiagnosisInput{
		UserID:        userID,
		HealthScore:   result.HealthScore,
		Status:        result.Status,
		StatusLabel:   result.StatusLabel,
		Severity:      result.Severity,
		SeverityLabel: result.SeverityLabel,
		Detail:        result.Detail,
		Ratios:        result.Ratios,
		ArtifactPaths: artifactPaths,
		RawResult: map[string]any{
			"request_id":   result.RequestID,
			"raw_stdout":   result.RawStdout,
			"dl_artifacts": result.Artifacts,
		},
	})
	if err != nil {
		return nil, err
	}

	return withArtifactURLs(diagnosis), nil
}

func (s *DiagnosisService) Latest(ctx context.Context, userID string) (*models.BonsaiDiagnosis, error) {
	diagnosis, err := s.store.GetLatestBonsaiDiagnosis(ctx, strings.TrimSpace(userID))
	if err != nil || diagnosis == nil {
		return diagnosis, err
	}
	return withArtifactURLs(diagnosis), nil
}

func (s *DiagnosisService) History(ctx context.Context, userID string, limit int) ([]models.BonsaiDiagnosis, error) {
	if limit <= 0 {
		limit = defaultDiagnosisHistoryLimit
	}
	if limit > maxDiagnosisHistoryLimit {
		limit = maxDiagnosisHistoryLimit
	}

	diagnoses, err := s.store.ListBonsaiDiagnoses(ctx, strings.TrimSpace(userID), limit)
	if err != nil {
		return nil, err
	}
	for i := range diagnoses {
		withArtifactURLs(&diagnoses[i])
	}
	return diagnoses, nil
}

func (s *DiagnosisService) ArtifactPath(ctx context.Context, userID string, id string, kind string) (string, error) {
	diagnosis, err := s.store.GetBonsaiDiagnosisByID(ctx, strings.TrimSpace(userID), strings.TrimSpace(id))
	if err != nil {
		return "", err
	}
	if diagnosis == nil {
		return "", errors.New("診断結果が見つかりません")
	}

	path := diagnosis.ArtifactPaths[kind]
	if path == "" {
		return "", errors.New("可視化画像が見つかりません")
	}
	if _, err := os.Stat(path); err != nil {
		return "", errors.New("可視化画像ファイルが見つかりません")
	}

	return path, nil
}

func (s *DiagnosisService) saveArtifacts(ctx context.Context, requestID string, artifacts map[string]string) (map[string]string, error) {
	saved := make(map[string]string)
	if len(artifacts) == 0 {
		return saved, nil
	}

	dir := filepath.Join(s.artifactDir, requestID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("可視化画像ディレクトリを作成できません: %w", err)
	}

	for kind := range artifacts {
		if !allowedArtifactKind(kind) {
			continue
		}
		payload, err := s.dlClient.DownloadArtifact(ctx, requestID, kind)
		if err != nil {
			return nil, err
		}
		path := filepath.Join(dir, kind+".jpg")
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			return nil, fmt.Errorf("可視化画像を保存できません: %w", err)
		}
		saved[kind] = path
	}

	return saved, nil
}

func allowedDiagnosisContentType(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func allowedArtifactKind(kind string) bool {
	switch kind {
	case "yolo_detection", "sam_segmentation", "color_extraction":
		return true
	default:
		return false
	}
}

func withArtifactURLs(diagnosis *models.BonsaiDiagnosis) *models.BonsaiDiagnosis {
	if diagnosis == nil {
		return nil
	}
	urls := make(map[string]string)
	for kind := range diagnosis.ArtifactPaths {
		if allowedArtifactKind(kind) {
			urls[kind] = "/api/v1/diagnosis/artifacts/" + diagnosis.ID + "/" + kind
		}
	}
	diagnosis.ArtifactURLs = urls
	return diagnosis
}

func randomID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("request_id を生成できません: %w", err)
	}
	return hex.EncodeToString(bytes[:]), nil
}

func DetectContentType(payload []byte) string {
	if len(payload) > 512 {
		payload = payload[:512]
	}
	return http.DetectContentType(payload)
}
