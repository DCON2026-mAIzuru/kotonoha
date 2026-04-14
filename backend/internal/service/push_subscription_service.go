package service

import (
	"context"
	"errors"
	"strings"

	"github.com/apple/kotonoha/backend/internal/models"
)

type pushSubscriptionStore interface {
	ListPushSubscriptionsByUserID(ctx context.Context, userID string) ([]models.PushSubscription, error)
	UpsertPushSubscription(ctx context.Context, input models.UpsertPushSubscriptionInput) (*models.PushSubscription, error)
	DeletePushSubscriptionByID(ctx context.Context, userID string, subscriptionID string) error
}

type PushSubscriptionService struct {
	store pushSubscriptionStore
}

func NewPushSubscriptionService(store pushSubscriptionStore) *PushSubscriptionService {
	return &PushSubscriptionService{store: store}
}

func (s *PushSubscriptionService) List(ctx context.Context, userID string) ([]models.PushSubscription, error) {
	if userID == "" {
		return nil, errors.New("ユーザーIDが空です")
	}

	return s.store.ListPushSubscriptionsByUserID(ctx, userID)
}

func (s *PushSubscriptionService) Upsert(ctx context.Context, input models.UpsertPushSubscriptionInput) (*models.PushSubscription, error) {
	input.Endpoint = strings.TrimSpace(input.Endpoint)

	switch {
	case input.UserID == "":
		return nil, errors.New("ユーザーIDが空です")
	case input.Endpoint == "":
		return nil, errors.New("Push endpoint がありません")
	case len(input.Subscription) == 0:
		return nil, errors.New("Push subscription がありません")
	}

	return s.store.UpsertPushSubscription(ctx, input)
}

func (s *PushSubscriptionService) Delete(ctx context.Context, userID string, subscriptionID string) error {
	userID = strings.TrimSpace(userID)
	subscriptionID = strings.TrimSpace(subscriptionID)

	switch {
	case userID == "":
		return errors.New("ユーザーIDが空です")
	case subscriptionID == "":
		return errors.New("Push subscription ID がありません")
	}

	return s.store.DeletePushSubscriptionByID(ctx, userID, subscriptionID)
}
