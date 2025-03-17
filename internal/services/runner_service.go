package services

import (
	"context"
	"errors"

	"github.com/theblitlabs/parity-server/internal/models"
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
}

type RunnerService struct {
	repo        RunnerRepository
	taskService *TaskService
}

func NewRunnerService(repo RunnerRepository) *RunnerService {
	return &RunnerService{
		repo: repo,
	}
}

func (s *RunnerService) SetTaskService(taskService *TaskService) {
	s.taskService = taskService
}

func (s *RunnerService) CreateRunner(ctx context.Context, runner *models.Runner) error {
	return s.repo.Create(ctx, runner)
}

func (s *RunnerService) GetRunner(ctx context.Context, deviceID string) (*models.Runner, error) {
	return s.repo.Get(ctx, deviceID)
}

func (s *RunnerService) CreateOrUpdateRunner(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	// Check if runner exists
	existingRunner, err := s.repo.Get(ctx, runner.DeviceID)

	// Determine if this is a new runner or one becoming available
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

	// If runner becomes available, let TaskMonitor handle task assignment
	if isNewOrBecomingAvailable && s.taskService != nil {
		go s.taskService.MonitorTasks()
	}

	return updatedRunner, nil
}

func (s *RunnerService) UpdateRunner(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	return s.repo.Update(ctx, runner)
}

func (s *RunnerService) ListRunnersByStatus(ctx context.Context, status models.RunnerStatus) ([]*models.Runner, error) {
	return s.repo.ListByStatus(ctx, status)
}

// UpdateRunnerStatus updates a runner's status via heartbeat
func (s *RunnerService) UpdateRunnerStatus(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	// Get the existing runner
	existingRunner, err := s.repo.Get(ctx, runner.DeviceID)
	if err != nil {
		return nil, err
	}

	// Check if runner is becoming available
	becomingAvailable := (existingRunner.Status == models.RunnerStatusOffline ||
		existingRunner.Status == models.RunnerStatusBusy) &&
		runner.Status == models.RunnerStatusOnline

	// Update only the status, preserving other fields
	existingRunner.Status = runner.Status

	// If wallet address is provided, update it
	if runner.WalletAddress != "" {
		existingRunner.WalletAddress = runner.WalletAddress
	}

	// Update the runner in the database
	updatedRunner, err := s.repo.Update(ctx, existingRunner)
	if err != nil {
		return nil, err
	}

	// If runner becomes available, let TaskMonitor handle task assignment
	if becomingAvailable && s.taskService != nil {
		go s.taskService.MonitorTasks()
	}

	return updatedRunner, nil
}
