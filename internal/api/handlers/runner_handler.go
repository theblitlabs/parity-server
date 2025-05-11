package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/api/models"
	coremodels "github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/services"
)

type RunnerHandler struct {
	taskService   *services.TaskService
	runnerService *services.RunnerService
}

func NewRunnerHandler(taskService *services.TaskService, runnerService *services.RunnerService) *RunnerHandler {
	return &RunnerHandler{
		taskService:   taskService,
		runnerService: runnerService,
	}
}

func (h *RunnerHandler) RegisterRunner(c *gin.Context) {
	var runner coremodels.Runner
	log := gologger.WithComponent("runner_handler")

	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))

	log.Info().Str("raw_body", string(rawBody)).Msg("Incoming request body")

	if err := c.ShouldBindJSON(&runner); err != nil {
		log.Error().Err(err).Msg("Invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	log.Info().Fields(map[string]interface{}{
		"wallet_address": runner.WalletAddress,
		"webhook":        runner.Webhook,
		"status":         runner.Status,
	}).Msg("Parsed request body")

	deviceID := c.GetHeader("X-Device-ID")
	if deviceID == "" {
		log.Error().Msg("X-Device-ID header is missing")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	if runner.WalletAddress == "" {
		log.Error().Msg("Wallet address is missing in request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Wallet address is required"})
		return
	}

	runner.Status = coremodels.RunnerStatusOnline
	runner.DeviceID = deviceID

	createdRunner, err := h.runnerService.CreateOrUpdateRunner(c.Request.Context(), &runner)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create/update runner")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Info().Fields(map[string]interface{}{
		"device_id":      createdRunner.DeviceID,
		"wallet_address": createdRunner.WalletAddress,
		"status":         createdRunner.Status,
		"webhook":        createdRunner.Webhook,
	}).Msg("Runner created/updated successfully")

	c.JSON(http.StatusCreated, createdRunner)
}

func (h *RunnerHandler) RunnerHeartbeat(c *gin.Context) {
	deviceID := c.GetHeader("X-Device-ID")

	var payload models.HeartbeatPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	runner := &coremodels.Runner{
		DeviceID: deviceID,
		Status:   coremodels.RunnerStatusOnline,
		Webhook:  payload.PublicIP,
	}

	if _, err := h.runnerService.UpdateRunnerStatus(c.Request.Context(), runner); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (h *RunnerHandler) ListAvailableTasks(c *gin.Context) {
	tasks, err := h.taskService.ListAvailableTasks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

func (h *RunnerHandler) StartTask(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task ID is required"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	if err := h.taskService.AssignTaskToRunner(c.Request.Context(), taskID, deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.taskService.StartTask(c.Request.Context(), taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	task, err := h.taskService.GetTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *RunnerHandler) CompleteTask(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task ID is required"})
		return
	}

	if err := h.taskService.CompleteTask(c.Request.Context(), taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	task, err := h.taskService.GetTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *RunnerHandler) NotifyRunnerOfTasks(runnerID string, tasks []*coremodels.Task) error {
	log := gologger.WithComponent("runner_handler")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runner, err := h.runnerService.GetRunner(ctx, runnerID)
	if err != nil {
		return fmt.Errorf("failed to get runner: %w", err)
	}

	if runner.Webhook == "" {
		return fmt.Errorf("runner has no webhook URL")
	}

	if len(tasks) == 0 {
		return nil
	}

	tasksJSON, err := json.Marshal(tasks)
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	message := models.WSMessage{
		Type:    "available_tasks",
		Payload: tasksJSON,
	}

	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", runner.Webhook, bytes.NewBuffer(messageJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Warn().Err(err).
			Str("runner_id", runnerID).
			Msg("Failed to notify runner, will be handled on next heartbeat")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Warn().
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Str("runner_id", runnerID).
			Msg("Webhook notification failed, will be handled on next heartbeat")
		return nil
	}

	log.Info().
		Str("runner_id", runnerID).
		Int("num_tasks", len(tasks)).
		Msg("Successfully notified runner of tasks")
	return nil
}
