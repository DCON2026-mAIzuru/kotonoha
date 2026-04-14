package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/apple/kotonoha/backend/internal/models"
)

func (s *PostgresStore) ListPushSubscriptionsByUserID(ctx context.Context, userID string) ([]models.PushSubscription, error) {
	const query = `
		select id, user_id, endpoint, subscription, created_at, updated_at
		from public.push_subscriptions
		where user_id = $1
		order by created_at desc
	`

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("Push購読一覧の取得に失敗しました: %w", err)
	}
	defer rows.Close()

	subscriptions := make([]models.PushSubscription, 0)
	for rows.Next() {
		var subscription models.PushSubscription
		var raw []byte
		if err := rows.Scan(
			&subscription.ID,
			&subscription.UserID,
			&subscription.Endpoint,
			&raw,
			&subscription.CreatedAt,
			&subscription.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("Push購読一覧の読み取りに失敗しました: %w", err)
		}

		if err := json.Unmarshal(raw, &subscription.Subscription); err != nil {
			return nil, fmt.Errorf("Push購読JSONの変換に失敗しました: %w", err)
		}

		subscriptions = append(subscriptions, subscription)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("Push購読一覧の走査に失敗しました: %w", err)
	}

	return subscriptions, nil
}

func (s *PostgresStore) UpsertPushSubscription(ctx context.Context, input models.UpsertPushSubscriptionInput) (*models.PushSubscription, error) {
	payload, err := json.Marshal(input.Subscription)
	if err != nil {
		return nil, fmt.Errorf("Push購読JSONの変換に失敗しました: %w", err)
	}

	const query = `
		insert into public.push_subscriptions (user_id, endpoint, subscription)
		values ($1, $2, $3::jsonb)
		on conflict (user_id, endpoint)
		do update set
			subscription = excluded.subscription
		returning id, user_id, endpoint, subscription, created_at, updated_at
	`

	var subscription models.PushSubscription
	var raw []byte
	if err := s.pool.QueryRow(ctx, query, input.UserID, input.Endpoint, string(payload)).Scan(
		&subscription.ID,
		&subscription.UserID,
		&subscription.Endpoint,
		&raw,
		&subscription.CreatedAt,
		&subscription.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("Push購読の保存に失敗しました: %w", err)
	}

	if err := json.Unmarshal(raw, &subscription.Subscription); err != nil {
		return nil, fmt.Errorf("Push購読JSONの読み取りに失敗しました: %w", err)
	}

	return &subscription, nil
}

func (s *PostgresStore) DeletePushSubscriptionByID(ctx context.Context, userID string, subscriptionID string) error {
	const query = `
		delete from public.push_subscriptions
		where id = $1 and user_id = $2
	`

	commandTag, err := s.pool.Exec(ctx, query, subscriptionID, userID)
	if err != nil {
		return fmt.Errorf("Push購読の削除に失敗しました: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return nil
	}

	return nil
}
