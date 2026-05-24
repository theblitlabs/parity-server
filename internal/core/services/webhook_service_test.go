package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-server/internal/core/models"
)

type staticTaskServicer struct {
	tasks []*models.Task
}

func (s *staticTaskServicer) ListAvailableTasks(ctx context.Context) ([]*models.Task, error) {
	return s.tasks, nil
}

func TestNotifyTaskUpdateDrainsSignalChannel(t *testing.T) {
	service := NewWebhookService(&staticTaskServicer{
		tasks: []*models.Task{
			{ID: uuid.New(), Title: "queued"},
		},
	})

	stopCh := make(chan struct{})
	service.SetStopChannel(stopCh)
	t.Cleanup(func() {
		close(stopCh)
	})

	for i := 0; i < 3; i++ {
		service.NotifyTaskUpdate()
	}

	deadline := time.Now().Add(2 * time.Second)
	for len(service.taskUpdateCh) != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if got := len(service.taskUpdateCh); got != 0 {
		t.Fatalf("taskUpdateCh length = %d, want 0", got)
	}
}
