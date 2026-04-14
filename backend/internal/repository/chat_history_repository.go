package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/apple/kotonoha/backend/internal/models"
)

func (s *PostgresStore) CreateChatHistoryEntry(ctx context.Context, input models.CreateChatHistoryEntryInput) (*models.ChatHistoryEntry, error) {
	payload, err := json.Marshal(input.SensorSnapshot)
	if err != nil {
		return nil, fmt.Errorf("会話履歴 sensor_snapshot の変換に失敗しました: %w", err)
	}

	const query = `
		insert into public.chat_history (user_id, role, content, sensor_snapshot)
		values ($1, $2, $3, $4)
		returning id, user_id, role, content, sensor_snapshot, created_at
	`

	var entry models.ChatHistoryEntry
	var raw []byte
	if err := s.pool.QueryRow(ctx, query, input.UserID, input.Role, input.Content, payload).Scan(
		&entry.ID,
		&entry.UserID,
		&entry.Role,
		&entry.Content,
		&raw,
		&entry.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("会話履歴の保存に失敗しました: %w", err)
	}

	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &entry.SensorSnapshot); err != nil {
			return nil, fmt.Errorf("会話履歴 sensor_snapshot の読み取りに失敗しました: %w", err)
		}
	}

	return &entry, nil
}

func (s *PostgresStore) GetLatestChatHistoryByRole(ctx context.Context, userID string, role string) (*models.ChatHistoryEntry, error) {
	const query = `
		select id, user_id, role, content, sensor_snapshot, created_at
		from public.chat_history
		where user_id = $1 and role = $2
		order by created_at desc
		limit 1
	`

	var entry models.ChatHistoryEntry
	var raw []byte
	if err := s.pool.QueryRow(ctx, query, userID, role).Scan(
		&entry.ID,
		&entry.UserID,
		&entry.Role,
		&entry.Content,
		&raw,
		&entry.CreatedAt,
	); err != nil {
		if errors.Is(err, ErrNoRows()) {
			return nil, nil
		}

		return nil, fmt.Errorf("最新の会話履歴の取得に失敗しました: %w", err)
	}

	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &entry.SensorSnapshot); err != nil {
			return nil, fmt.Errorf("会話履歴 sensor_snapshot の変換に失敗しました: %w", err)
		}
	}

	return &entry, nil
}

func (s *PostgresStore) ListRecentChatHistoryByUserID(ctx context.Context, userID string, limit int) ([]models.ChatHistoryEntry, error) {
	if limit <= 0 {
		limit = 6
	}

	const query = `
		select id, user_id, role, content, sensor_snapshot, created_at
		from public.chat_history
		where user_id = $1
		order by created_at desc
		limit $2
	`

	rows, err := s.pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("最近の会話履歴の取得に失敗しました: %w", err)
	}
	defer rows.Close()

	entries := make([]models.ChatHistoryEntry, 0, limit)
	for rows.Next() {
		var entry models.ChatHistoryEntry
		var raw []byte
		if err := rows.Scan(
			&entry.ID,
			&entry.UserID,
			&entry.Role,
			&entry.Content,
			&raw,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("最近の会話履歴の読み取りに失敗しました: %w", err)
		}

		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &entry.SensorSnapshot); err != nil {
				return nil, fmt.Errorf("最近の会話履歴 sensor_snapshot の変換に失敗しました: %w", err)
			}
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("最近の会話履歴の走査に失敗しました: %w", err)
	}

	slices.Reverse(entries)

	return entries, nil
}
