package services

import (
	"context"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// Use a private type for context keys to avoid collisions
type serviceContextKey string

const (
	ctxKeyProjectID serviceContextKey = "project_id"
)

// ContextWithProject injects the ProjectID into the context
func ContextWithProject(ctx context.Context, id domain.ProjectID) context.Context {
	return context.WithValue(ctx, ctxKeyProjectID, id)
}

// GetProjectFromContext retrieves the ProjectID from the context
func GetProjectFromContext(ctx context.Context) (domain.ProjectID, bool) {
	id, ok := ctx.Value(ctxKeyProjectID).(domain.ProjectID)
	return id, ok
}
