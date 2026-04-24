package webpush

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	externalwebpush "github.com/SherClockHolmes/webpush-go"
	"github.com/apple/kotonoha/backend/internal/models"
)

type Payload struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Source string `json:"source,omitempty"`
	Event  string `json:"event,omitempty"`
	Tag    string `json:"tag,omitempty"`
}

type Sender interface {
	Enabled() bool
	Send(ctx context.Context, subscription models.PushSubscription, payload Payload) error
}

type noopSender struct{}

type vapidSender struct {
	options *externalwebpush.Options
}

func NewSender(publicKey string, privateKey string, subject string) Sender {
	if strings.TrimSpace(publicKey) == "" || strings.TrimSpace(privateKey) == "" || strings.TrimSpace(subject) == "" {
		return noopSender{}
	}

	return &vapidSender{
		options: &externalwebpush.Options{
			Subscriber:      subject,
			VAPIDPublicKey:  publicKey,
			VAPIDPrivateKey: privateKey,
			TTL:             60,
			Urgency:         externalwebpush.UrgencyHigh,
		},
	}
}

func (noopSender) Enabled() bool {
	return false
}

func (noopSender) Send(context.Context, models.PushSubscription, Payload) error {
	return nil
}

func (s *vapidSender) Enabled() bool {
	return s != nil && s.options != nil
}

func (s *vapidSender) Send(ctx context.Context, subscription models.PushSubscription, payload Payload) error {
	target, err := decodeSubscription(subscription)
	if err != nil {
		return err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("Push payload の変換に失敗しました: %w", err)
	}

	pushCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	response, err := externalwebpush.SendNotificationWithContext(pushCtx, body, target, s.options)
	if err != nil {
		return fmt.Errorf("Push通知の送信に失敗しました: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Push通知の送信先がエラーを返しました: status=%d", response.StatusCode)
	}

	return nil
}

func decodeSubscription(subscription models.PushSubscription) (*externalwebpush.Subscription, error) {
	if len(subscription.Subscription) == 0 {
		return nil, errors.New("Push subscription が空です")
	}

	raw, err := json.Marshal(subscription.Subscription)
	if err != nil {
		return nil, fmt.Errorf("Push subscription の変換に失敗しました: %w", err)
	}

	var decoded externalwebpush.Subscription
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("Push subscription の解析に失敗しました: %w", err)
	}

	if strings.TrimSpace(decoded.Endpoint) == "" {
		decoded.Endpoint = strings.TrimSpace(subscription.Endpoint)
	}

	if decoded.Endpoint == "" || decoded.Keys.P256dh == "" || decoded.Keys.Auth == "" {
		return nil, errors.New("Push subscription に必要なキーが不足しています")
	}

	return &decoded, nil
}
