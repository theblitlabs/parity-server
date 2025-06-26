package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
	requestmodels "github.com/theblitlabs/parity-server/internal/api/models"
	"github.com/theblitlabs/parity-server/internal/core/services"
)

type LLMHandler struct {
	llmService *services.LLMService
}

func NewLLMHandler(llmService *services.LLMService) *LLMHandler {
	return &LLMHandler{
		llmService: llmService,
	}
}

func (h *LLMHandler) SubmitPrompt(c *gin.Context) {
	log := gologger.WithComponent("llm_handler")

	var req requestmodels.PromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error().Err(err).Msg("Invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	clientID := c.GetHeader("X-Client-ID")
	if clientID == "" {
		log.Error().Msg("Client ID is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Client ID is required"})
		return
	}

	promptReq, err := h.llmService.SubmitPrompt(c.Request.Context(), clientID, req.Prompt, req.ModelName)
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Str("model_name", req.ModelName).Msg("Failed to submit prompt")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := &requestmodels.PromptResponse{
		ID:        promptReq.ID.String(),
		Status:    string(promptReq.Status),
		ModelName: promptReq.ModelName,
		CreatedAt: promptReq.CreatedAt.Format(time.RFC3339),
	}

	log.Info().
		Str("prompt_id", promptReq.ID.String()).
		Str("client_id", clientID).
		Str("model_name", req.ModelName).
		Msg("Prompt submitted successfully")

	c.JSON(http.StatusAccepted, response)
}

func (h *LLMHandler) GetPrompt(c *gin.Context) {
	log := gologger.WithComponent("llm_handler")

	promptID := c.Param("id")
	if promptID == "" {
		log.Error().Msg("Prompt ID is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prompt ID is required"})
		return
	}

	id, err := uuid.Parse(promptID)
	if err != nil {
		log.Error().Err(err).Str("prompt_id", promptID).Msg("Invalid prompt ID format")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid prompt ID format"})
		return
	}

	promptReq, err := h.llmService.GetPrompt(c.Request.Context(), id)
	if err != nil {
		log.Error().Err(err).Str("prompt_id", promptID).Msg("Failed to get prompt")
		c.JSON(http.StatusNotFound, gin.H{"error": "Prompt not found"})
		return
	}

	response := &requestmodels.PromptResponse{
		ID:        promptReq.ID.String(),
		Response:  promptReq.Response,
		Status:    string(promptReq.Status),
		ModelName: promptReq.ModelName,
		CreatedAt: promptReq.CreatedAt.Format(time.RFC3339),
	}

	if promptReq.CompletedAt != nil {
		completedAt := promptReq.CompletedAt.Format(time.RFC3339)
		response.CompletedAt = &completedAt
	}

	c.JSON(http.StatusOK, response)
}

func (h *LLMHandler) ListPrompts(c *gin.Context) {
	log := gologger.WithComponent("llm_handler")

	clientID := c.GetHeader("X-Client-ID")
	if clientID == "" {
		log.Error().Msg("Client ID is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Client ID is required"})
		return
	}

	limit := 10
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	prompts, err := h.llmService.ListPrompts(c.Request.Context(), clientID, limit, offset)
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("Failed to list prompts")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responses := make([]*requestmodels.PromptResponse, len(prompts))
	for i, prompt := range prompts {
		responses[i] = &requestmodels.PromptResponse{
			ID:        prompt.ID.String(),
			Response:  prompt.Response,
			Status:    string(prompt.Status),
			ModelName: prompt.ModelName,
			CreatedAt: prompt.CreatedAt.Format(time.RFC3339),
		}
		if prompt.CompletedAt != nil {
			completedAt := prompt.CompletedAt.Format(time.RFC3339)
			responses[i].CompletedAt = &completedAt
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"prompts": responses,
		"limit":   limit,
		"offset":  offset,
	})
}

func (h *LLMHandler) CompletePrompt(c *gin.Context) {
	log := gologger.WithComponent("llm_handler")

	promptID := c.Param("id")
	if promptID == "" {
		log.Error().Msg("Prompt ID is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prompt ID is required"})
		return
	}

	id, err := uuid.Parse(promptID)
	if err != nil {
		log.Error().Err(err).Str("prompt_id", promptID).Msg("Invalid prompt ID format")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid prompt ID format"})
		return
	}

	var req struct {
		Response       string `json:"response" binding:"required"`
		PromptTokens   int    `json:"prompt_tokens" binding:"required"`
		ResponseTokens int    `json:"response_tokens" binding:"required"`
		InferenceTime  int64  `json:"inference_time_ms" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error().Err(err).Msg("Invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	err = h.llmService.CompletePrompt(c.Request.Context(), id, req.Response, req.PromptTokens, req.ResponseTokens, req.InferenceTime)
	if err != nil {
		log.Error().Err(err).Str("prompt_id", promptID).Msg("Failed to complete prompt")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Info().
		Str("prompt_id", promptID).
		Int("total_tokens", req.PromptTokens+req.ResponseTokens).
		Int64("inference_time_ms", req.InferenceTime).
		Msg("Prompt completed successfully")

	c.JSON(http.StatusOK, gin.H{"message": "Prompt completed successfully"})
}

func (h *LLMHandler) GetBillingMetrics(c *gin.Context) {
	log := gologger.WithComponent("llm_handler")

	clientID := c.GetHeader("X-Client-ID")
	if clientID == "" {
		log.Error().Msg("Client ID is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Client ID is required"})
		return
	}

	metrics, err := h.llmService.GetBillingMetrics(c.Request.Context(), clientID)
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("Failed to get billing metrics")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := &requestmodels.BillingMetricsResponse{
		TotalTokens:      metrics.TotalTokens,
		AvgInferenceTime: float64(metrics.InferenceTime),
	}

	c.JSON(http.StatusOK, response)
}
