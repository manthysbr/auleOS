package docker

import (
	"context"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/core/ports"
)

type Manager struct {
	// client *client.Client // Real implementation would have Docker client
}

func NewManager() (*Manager, error) {
	return &Manager{}, nil
}

// Ensure Manager implements WorkerManager
var _ ports.WorkerManager = (*Manager)(nil)

func (m *Manager) Spawn(ctx context.Context, spec domain.WorkerSpec) (domain.WorkerID, error) {
	return "", nil
}

func (m *Manager) HealthCheck(ctx context.Context, id domain.WorkerID) (domain.HealthStatus, error) {
	return domain.HealthStatusUnknown, nil
}

func (m *Manager) Kill(ctx context.Context, id domain.WorkerID) error {
	return nil
}

func (m *Manager) List(ctx context.Context) ([]domain.Worker, error) {
	return []domain.Worker{}, nil
}
