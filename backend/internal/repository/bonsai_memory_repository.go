package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/apple/kotonoha/backend/internal/models"
)

func (s *PostgresStore) CreateBonsaiMemory(ctx context.Context, input models.CreateBonsaiMemoryInput) error {
	const query = `
		insert into public.bonsai_memories (user_id, summary, source_message_ids, embedding)
		values ($1, $2, $3, $4::vector)
	`

	if _, err := s.pool.Exec(
		ctx,
		query,
		input.UserID,
		input.Summary,
		input.SourceMessageIDs,
		formatVector(input.Embedding),
	); err != nil {
		return fmt.Errorf("長期記憶の保存に失敗しました: %w", err)
	}

	return nil
}

func (s *PostgresStore) MatchBonsaiMemories(ctx context.Context, userID string, embedding []float64, limit int) ([]models.BonsaiMemory, error) {
	if limit <= 0 {
		limit = 3
	}

	const query = `
		select
			id,
			user_id,
			summary,
			1 - (embedding <=> $2::vector) as similarity,
			created_at
		from public.bonsai_memories
		where user_id = $1
		order by embedding <=> $2::vector
		limit $3
	`

	rows, err := s.pool.Query(ctx, query, userID, formatVector(embedding), limit)
	if err != nil {
		return nil, fmt.Errorf("長期記憶の検索に失敗しました: %w", err)
	}
	defer rows.Close()

	memories := make([]models.BonsaiMemory, 0)
	for rows.Next() {
		var memory models.BonsaiMemory
		if err := rows.Scan(
			&memory.ID,
			&memory.UserID,
			&memory.Summary,
			&memory.Similarity,
			&memory.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("長期記憶の読み取りに失敗しました: %w", err)
		}

		memories = append(memories, memory)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("長期記憶の走査に失敗しました: %w", err)
	}

	return memories, nil
}

func (s *PostgresStore) DeleteBonsaiMemoriesByUserID(ctx context.Context, userID string) (int64, error) {
	const query = `
		delete from public.bonsai_memories
		where user_id = $1
	`

	tag, err := s.pool.Exec(ctx, query, userID)
	if err != nil {
		return 0, fmt.Errorf("長期記憶の削除に失敗しました: %w", err)
	}

	return tag.RowsAffected(), nil
}

func formatVector(values []float64) string {
	if len(values) == 0 {
		return "[]"
	}

	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.FormatFloat(value, 'f', -1, 64))
	}

	return "[" + strings.Join(parts, ",") + "]"
}
