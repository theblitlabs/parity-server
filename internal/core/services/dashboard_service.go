package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/theblitlabs/parity-server/internal/core/models"
)

type dashboardTaskService interface {
	ListTasks(ctx context.Context, limit, offset int) ([]*models.Task, error)
	GetTaskCounts(ctx context.Context) (TaskStatusCounts, error)
	GetTaskResults(ctx context.Context, ids []string) (map[string]*models.TaskResult, error)
}

type dashboardRunnerService interface {
	ListRecentRunners(ctx context.Context, limit int) ([]*models.Runner, error)
	GetRunnerCounts(ctx context.Context) (DashboardRunnerCounts, error)
}

type dashboardReputationService interface {
	GetNetworkStats(ctx context.Context) (*models.NetworkStats, error)
}

type dashboardFederatedLearningService interface {
	ListRecentSessions(ctx context.Context, limit int) ([]*models.FederatedLearningSession, error)
	GetSessionCounts(ctx context.Context) (DashboardFLCounts, error)
}

type dashboardLLMService interface {
	GetAvailableModels(ctx context.Context) ([]models.ModelCapability, error)
}

type DashboardService struct {
	taskService       dashboardTaskService
	runnerService     dashboardRunnerService
	reputationService dashboardReputationService
	flService         dashboardFederatedLearningService
	llmService        dashboardLLMService
}

type DashboardOverview struct {
	GeneratedAt       time.Time             `json:"generated_at"`
	Summary           DashboardSummary      `json:"summary"`
	RecentTasks       []DashboardTaskItem   `json:"recent_tasks"`
	Runners           []DashboardRunnerItem `json:"runners"`
	Models            []DashboardModelItem  `json:"models"`
	FederatedLearning DashboardFLSummary    `json:"federated_learning"`
	Network           *models.NetworkStats  `json:"network,omitempty"`
	Warnings          []string              `json:"warnings,omitempty"`
	QuickLinks        map[string]string     `json:"quick_links"`
}

type DashboardSummary struct {
	Tasks          TaskStatusCounts      `json:"tasks"`
	Runners        DashboardRunnerCounts `json:"runners"`
	AvailableModel int                   `json:"available_models"`
	ActiveSessions int                   `json:"active_sessions"`
}

type DashboardRunnerCounts struct {
	Total   int `json:"total"`
	Online  int `json:"online"`
	Busy    int `json:"busy"`
	Offline int `json:"offline"`
}

type DashboardTaskItem struct {
	ID                   string                      `json:"id"`
	Title                string                      `json:"title"`
	Description          string                      `json:"description"`
	Type                 models.TaskType             `json:"type"`
	Status               models.TaskStatus           `json:"status"`
	RunnerID             string                      `json:"runner_id,omitempty"`
	CreatorAddress       string                      `json:"creator_address,omitempty"`
	CreatorDeviceID      string                      `json:"creator_device_id,omitempty"`
	Reward               float64                     `json:"reward"`
	CreatedAt            time.Time                   `json:"created_at"`
	UpdatedAt            time.Time                   `json:"updated_at"`
	CompletedAt          *time.Time                  `json:"completed_at,omitempty"`
	AgeMS                int64                       `json:"age_ms"`
	EstimatedQueueWaitMS *int64                      `json:"estimated_queue_wait_ms,omitempty"`
	Result               *DashboardTaskResultSummary `json:"result,omitempty"`
}

type DashboardTaskResultSummary struct {
	ExitCode            int     `json:"exit_code"`
	ExecutionTime       int64   `json:"execution_time"`
	VerificationStatus  string  `json:"verification_status"`
	ResultHash          string  `json:"result_hash,omitempty"`
	ImageHashVerified   string  `json:"image_hash_verified,omitempty"`
	CommandHashVerified string  `json:"command_hash_verified,omitempty"`
	OutputPreview       string  `json:"output_preview,omitempty"`
	ErrorPreview        string  `json:"error_preview,omitempty"`
	Reward              float64 `json:"reward"`
}

type DashboardRunnerItem struct {
	DeviceID             string                     `json:"device_id"`
	WalletAddress        string                     `json:"wallet_address,omitempty"`
	Status               models.RunnerStatus        `json:"status"`
	Webhook              string                     `json:"webhook,omitempty"`
	LastHeartbeat        time.Time                  `json:"last_heartbeat"`
	TimeSinceHeartbeatMS int64                      `json:"time_since_heartbeat_ms"`
	CurrentTask          *DashboardRunnerTaskBrief  `json:"current_task,omitempty"`
	ModelCapabilities    []DashboardModelCapability `json:"model_capabilities,omitempty"`
}

