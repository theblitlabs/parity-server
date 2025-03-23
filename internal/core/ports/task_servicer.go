package ports

import (
	"context"

	"github.com/theblitlabs/parity-server/internal/core/models"
)

type TaskServicer interface {
	ListAvailableTasks(ctx context.Context) ([]*models.Task, error)
}
