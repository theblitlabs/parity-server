package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-server/internal/core/models"
)

type dashboardTaskStub struct {
	tasks               []*models.Task
	results             map[string]*models.TaskResult
	counts              TaskStatusCounts
	getTaskResultsCalls int
}

func (s *dashboardTaskStub) ListTasks(ctx context.Context, limit, offset int) ([]*models.Task, error) {
	return s.tasks, nil
}

func (s *dashboardTaskStub) GetTaskCounts(ctx context.Context) (TaskStatusCounts, error) {
	return s.counts, nil
}

func (s *dashboardTaskStub) GetTaskResults(ctx context.Context, ids []string) (map[string]*models.TaskResult, error) {
	s.getTaskResultsCalls++
	results := make(map[string]*models.TaskResult, len(ids))
	for _, id := range ids {
		if result, ok := s.results[id]; ok {
			results[id] = result
		}
	}
	return results, nil
}

type dashboardRunnerStub struct {
	runners               []*models.Runner
	counts                DashboardRunnerCounts
	listRecentCalls       int
	getRunnerCountsCalls  int
	lastRecentRunnerLimit int
}

func (s *dashboardRunnerStub) ListRecentRunners(ctx context.Context, limit int) ([]*models.Runner, error) {
	s.listRecentCalls++
	s.lastRecentRunnerLimit = limit
	return s.runners, nil
}

func (s *dashboardRunnerStub) GetRunnerCounts(ctx context.Context) (DashboardRunnerCounts, error) {
	s.getRunnerCountsCalls++
	return s.counts, nil
}

type dashboardReputationStub struct {
	stats *models.NetworkStats
}

func (s *dashboardReputationStub) GetNetworkStats(ctx context.Context) (*models.NetworkStats, error) {
	return s.stats, nil
}

type dashboardFLStub struct {
	sessions []*models.FederatedLearningSession
	counts   DashboardFLCounts
}

func (s *dashboardFLStub) ListRecentSessions(ctx context.Context, limit int) ([]*models.FederatedLearningSession, error) {
	return s.sessions, nil
}

func (s *dashboardFLStub) GetSessionCounts(ctx context.Context) (DashboardFLCounts, error) {
	return s.counts, nil
}

type dashboardLLMStub struct {
	models []models.ModelCapability
}

func (s *dashboardLLMStub) GetAvailableModels(ctx context.Context) ([]models.ModelCapability, error) {
	return s.models, nil
}

