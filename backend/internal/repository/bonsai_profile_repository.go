package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/apple/kotonoha/backend/internal/models"
)

func (s *PostgresStore) GetBonsaiProfileByUserID(ctx context.Context, userID string) (*models.BonsaiProfile, error) {
	const query = `
		select user_id, name, personality, backstory, tone_mode, bond_score, bond_updated_at, created_at, updated_at
		from public.bonsai_profiles
		where user_id = $1
	`

	var profile models.BonsaiProfile
	if err := s.pool.QueryRow(ctx, query, userID).Scan(
		&profile.UserID,
		&profile.Name,
		&profile.Personality,
		&profile.Backstory,
		&profile.ToneMode,
		&profile.BondScore,
		&profile.BondUpdatedAt,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	); err != nil {
		if errors.Is(err, ErrNoRows()) {
			return nil, nil
		}

		return nil, fmt.Errorf("盆栽プロフィールの取得に失敗しました: %w", err)
	}

	return &profile, nil
}

func (s *PostgresStore) UpsertBonsaiProfile(ctx context.Context, input models.UpsertBonsaiProfileInput) (*models.BonsaiProfile, error) {
	const query = `
		insert into public.bonsai_profiles (user_id, name, personality, backstory, tone_mode)
		values ($1, $2, $3, $4, $5)
		on conflict (user_id)
		do update set
			name = excluded.name,
			personality = excluded.personality,
			backstory = excluded.backstory,
			tone_mode = excluded.tone_mode
		returning user_id, name, personality, backstory, tone_mode, bond_score, bond_updated_at, created_at, updated_at
	`

	var profile models.BonsaiProfile
	if err := s.pool.QueryRow(ctx, query, input.UserID, input.Name, input.Personality, input.Backstory, input.ToneMode).Scan(
		&profile.UserID,
		&profile.Name,
		&profile.Personality,
		&profile.Backstory,
		&profile.ToneMode,
		&profile.BondScore,
		&profile.BondUpdatedAt,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("盆栽プロフィールの保存に失敗しました: %w", err)
	}

	return &profile, nil
}

func (s *PostgresStore) UpdateBonsaiBondScore(ctx context.Context, userID string, score int) (*models.BonsaiProfile, error) {
	const query = `
		update public.bonsai_profiles
		set bond_score = $2,
			bond_updated_at = timezone('utc', now())
		where user_id = $1
		returning user_id, name, personality, backstory, tone_mode, bond_score, bond_updated_at, created_at, updated_at
	`

	var profile models.BonsaiProfile
	if err := s.pool.QueryRow(ctx, query, userID, score).Scan(
		&profile.UserID,
		&profile.Name,
		&profile.Personality,
		&profile.Backstory,
		&profile.ToneMode,
		&profile.BondScore,
		&profile.BondUpdatedAt,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("なじみ具合の更新に失敗しました: %w", err)
	}

	return &profile, nil
}