type DashboardRunnerTaskBrief struct {
	ID     string            `json:"id"`
	Title  string            `json:"title"`
	Status models.TaskStatus `json:"status"`
}

type DashboardModelCapability struct {
	ModelName string `json:"model_name"`
	MaxTokens int    `json:"max_tokens"`
}

type DashboardModelItem struct {
	ModelName   string   `json:"model_name"`
	MaxTokens   int      `json:"max_tokens"`
	RunnerCount int      `json:"runner_count"`
	LoadedBy    []string `json:"loaded_by"`
}

type DashboardFLSummary struct {
	Total     int                  `json:"total"`
	Pending   int                  `json:"pending"`
	Active    int                  `json:"active"`
	Completed int                  `json:"completed"`
	Failed    int                  `json:"failed"`
	Sessions  []DashboardFLSession `json:"sessions"`
}

type DashboardFLCounts struct {
	Total     int
	Pending   int
	Active    int
	Completed int
	Failed    int
}

type DashboardFLSession struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	ModelType       string                 `json:"model_type"`
	Status          models.FLSessionStatus `json:"status"`
	CurrentRound    int                    `json:"current_round"`
	TotalRounds     int                    `json:"total_rounds"`
	MinParticipants int                    `json:"min_participants"`
	CreatorAddress  string                 `json:"creator_address"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
}

func NewDashboardService(
	taskService dashboardTaskService,
	runnerService dashboardRunnerService,
	reputationService dashboardReputationService,
	flService dashboardFederatedLearningService,
	llmService dashboardLLMService,
) *DashboardService {
	return &DashboardService{
		taskService:       taskService,
		runnerService:     runnerService,
		reputationService: reputationService,
		flService:         flService,
		llmService:        llmService,
	}
}

func (s *DashboardService) BuildOverview(ctx context.Context, recentTaskLimit int) (*DashboardOverview, error) {
	if recentTaskLimit <= 0 {
		recentTaskLimit = 20
	}
	if recentTaskLimit > 100 {
		recentTaskLimit = 100
	}

	overview := &DashboardOverview{
		GeneratedAt: time.Now().UTC(),
		QuickLinks: map[string]string{
			"tasks":       "/api/v1/tasks",
			"network":     "/api/v1/reputation/network/stats",
			"health":      "/health",
			"models":      "/api/v1/llm/models",
			"fl_sessions": "/api/v1/federated-learning/sessions",
		},
	}

	taskCounts, err := s.taskService.GetTaskCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("load task counts: %w", err)
	}
	overview.Summary.Tasks = taskCounts

	recentTasks, err := s.taskService.ListTasks(ctx, recentTaskLimit, 0)
	if err != nil {
		return nil, fmt.Errorf("load recent tasks: %w", err)
	}
	taskResults, resultErr := s.loadTaskResults(ctx, recentTasks)
	if resultErr != nil {
		overview.Warnings = append(overview.Warnings, fmt.Sprintf("task results unavailable: %v", resultErr))
	}
	overview.RecentTasks = s.buildRecentTasks(recentTasks, taskResults)

	if s.runnerService != nil {
		runnerCounts, countErr := s.runnerService.GetRunnerCounts(ctx)
		if countErr != nil {
			overview.Warnings = append(overview.Warnings, fmt.Sprintf("runner counts unavailable: %v", countErr))
		} else {
			overview.Summary.Runners = runnerCounts
		}

		runners, runnerErr := s.runnerService.ListRecentRunners(ctx, recentTaskLimit)
		if runnerErr != nil {
			overview.Warnings = append(overview.Warnings, fmt.Sprintf("runner inventory unavailable: %v", runnerErr))
		} else {
			overview.Runners = buildRunnerItems(runners)
			if countErr != nil {
				overview.Summary.Runners = summarizeRunnerItems(runners)
			}
		}
	}

	if s.llmService != nil {
		models, modelErr := s.llmService.GetAvailableModels(ctx)
		if modelErr != nil {
			overview.Warnings = append(overview.Warnings, fmt.Sprintf("model inventory unavailable: %v", modelErr))
		} else {
			overview.Models = buildModelItems(models)
			overview.Summary.AvailableModel = len(overview.Models)
		}
	}

	if s.flService != nil {
		flCounts, countErr := s.flService.GetSessionCounts(ctx)
		if countErr != nil {
			overview.Warnings = append(overview.Warnings, fmt.Sprintf("federated learning counts unavailable: %v", countErr))
		}

		sessions, sessionErr := s.flService.ListRecentSessions(ctx, 8)
		if sessionErr != nil {
			overview.Warnings = append(overview.Warnings, fmt.Sprintf("federated learning overview unavailable: %v", sessionErr))
		}

		overview.FederatedLearning = buildFLSummary(sessions, flCounts)
		overview.Summary.ActiveSessions = overview.FederatedLearning.Active
	}

	if s.reputationService != nil {
		network, networkErr := s.reputationService.GetNetworkStats(ctx)
		if networkErr != nil {
			overview.Warnings = append(overview.Warnings, fmt.Sprintf("network stats unavailable: %v", networkErr))
		} else {
			overview.Network = network
		}
	}

	return overview, nil
}

func (s *DashboardService) loadTaskResults(ctx context.Context, tasks []*models.Task) (map[string]*models.TaskResult, error) {
	if len(tasks) == 0 {
		return map[string]*models.TaskResult{}, nil
	}

	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		taskIDs = append(taskIDs, task.ID.String())
	}

	return s.taskService.GetTaskResults(ctx, taskIDs)
}

func (s *DashboardService) buildRecentTasks(tasks []*models.Task, taskResults map[string]*models.TaskResult) []DashboardTaskItem {
	items := make([]DashboardTaskItem, 0, len(tasks))
	now := time.Now()

	for _, task := range tasks {
		createdAt := normalizeDashboardTimestamp(now, task.CreatedAt)
		updatedAt := normalizeDashboardTimestamp(now, task.UpdatedAt)
		var completedAt *time.Time
		if task.CompletedAt != nil {
			normalizedCompletedAt := normalizeDashboardTimestamp(now, *task.CompletedAt)
			completedAt = &normalizedCompletedAt
		}

		item := DashboardTaskItem{
			ID:              task.ID.String(),
			Title:           task.Title,
			Description:     task.Description,
			Type:            task.Type,
			Status:          task.Status,
			RunnerID:        task.RunnerID,
			CreatorAddress:  task.CreatorAddress,
			CreatorDeviceID: task.CreatorDeviceID,
			Reward:          task.Reward,
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
			CompletedAt:     completedAt,
			AgeMS:           elapsedMilliseconds(now, createdAt),
		}

		if result := taskResults[task.ID.String()]; result != nil {
			item.Result = &DashboardTaskResultSummary{
				ExitCode:            result.ExitCode,
				ExecutionTime:       result.ExecutionTime,
				VerificationStatus:  result.VerificationStatus,
				ResultHash:          result.ResultHash,
				ImageHashVerified:   result.ImageHashVerified,
				CommandHashVerified: result.CommandHashVerified,
				OutputPreview:       previewText(result.Output, 160),
				ErrorPreview:        previewText(result.Error, 120),
				Reward:              result.Reward,
			}

			if completedAt != nil && result.ExecutionTime > 0 {
				wait := completedAt.Sub(createdAt).Milliseconds() - result.ExecutionTime
				if wait < 0 {
					wait = 0
				}
				item.EstimatedQueueWaitMS = &wait
			}
		}

		items = append(items, item)
	}

	return items
}

func buildRunnerItems(runners []*models.Runner) []DashboardRunnerItem {
	items := make([]DashboardRunnerItem, 0, len(runners))
	now := time.Now()

	for _, runner := range runners {
		displayStatus := runner.Status
		if displayStatus == models.RunnerStatusOnline && (runner.TaskID != nil || runner.Task != nil) {
			displayStatus = models.RunnerStatusBusy
		}

		lastHeartbeat := normalizeDashboardTimestamp(now, runner.LastHeartbeat)

		item := DashboardRunnerItem{
			DeviceID:             runner.DeviceID,
			WalletAddress:        runner.WalletAddress,
			Status:               displayStatus,
			Webhook:              runner.Webhook,
			LastHeartbeat:        lastHeartbeat,
			TimeSinceHeartbeatMS: elapsedMilliseconds(now, lastHeartbeat),
		}

		if runner.Task != nil {
			item.CurrentTask = &DashboardRunnerTaskBrief{
				ID:     runner.Task.ID.String(),
				Title:  runner.Task.Title,
				Status: runner.Task.Status,
			}
		}

		item.ModelCapabilities = make([]DashboardModelCapability, 0, len(runner.ModelCapabilities))
		for _, capability := range runner.ModelCapabilities {
			if !capability.IsLoaded {
				continue
			}
			item.ModelCapabilities = append(item.ModelCapabilities, DashboardModelCapability{
				ModelName: capability.ModelName,
				MaxTokens: capability.MaxTokens,
			})
		}

		sort.Slice(item.ModelCapabilities, func(i, j int) bool {
			return item.ModelCapabilities[i].ModelName < item.ModelCapabilities[j].ModelName
		})

		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		leftRank := runnerStatusRank(items[i].Status)
		rightRank := runnerStatusRank(items[j].Status)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return items[i].LastHeartbeat.After(items[j].LastHeartbeat)
	})

	return items
}

func summarizeRunnerItems(runners []*models.Runner) DashboardRunnerCounts {
	counts := DashboardRunnerCounts{Total: len(runners)}
	for _, runner := range runners {
		displayStatus := runner.Status
		if displayStatus == models.RunnerStatusOnline && (runner.TaskID != nil || runner.Task != nil) {
			displayStatus = models.RunnerStatusBusy
		}

		switch displayStatus {
		case models.RunnerStatusOnline:
			counts.Online++
		case models.RunnerStatusBusy:
			counts.Busy++
		case models.RunnerStatusOffline:
			counts.Offline++
		}
	}

	return counts
}

func buildModelItems(capabilities []models.ModelCapability) []DashboardModelItem {
	grouped := make(map[string]*DashboardModelItem)
	for _, capability := range capabilities {
		if !capability.IsLoaded {
			continue
		}

		item, ok := grouped[capability.ModelName]
		if !ok {
			item = &DashboardModelItem{
				ModelName: capability.ModelName,
				MaxTokens: capability.MaxTokens,
			}
			grouped[capability.ModelName] = item
		}

		item.RunnerCount++
		item.LoadedBy = append(item.LoadedBy, capability.RunnerID)
		if capability.MaxTokens > item.MaxTokens {
			item.MaxTokens = capability.MaxTokens
		}
	}

	items := make([]DashboardModelItem, 0, len(grouped))
	for _, item := range grouped {
		sort.Strings(item.LoadedBy)
		items = append(items, *item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].RunnerCount != items[j].RunnerCount {
			return items[i].RunnerCount > items[j].RunnerCount
		}
		return items[i].ModelName < items[j].ModelName
	})

	return items
}

func buildFLSummary(sessions []*models.FederatedLearningSession, counts DashboardFLCounts) DashboardFLSummary {
	summary := DashboardFLSummary{
		Total:     counts.Total,
		Pending:   counts.Pending,
		Active:    counts.Active,
		Completed: counts.Completed,
		Failed:    counts.Failed,
		Sessions:  []DashboardFLSession{},
	}
	if len(sessions) == 0 {
		return summary
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	summary.Sessions = make([]DashboardFLSession, 0, minInt(len(sessions), 8))
	for index, session := range sessions {
		createdAt := normalizeDashboardTimestamp(time.Now(), session.CreatedAt)
		updatedAt := normalizeDashboardTimestamp(time.Now(), session.UpdatedAt)
		var completedAt *time.Time
		if session.CompletedAt != nil {
			normalizedCompletedAt := normalizeDashboardTimestamp(time.Now(), *session.CompletedAt)
			completedAt = &normalizedCompletedAt
		}

		if index < 8 {
			summary.Sessions = append(summary.Sessions, DashboardFLSession{
				ID:              session.ID.String(),
				Name:            session.Name,
				ModelType:       session.ModelType,
				Status:          session.Status,
				CurrentRound:    session.CurrentRound,
				TotalRounds:     session.TotalRounds,
				MinParticipants: session.MinParticipants,
				CreatorAddress:  session.CreatorAddress,
				CreatedAt:       createdAt,
				UpdatedAt:       updatedAt,
				CompletedAt:     completedAt,
			})
		}
	}

	return summary
}

func previewText(raw string, limit int) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	if len(trimmed) <= limit {
		return trimmed
	}

	return trimmed[:limit-3] + "..."
}

func elapsedMilliseconds(now, then time.Time) int64 {
	if then.IsZero() {
		return 0
	}

	elapsed := now.Sub(then)
	if elapsed < 0 {
		return 0
	}

	return elapsed.Milliseconds()
}

func normalizeDashboardTimestamp(now, observed time.Time) time.Time {
	if observed.IsZero() {
		return observed
	}

	if observed.After(now.Add(time.Minute)) {
		_, offsetSeconds := now.Zone()
		adjusted := observed.Add(-time.Duration(offsetSeconds) * time.Second)
		if !adjusted.After(now.Add(time.Minute)) {
			return adjusted
		}
	}

	return observed
}

func runnerStatusRank(status models.RunnerStatus) int {
	switch status {
	case models.RunnerStatusBusy:
		return 0
	case models.RunnerStatusOnline:
		return 1
	case models.RunnerStatusOffline:
		return 2
	default:
		return 3
	}
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
