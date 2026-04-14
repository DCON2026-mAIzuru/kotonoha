package service

import "context"

type healthStore interface {
	Ping(ctx context.Context) error
}

type HealthService struct {
	store healthStore
}

func NewHealthService(store healthStore) *HealthService {
	return &HealthService{store: store}
}

func (s *HealthService) Check(ctx context.Context) error {
	return s.store.Ping(ctx)
}
