package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/apple/kotonoha/backend/internal/models"
	pushtransport "github.com/apple/kotonoha/backend/internal/webpush"
)

type pushSubscriptionStore interface {
	ListPushSubscriptionsByUserID(ctx context.Context, userID string) ([]models.PushSubscription, error)
	UpsertPushSubscription(ctx context.Context, input models.UpsertPushSubscriptionInput) (*models.PushSubscription, error)
	DeletePushSubscriptionByID(ctx context.Context, userID string, subscriptionID string) error
	GetBonsaiProfileByUserID(ctx context.Context, userID string) (*models.BonsaiProfile, error)
}

type pushSubscriptionSender interface {
	Enabled() bool
	Send(ctx context.Context, subscription models.PushSubscription, payload pushtransport.Payload) error
}

type SendPushDemoResult struct {
	SentCount   int `json:"sent_count"`
	FailedCount int `json:"failed_count,omitempty"`
}

type PushSubscriptionService struct {
	store      pushSubscriptionStore
	pushSender pushSubscriptionSender
}

func NewPushSubscriptionService(store pushSubscriptionStore, pushSender pushSubscriptionSender) *PushSubscriptionService {
	return &PushSubscriptionService{
		store:      store,
		pushSender: pushSender,
	}
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

func (s *PushSubscriptionService) SendDemo(ctx context.Context, userID string) (*SendPushDemoResult, error) {
	userID = strings.TrimSpace(userID)

	switch {
	case userID == "":
		return nil, errors.New("ユーザーIDが空です")
	case s.pushSender == nil || !s.pushSender.Enabled():
		return nil, errors.New("Push通知送信が設定されていません")
	}

	subscriptions, err := s.store.ListPushSubscriptionsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(subscriptions) == 0 {
		return nil, errors.New("Push購読が登録されていません")
	}

	profile, err := s.store.GetBonsaiProfileByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := &SendPushDemoResult{}
	title, body, event := buildDemoPushNotification(profile)
	payload := pushtransport.Payload{
		Title:  title,
		Body:   body,
		Source: "push_demo",
		Event:  event,
		Tag:    "push-demo-" + event,
	}

	for _, subscription := range subscriptions {
		if err := s.pushSender.Send(ctx, subscription, payload); err != nil {
			result.FailedCount++
			log.Printf("Pushデモ通知の送信に失敗しました: subscription_id=%s err=%v", subscription.ID, err)
			continue
		}
		result.SentCount++
	}

	if result.SentCount == 0 {
		return nil, errors.New("Push通知の送信に失敗しました")
	}

	return result, nil
}

func buildDemoPushNotification(profile *models.BonsaiProfile) (string, string, string) {
	name := "盆栽"
	if profile != nil {
		if trimmed := strings.TrimSpace(profile.Name); trimmed != "" {
			name = trimmed
		}
	}

	title := fmt.Sprintf("%s からのお知らせ", name)
	return title, "お水が足りないぼんす。少しお水をくださいぼんす。", "demo_low_moisture"
}
