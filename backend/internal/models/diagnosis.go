package models

import "time"

type DiagnosisColorRatios struct {
	Green    float64 `json:"green"`
	Yellow   float64 `json:"yellow"`
	BrownRed float64 `json:"brown_red"`
}

type DiagnosisRatios struct {
	Total  DiagnosisColorRatios `json:"total"`
	Top    DiagnosisColorRatios `json:"top"`
	Bottom DiagnosisColorRatios `json:"bottom"`
}

type BonsaiDiagnosis struct {
	ID            string            `json:"id"`
	UserID        string            `json:"user_id"`
	HealthScore   float64           `json:"health_score"`
	Status        string            `json:"status"`
	StatusLabel   string            `json:"status_label"`
	Severity      string            `json:"severity"`
	SeverityLabel string            `json:"severity_label"`
	Detail        string            `json:"detail"`
	Ratios        DiagnosisRatios   `json:"ratios"`
	ArtifactPaths map[string]string `json:"-"`
	ArtifactURLs  map[string]string `json:"artifact_urls,omitempty"`
	RawResult     map[string]any    `json:"raw_result,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
}

type CreateBonsaiDiagnosisInput struct {
	UserID        string
	HealthScore   float64
	Status        string
	StatusLabel   string
	Severity      string
	SeverityLabel string
	Detail        string
	Ratios        DiagnosisRatios
	ArtifactPaths map[string]string
	RawResult     map[string]any
}
