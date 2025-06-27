package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/services"
)

type ReputationHandler struct {
	reputationService       *services.ReputationService
	runnerMonitoringService *services.RunnerMonitoringService
}

func NewReputationHandler(
	reputationService *services.ReputationService,
	runnerMonitoringService *services.RunnerMonitoringService,
) *ReputationHandler {
	return &ReputationHandler{
		reputationService:       reputationService,
		runnerMonitoringService: runnerMonitoringService,
	}
}

// GetRunnerReputation returns reputation information for a specific runner
func (h *ReputationHandler) GetRunnerReputation(c *gin.Context) {
	log := gologger.WithComponent("reputation_handler")

	runnerID := c.Param("runner_id")
	if runnerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runner_id is required"})
		return
	}

	reputation, err := h.reputationService.GetRunnerReputation(c.Request.Context(), runnerID)
	if err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to get runner reputation")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve reputation"})
		return
	}

	c.JSON(http.StatusOK, reputation)
}

// GetLeaderboard returns the reputation leaderboard
func (h *ReputationHandler) GetLeaderboard(c *gin.Context) {
	log := gologger.WithComponent("reputation_handler")

	leaderboardType := c.DefaultQuery("type", "overall")
	limitStr := c.DefaultQuery("limit", "10")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 10
	}

	leaderboard, err := h.reputationService.GetLeaderboard(c.Request.Context(), leaderboardType, limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get reputation leaderboard")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve leaderboard"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"type":        leaderboardType,
		"limit":       limit,
		"leaderboard": leaderboard,
	})
}

// GetNetworkStats returns network-wide reputation statistics
func (h *ReputationHandler) GetNetworkStats(c *gin.Context) {
	log := gologger.WithComponent("reputation_handler")

	stats, err := h.reputationService.GetNetworkStats(c.Request.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get network reputation stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve network stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetRunnerEvents returns reputation events for a specific runner
func (h *ReputationHandler) GetRunnerEvents(c *gin.Context) {
	runnerID := c.Param("runner_id")
	if runnerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runner_id is required"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	}

	// For now, return a placeholder since GetRunnerEvents is not implemented in ReputationService
	c.JSON(http.StatusOK, gin.H{
		"runner_id": runnerID,
		"events":    []interface{}{},
		"limit":     limit,
		"message":   "Event history feature coming soon",
	})
}

// GetMonitoringAssignments returns active monitoring assignments
func (h *ReputationHandler) GetMonitoringAssignments(c *gin.Context) {
	if h.runnerMonitoringService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Monitoring service not available"})
		return
	}

	assignments := h.runnerMonitoringService.GetActiveAssignments()

	c.JSON(http.StatusOK, gin.H{
		"assignments": assignments,
		"count":       len(assignments),
	})
}

// GetMonitoringStats returns monitoring system statistics
func (h *ReputationHandler) GetMonitoringStats(c *gin.Context) {
	if h.runnerMonitoringService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Monitoring service not available"})
		return
	}

	stats := h.runnerMonitoringService.GetMonitoringStats()

	c.JSON(http.StatusOK, stats)
}

// GetRunnerMetrics returns monitoring metrics for a specific runner
func (h *ReputationHandler) GetRunnerMetrics(c *gin.Context) {
	runnerID := c.Param("runner_id")
	if runnerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runner_id is required"})
		return
	}

	if h.runnerMonitoringService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Monitoring service not available"})
		return
	}

	metrics := h.runnerMonitoringService.GetRunnerMetrics(runnerID)
	if metrics == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No metrics found for runner"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"runner_id": runnerID,
		"metrics":   metrics,
	})
}

// ReportMaliciousBehavior allows reporting malicious behavior
func (h *ReputationHandler) ReportMaliciousBehavior(c *gin.Context) {
	log := gologger.WithComponent("reputation_handler")

	var request struct {
		RunnerID string                 `json:"runner_id" binding:"required"`
		Reason   string                 `json:"reason" binding:"required"`
		Evidence map[string]interface{} `json:"evidence"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.reputationService.ReportMaliciousBehavior(
		c.Request.Context(),
		request.RunnerID,
		request.Reason,
		request.Evidence,
	)
	if err != nil {
		log.Error().Err(err).
			Str("runner_id", request.RunnerID).
			Str("reason", request.Reason).
			Msg("Failed to report malicious behavior")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to report malicious behavior"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Malicious behavior report submitted successfully",
		"runner_id": request.RunnerID,
	})
}