func TestDashboardServiceBuildOverview(t *testing.T) {
	now := time.Now().UTC()
	taskID := uuid.New()
	runnerTaskID := taskID
	completedAt := now

	taskStub := &dashboardTaskStub{
		tasks: []*models.Task{
			{
				ID:              taskID,
				Title:           "heavy compute",
				Description:     "sum a large sequence",
				Type:            models.TaskTypeDocker,
				Status:          models.TaskStatusCompleted,
				RunnerID:        "runner-1",
				CreatorAddress:  "0xabc",
				CreatorDeviceID: "creator-1",
				Reward:          1.5,
				CreatedAt:       now.Add(-5 * time.Second),
				UpdatedAt:       now.Add(-1 * time.Second),
				CompletedAt:     &completedAt,
			},
		},
		results: map[string]*models.TaskResult{
			taskID.String(): {
				TaskID:             taskID,
				ExitCode:           0,
				ExecutionTime:      2200,
				VerificationStatus: "verified",
				Output:             "719999710",
				Reward:             1.5,
			},
		},
		counts: TaskStatusCounts{
			Total:     1,
			Completed: 1,
		},
	}

	runnerStub := &dashboardRunnerStub{
		runners: []*models.Runner{
			{
				DeviceID:      "runner-1",
				WalletAddress: "0xrunner",
				Status:        models.RunnerStatusBusy,
				LastHeartbeat: now.Add(-2 * time.Second),
				TaskID:        &runnerTaskID,
				Task: &models.Task{
					ID:     taskID,
					Title:  "heavy compute",
					Status: models.TaskStatusCompleted,
				},
				ModelCapabilities: []models.ModelCapability{
					{RunnerID: "runner-1", ModelName: "qwen3:8b", IsLoaded: true, MaxTokens: 8192},
				},
			},
		},
		counts: DashboardRunnerCounts{
			Total:   7,
			Online:  5,
			Busy:    1,
			Offline: 1,
		},
	}

	service := NewDashboardService(
		taskStub,
		runnerStub,
		&dashboardReputationStub{
			stats: &models.NetworkStats{
				TotalRunners:  1,
				ActiveRunners: 1,
				NetworkHealth: "healthy",
			},
		},
		&dashboardFLStub{
			sessions: []*models.FederatedLearningSession{
				{
					ID:              uuid.New(),
					Name:            "image-classifier",
					ModelType:       "cnn",
					Status:          models.FLSessionStatusActive,
					CurrentRound:    2,
					TotalRounds:     5,
					MinParticipants: 1,
					CreatorAddress:  "0xabc",
					CreatedAt:       now.Add(-30 * time.Minute),
					UpdatedAt:       now.Add(-10 * time.Second),
				},
			},
			counts: DashboardFLCounts{
				Total:  1,
				Active: 1,
			},
		},
		&dashboardLLMStub{
			models: []models.ModelCapability{
				{RunnerID: "runner-1", ModelName: "qwen3:8b", IsLoaded: true, MaxTokens: 8192},
			},
		},
	)

	overview, err := service.BuildOverview(context.Background(), 10)
	if err != nil {
		t.Fatalf("BuildOverview() error = %v", err)
	}

	if overview.Summary.Tasks.Completed != 1 {
		t.Fatalf("completed tasks = %d, want 1", overview.Summary.Tasks.Completed)
	}

	if len(overview.RecentTasks) != 1 {
		t.Fatalf("recent tasks length = %d, want 1", len(overview.RecentTasks))
	}

	if overview.RecentTasks[0].Result == nil || overview.RecentTasks[0].Result.ExecutionTime != 2200 {
		t.Fatalf("expected execution time to be surfaced in dashboard task result: %#v", overview.RecentTasks[0].Result)
	}

	if taskStub.getTaskResultsCalls != 1 {
		t.Fatalf("GetTaskResults() calls = %d, want 1", taskStub.getTaskResultsCalls)
	}

	if runnerStub.listRecentCalls != 1 {
		t.Fatalf("ListRecentRunners() calls = %d, want 1", runnerStub.listRecentCalls)
	}

	if runnerStub.getRunnerCountsCalls != 1 {
		t.Fatalf("GetRunnerCounts() calls = %d, want 1", runnerStub.getRunnerCountsCalls)
	}

	if runnerStub.lastRecentRunnerLimit != 10 {
		t.Fatalf("ListRecentRunners() limit = %d, want 10", runnerStub.lastRecentRunnerLimit)
	}

	if overview.Summary.Runners.Total != 7 {
		t.Fatalf("runner total = %d, want 7", overview.Summary.Runners.Total)
	}

	if overview.Summary.Runners.Busy != 1 {
		t.Fatalf("busy runners = %d, want 1", overview.Summary.Runners.Busy)
	}

	if len(overview.Runners) != 1 {
		t.Fatalf("runner preview length = %d, want 1", len(overview.Runners))
	}

	if len(overview.Models) != 1 || overview.Models[0].RunnerCount != 1 {
		t.Fatalf("model inventory = %#v, want one loaded model", overview.Models)
	}

	if overview.FederatedLearning.Active != 1 {
		t.Fatalf("active sessions = %d, want 1", overview.FederatedLearning.Active)
	}

	if overview.Network == nil || overview.Network.NetworkHealth != "healthy" {
		t.Fatalf("network stats = %#v, want healthy payload", overview.Network)
	}
}
