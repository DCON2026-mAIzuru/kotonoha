package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/apple/kotonoha/backend/internal/models"
)

func (s *PostgresStore) CreateBonsaiMemory(ctx context.Context, input models.CreateBonsaiMemoryInput) error {
	if err := validateMemoryVector(input.Embedding, input.EmbeddingSpace); err != nil {
		return err
	}
	if len(input.SourceMessageIDs) != 2 || input.SourceMessageIDs[0] == "" || input.SourceMessageIDs[1] == "" || input.SourceMessageIDs[0] == input.SourceMessageIDs[1] {
		return errors.New("会話記憶にはユーザー発言と応答の2つの履歴IDが必要です")
	}
	const query = `
		insert into public.bonsai_memories (user_id, summary, source_message_ids, embedding, embedding_space)
		values ($1, $2, $3, $4::vector, $5)
		on conflict do nothing
	`

	if _, err := s.pool.Exec(
		ctx,
		query,
		input.UserID,
		input.Summary,
		input.SourceMessageIDs,
		formatVector(input.Embedding),
		input.EmbeddingSpace,
	); err != nil {
		return fmt.Errorf("長期記憶の保存に失敗しました: %w", err)
	}

	return nil
}

func (s *PostgresStore) MatchBonsaiMemories(ctx context.Context, userID string, embedding []float64, space string, limit int) ([]models.BonsaiMemory, error) {
	if err := validateMemoryVector(embedding, space); err != nil {
		return nil, err
	}
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
		where user_id = $1 and embedding_space = $4
		order by embedding <=> $2::vector
		limit $3
	`

	rows, err := s.pool.Query(ctx, query, userID, formatVector(embedding), limit, space)
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

func validateMemoryVector(values []float64, space string) error {
	if strings.TrimSpace(space) == "" || space == "legacy:unknown" || len(space) > 200 {
		return errors.New("記憶の埋め込み空間が設定されていません")
	}
	if len(values) != 3072 {
		return errors.New("記憶の埋め込みは3072次元である必要があります")
	}
	var norm float64
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("記憶の埋め込みに有限でない値があります")
		}
		norm += value * value
	}
	if norm == 0 || math.IsInf(norm, 0) {
		return errors.New("記憶の埋め込みのノルムが不正です")
	}
	return nil
}

// 返信関係・既存記憶の出典を優先する。旧履歴の隣接推定は明示的に許可された場合だけ行う。
func (s *PostgresStore) ListUnembeddedConversationPairs(ctx context.Context, userID, space string, limit int, legacyAdjacent bool) ([]models.ConversationMemoryPair, error) {
	if strings.TrimSpace(space) == "" || space == "legacy:unknown" {
		return nil, errors.New("取り込み先の埋め込み空間が必要です")
	}
	if limit <= 0 || limit > 1000 {
		return nil, errors.New("取り込み件数は1から1000で指定してください")
	}
	const query = `
        with ordered as (
            select user_id, id, role, content, created_at,
                lead(id) over history_order as assistant_id,
                lead(role) over history_order as assistant_role,
                lead(content) over history_order as assistant_content,
                lead(reply_to_message_id) over history_order as assistant_reply_to
            from public.chat_history
            where ($1 = '' or user_id = nullif($1, '')::uuid)
            window history_order as (partition by user_id order by created_at, id)
        ), known_links as (
            select a.user_id, a.reply_to_message_id as user_message_id, a.id as assistant_id
            from public.chat_history a
            where a.reply_to_message_id is not null and a.role = 'assistant'
            union
            select m.user_id, m.source_message_ids[1], m.source_message_ids[2]
            from public.bonsai_memories m
            join public.chat_history a on a.id = m.source_message_ids[2]
            where cardinality(m.source_message_ids) = 2 and a.reply_to_message_id is null
        ), verified as (
            select distinct u.user_id, u.id, a.id as assistant_id, u.content,
                a.content as assistant_content, u.created_at
            from known_links k
            join public.chat_history u on u.id = k.user_message_id and u.user_id = k.user_id and u.role = 'user'
            join public.chat_history a on a.id = k.assistant_id and a.user_id = u.user_id and a.role = 'assistant'
            where ($1 = '' or u.user_id = nullif($1, '')::uuid)
                and not exists (select 1 from known_links other where other.user_id = k.user_id
                    and other.assistant_id = k.assistant_id and other.user_message_id <> k.user_message_id)
        ), candidates as (
            select * from verified
            union all
            select user_id, id, assistant_id, content, assistant_content, created_at from ordered
            where $4::boolean and role = 'user' and assistant_role = 'assistant'
                and assistant_reply_to is null
                and not exists (select 1 from known_links k where k.user_id = ordered.user_id and k.assistant_id = ordered.assistant_id)
        )
        select user_id, id, assistant_id, content, assistant_content
        from candidates
        where btrim(content) <> '' and btrim(assistant_content) <> ''
            and not exists (
                select 1 from public.bonsai_memories m
                where m.user_id = candidates.user_id and m.embedding_space = $2
                    and m.source_message_ids[2] = candidates.assistant_id
            )
        order by created_at, id limit $3
    `
	rows, err := s.pool.Query(ctx, query, userID, space, limit, legacyAdjacent)
	if err != nil {
		return nil, fmt.Errorf("未取り込み会話の検索に失敗しました: %w", err)
	}
	defer rows.Close()
	pairs := make([]models.ConversationMemoryPair, 0)
	for rows.Next() {
		var pair models.ConversationMemoryPair
		if err := rows.Scan(&pair.UserID, &pair.UserMessageID, &pair.AssistantMessageID, &pair.UserText, &pair.AssistantText); err != nil {
			return nil, fmt.Errorf("未取り込み会話の読み取りに失敗しました: %w", err)
		}
		pairs = append(pairs, pair)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("未取り込み会話の走査に失敗しました: %w", err)
	}
	return pairs, nil
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
