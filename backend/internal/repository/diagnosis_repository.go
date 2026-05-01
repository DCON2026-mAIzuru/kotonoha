package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/apple/kotonoha/backend/internal/models"
)

func (s *PostgresStore) CreateBonsaiDiagnosis(ctx context.Context, input models.CreateBonsaiDiagnosisInput) (*models.BonsaiDiagnosis, error) {
	ratios, err := json.Marshal(input.Ratios)
	if err != nil {
		return nil, fmt.Errorf("診断 ratios の変換に失敗しました: %w", err)
	}
	artifactPaths, err := json.Marshal(input.ArtifactPaths)
	if err != nil {
		return nil, fmt.Errorf("診断 artifact_paths の変換に失敗しました: %w", err)
	}
	rawResult, err := json.Marshal(input.RawResult)
	if err != nil {
		return nil, fmt.Errorf("診断 raw_result の変換に失敗しました: %w", err)
	}

	const query = `
		insert into public.bonsai_diagnoses (
			user_id,
			health_score,
			status,
			status_label,
			severity,
			severity_label,
			detail,
			ratios,
			artifact_paths,
			raw_result
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		returning
			id,
			user_id,
			health_score,
			status,
			status_label,
			severity,
			severity_label,
			detail,
			ratios,
			artifact_paths,
			raw_result,
			created_at
	`

	return scanDiagnosis(s.pool.QueryRow(
		ctx,
		query,
		input.UserID,
		input.HealthScore,
		input.Status,
		input.StatusLabel,
		input.Severity,
		input.SeverityLabel,
		input.Detail,
		ratios,
		artifactPaths,
		rawResult,
	))
}

func (s *PostgresStore) GetLatestBonsaiDiagnosis(ctx context.Context, userID string) (*models.BonsaiDiagnosis, error) {
	const query = `
		select
			id,
			user_id,
			health_score,
			status,
			status_label,
			severity,
			severity_label,
			detail,
			ratios,
			artifact_paths,
			raw_result,
			created_at
		from public.bonsai_diagnoses
		where user_id = $1
		order by created_at desc
		limit 1
	`

	diagnosis, err := scanDiagnosis(s.pool.QueryRow(ctx, query, userID))
	if err != nil {
		if errors.Is(err, ErrNoRows()) {
			return nil, nil
		}
		return nil, err
	}

	return diagnosis, nil
}

func (s *PostgresStore) ListBonsaiDiagnoses(ctx context.Context, userID string, limit int) ([]models.BonsaiDiagnosis, error) {
	const query = `
		select
			id,
			user_id,
			health_score,
			status,
			status_label,
			severity,
			severity_label,
			detail,
			ratios,
			artifact_paths,
			raw_result,
			created_at
		from public.bonsai_diagnoses
		where user_id = $1
		order by created_at desc
		limit $2
	`

	rows, err := s.pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("診断履歴の取得に失敗しました: %w", err)
	}
	defer rows.Close()

	var diagnoses []models.BonsaiDiagnosis
	for rows.Next() {
		diagnosis, err := scanDiagnosis(rows)
		if err != nil {
			return nil, err
		}
		diagnoses = append(diagnoses, *diagnosis)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("診断履歴の読み取りに失敗しました: %w", err)
	}

	return diagnoses, nil
}

func (s *PostgresStore) GetBonsaiDiagnosisByID(ctx context.Context, userID string, id string) (*models.BonsaiDiagnosis, error) {
	const query = `
		select
			id,
			user_id,
			health_score,
			status,
			status_label,
			severity,
			severity_label,
			detail,
			ratios,
			artifact_paths,
			raw_result,
			created_at
		from public.bonsai_diagnoses
		where user_id = $1 and id = $2
	`

	diagnosis, err := scanDiagnosis(s.pool.QueryRow(ctx, query, userID, id))
	if err != nil {
		if errors.Is(err, ErrNoRows()) {
			return nil, nil
		}
		return nil, err
	}

	return diagnosis, nil
}

type diagnosisScanner interface {
	Scan(dest ...any) error
}

func scanDiagnosis(scanner diagnosisScanner) (*models.BonsaiDiagnosis, error) {
	var diagnosis models.BonsaiDiagnosis
	var ratiosRaw []byte
	var artifactPathsRaw []byte
	var rawResultRaw []byte

	if err := scanner.Scan(
		&diagnosis.ID,
		&diagnosis.UserID,
		&diagnosis.HealthScore,
		&diagnosis.Status,
		&diagnosis.StatusLabel,
		&diagnosis.Severity,
		&diagnosis.SeverityLabel,
		&diagnosis.Detail,
		&ratiosRaw,
		&artifactPathsRaw,
		&rawResultRaw,
		&diagnosis.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("診断結果の読み取りに失敗しました: %w", err)
	}

	if len(ratiosRaw) > 0 {
		if err := json.Unmarshal(ratiosRaw, &diagnosis.Ratios); err != nil {
			return nil, fmt.Errorf("診断 ratios の読み取りに失敗しました: %w", err)
		}
	}
	if len(artifactPathsRaw) > 0 {
		if err := json.Unmarshal(artifactPathsRaw, &diagnosis.ArtifactPaths); err != nil {
			return nil, fmt.Errorf("診断 artifact_paths の読み取りに失敗しました: %w", err)
		}
	}
	if len(rawResultRaw) > 0 {
		if err := json.Unmarshal(rawResultRaw, &diagnosis.RawResult); err != nil {
			return nil, fmt.Errorf("診断 raw_result の読み取りに失敗しました: %w", err)
		}
	}

	return &diagnosis, nil
}
