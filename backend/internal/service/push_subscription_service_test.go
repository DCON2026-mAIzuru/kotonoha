package service

import (
	"context"
	"errors"
	"testing"

	"github.com/apple/kotonoha/backend/internal/models"
	pushtransport "github.com/apple/kotonoha/backend/internal/webpush"
)

type pushSubscriptionServiceTestStore struct {
	subscriptions []models.PushSubscription
}

func (s *pushSubscriptionServiceTestStore) ListPushSubscriptionsByUserID(context.Context, string) ([]models.PushSubscription, error) {
	return s.subscriptions, nil
}

func (s *pushSubscriptionServiceTestStore) UpsertPushSubscription(context.Context, models.UpsertPushSubscriptionInput) (*models.PushSubscription, error) {
	return nil, nil
}

func (s *pushSubscriptionServiceTestStore) DeletePushSubscriptionByID(context.Context, string, string) error {
	return nil
}

type pushSubscriptionServiceTestSender struct {
	enabled     bool
	failures    map[string]error
	sendCount   int
	lastPayload pushtransport.Payload
}

func (s *pushSubscriptionServiceTestSender) Enabled() bool {
	return s.enabled
}

func (s *pushSubscriptionServiceTestSender) Send(_ context.Context, subscription models.PushSubscription, payload pushtransport.Payload) error {
	s.sendCount++
	s.lastPayload = payload
	if err, ok := s.failures[subscription.ID]; ok {
		return err
	}
	return nil
}

func TestPushSubscriptionServiceSendDemoRequiresConfiguredSender(t *testing.T) {
	t.Parallel()

	service := NewPushSubscriptionService(&pushSubscriptionServiceTestStore{}, &pushSubscriptionServiceTestSender{})

	_, err := service.SendDemo(context.Background(), "user-1")
	if err == nil || err.Error() != "Push通知送信が設定されていません" {
		t.Fatalf("expected sender configuration error, got %v", err)
	}
}

func TestPushSubscriptionServiceSendDemoRequiresSubscription(t *testing.T) {
	t.Parallel()

	service := NewPushSubscriptionService(&pushSubscriptionServiceTestStore{}, &pushSubscriptionServiceTestSender{enabled: true})

	_, err := service.SendDemo(context.Background(), "user-1")
	if err == nil || err.Error() != "Push購読が登録されていません" {
		t.Fatalf("expected subscription error, got %v", err)
	}
}

func TestPushSubscriptionServiceSendDemoReturnsCounts(t *testing.T) {
	t.Parallel()

	service := NewPushSubscriptionService(&pushSubscriptionServiceTestStore{
		subscriptions: []models.PushSubscription{
			{ID: "sub-1"},
			{ID: "sub-2"},
		},
	}, &pushSubscriptionServiceTestSender{
		enabled: true,
		failures: map[string]error{
			"sub-2": errors.New("boom"),
		},
	})

	result, err := service.SendDemo(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("SendDemo returned error: %v", err)
	}
	if result.SentCount != 1 || result.FailedCount != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
