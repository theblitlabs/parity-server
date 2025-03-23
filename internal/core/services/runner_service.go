package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/models"
)

var (
	ErrRunnerNotFound = errors.New("runner not found")
)

type RunnerRepository interface {
	Create(ctx context.Context, runner *models.Runner) error
	Get(ctx context.Context, deviceID string) (*models.Runner, error)
	CreateOrUpdate(ctx context.Context, runner *models.Runner) (*models.Runner, error)
	Update(ctx context.Context, runner *models.Runner) (*models.Runner, error)
	ListByStatus(ctx context.Context, status models.RunnerStatus) ([]*models.Runner, error)
	UpdateRunnersToOffline(ctx context.Context, heartbeatTimeout time.Duration) (int64, []string, error)
}

type RunnerService struct {
	repo             RunnerRepository
	taskService      *TaskService
	heartbeatTimeout time.Duration
	taskMonitorCh    chan struct{}
}

func NewRunnerService(repo RunnerRepository) *RunnerService {
	return &RunnerService{
		repo:             repo,
		heartbeatTimeout: 2 * time.Minute,
		taskMonitorCh:    make(chan struct{}, 10),
	}
}

func (s *RunnerService) SetTaskService(taskService *TaskService) {
	s.taskService = taskService

	go s.taskMonitorWorker()
}

func (s *RunnerService) taskMonitorWorker() {
	var timer *time.Timer
	for range s.taskMonitorCh {
		if timer != nil {
			timer.Stop()
		}

		timer = time.AfterFunc(100*time.Millisecond, func() {
			if s.taskService != nil {
				s.taskService.MonitorTasks()
			}
		})
	}
}

func (s *RunnerService) triggerTaskMonitor() {
	select {
	case s.taskMonitorCh <- struct{}{}:
	default:
	}
}

func (s *RunnerService) SetHeartbeatTimeout(timeout time.Duration) {
	s.heartbeatTimeout = timeout
}

func (s *RunnerService) CreateRunner(ctx context.Context, runner *models.Runner) error {
	return s.repo.Create(ctx, runner)
}

func (s *RunnerService) GetRunner(ctx context.Context, deviceID string) (*models.Runner, error) {
	return s.repo.Get(ctx, deviceID)
}

func (s *RunnerService) CreateOrUpdateRunner(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	existingRunner, err := s.repo.Get(ctx, runner.DeviceID)

	isNewOrBecomingAvailable := false

	if err != nil {
		isNewOrBecomingAvailable = runner.Status == models.RunnerStatusOnline
	} else {
		isNewOrBecomingAvailable = (existingRunner.Status == models.RunnerStatusOffline ||
			existingRunner.Status == models.RunnerStatusBusy) &&
			runner.Status == models.RunnerStatusOnline
	}

	updatedRunner, err := s.repo.CreateOrUpdate(ctx, runner)
	if err != nil {
		return nil, err
	}

	if isNewOrBecomingAvailable {
		s.triggerTaskMonitor()
	}

	return updatedRunner, nil
}

func (s *RunnerService) UpdateRunner(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	return s.repo.Update(ctx, runner)
}

func (s *RunnerService) ListRunnersByStatus(ctx context.Context, status models.RunnerStatus) ([]*models.Runner, error) {
	return s.repo.ListByStatus(ctx, status)
}

func (s *RunnerService) UpdateRunnerStatus(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	existingRunner, err := s.repo.Get(ctx, runner.DeviceID)
	if err != nil {
		return nil, err
	}

	becomingAvailable := (existingRunner.Status == models.RunnerStatusOffline ||
		existingRunner.Status == models.RunnerStatusBusy) &&
		runner.Status == models.RunnerStatusOnline

	existingRunner.Status = runner.Status

	if runner.WalletAddress != "" {
		existingRunner.WalletAddress = runner.WalletAddress
	}

	if runner.Status == models.RunnerStatusOnline {
		log := gologger.WithComponent("runner_service")
		log.Debug().
			Str("device_id", runner.DeviceID).
			Msg("Updating runner heartbeat")
	}

	updatedRunner, err := s.repo.Update(ctx, existingRunner)
	if err != nil {
		return nil, err
	}

	if becomingAvailable {
		s.triggerTaskMonitor()
	}

	return updatedRunner, nil
}

func (s *RunnerService) UpdateOfflineRunners(ctx context.Context) error {
	log := gologger.WithComponent("runner_service")

	count, deviceIDs, err := s.repo.UpdateRunnersToOffline(ctx, s.heartbeatTimeout)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update offline runners")
		return err
	}

	if count > 0 {
		var deviceIDsStr string
		maxDisplay := 3

		if len(deviceIDs) <= maxDisplay {
			deviceIDsStr = strings.Join(deviceIDs, ", ")
		} else {
			deviceIDsStr = fmt.Sprintf("%s and %d more",
				strings.Join(deviceIDs[:maxDisplay], ", "),
				len(deviceIDs)-maxDisplay)
		}

		log.Info().
			Int64("count", count).
			Dur("timeout", s.heartbeatTimeout).
			Str("runner_ids", deviceIDsStr).
			Msg("Marked runners as offline due to heartbeat timeout")

		s.triggerTaskMonitor()
	} else {
		log.Debug().
			Dur("timeout", s.heartbeatTimeout).
			Msg("Heartbeat check completed - all runners active")
	}

	return nil
}

func (s *RunnerService) StopTaskMonitor() error {
	if s.taskMonitorCh != nil {
		close(s.taskMonitorCh)
	}
	return nil
} 