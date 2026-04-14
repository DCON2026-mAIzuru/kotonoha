package service

import (
	"context"
	"errors"
	"strings"

	"github.com/apple/kotonoha/backend/internal/models"
)

type profileStore interface {
	GetBonsaiProfileByUserID(ctx context.Context, userID string) (*models.BonsaiProfile, error)
	UpsertBonsaiProfile(ctx context.Context, input models.UpsertBonsaiProfileInput) (*models.BonsaiProfile, error)
}

type ProfileService struct {
	store profileStore
}

func NewProfileService(store profileStore) *ProfileService {
	return &ProfileService{store: store}
}

func (s *ProfileService) Get(ctx context.Context, userID string) (*models.BonsaiProfile, error) {
	if userID == "" {
		return nil, errors.New("ユーザーIDが空です")
	}

	return s.store.GetBonsaiProfileByUserID(ctx, userID)
}

func (s *ProfileService) Upsert(ctx context.Context, input models.UpsertBonsaiProfileInput) (*models.BonsaiProfile, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Personality = strings.TrimSpace(input.Personality)

	switch {
	case input.UserID == "":
		return nil, errors.New("ユーザーIDが空です")
	case input.Name == "":
		return nil, errors.New("盆栽の名前を入力してください")
	case input.Personality == "":
		return nil, errors.New("性格設定を入力してください")
	case len([]rune(input.Name)) > 50:
		return nil, errors.New("盆栽の名前は50文字以内で入力してください")
	case len([]rune(input.Personality)) > 500:
		return nil, errors.New("性格設定は500文字以内で入力してください")
	}

	return s.store.UpsertBonsaiProfile(ctx, input)
}
