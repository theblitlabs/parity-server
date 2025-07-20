package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
	requestmodels "github.com/theblitlabs/parity-server/internal/api/models"
	"github.com/theblitlabs/parity-server/internal/core/ports"
)

type FederatedLearningHandler struct {
	service ports.FederatedLearningService
}

func NewFederatedLearningHandler(service ports.FederatedLearningService) *FederatedLearningHandler {
	return &FederatedLearningHandler{
		service: service,
	}
}

func (h *FederatedLearningHandler) CreateSession(c *gin.Context) {
	log := gologger.WithComponent("fl_handler")

	var req requestmodels.CreateFLSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error().Err(err).Msg("Failed to bind create FL session request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	session, err := h.service.CreateSession(c.Request.Context(), &req)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create FL session")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	response := requestmodels.FLSessionResponse{
		ID:              session.ID.String(),
		Name:            session.Name,
		Description:     session.Description,
		ModelType:       session.ModelType,
		Status:          string(session.Status),
		CurrentRound:    session.CurrentRound,
		TotalRounds:     session.TotalRounds,
		MinParticipants: session.MinParticipants,
		CreatorAddress:  session.CreatorAddress,
		Config: requestmodels.FLConfigRequest{
			AggregationMethod: session.Config.AggregationMethod,
			LearningRate:      session.Config.LearningRate,
			BatchSize:         session.Config.BatchSize,
			LocalEpochs:       session.Config.LocalEpochs,
			ClientSelection:   session.Config.ClientSelection,
			ModelConfig:       session.Config.ModelConfig,
			PrivacyConfig: requestmodels.PrivacyConfigRequest{
				DifferentialPrivacy: session.Config.PrivacyConfig.DifferentialPrivacy,
				NoiseMultiplier:     session.Config.PrivacyConfig.NoiseMultiplier,
				L2NormClip:          session.Config.PrivacyConfig.L2NormClip,
				SecureAggregation:   session.Config.PrivacyConfig.SecureAggregation,
			},
		},
		CreatedAt: session.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: session.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if session.CompletedAt != nil {
		completedAt := session.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
		response.CompletedAt = &completedAt
	}

	c.JSON(http.StatusCreated, response)
}

func (h *FederatedLearningHandler) GetSession(c *gin.Context) {
	log := gologger.WithComponent("fl_handler")

	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionIDStr).Msg("Invalid session ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	session, err := h.service.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionIDStr).Msg("Failed to get FL session")
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	response := requestmodels.FLSessionResponse{
		ID:              session.ID.String(),
		Name:            session.Name,
		Description:     session.Description,
		ModelType:       session.ModelType,
		Status:          string(session.Status),
		CurrentRound:    session.CurrentRound,
		TotalRounds:     session.TotalRounds,
		MinParticipants: session.MinParticipants,
		CreatorAddress:  session.CreatorAddress,
		Config: requestmodels.FLConfigRequest{
			AggregationMethod: session.Config.AggregationMethod,
			LearningRate:      session.Config.LearningRate,
			BatchSize:         session.Config.BatchSize,
			LocalEpochs:       session.Config.LocalEpochs,
			ClientSelection:   session.Config.ClientSelection,
			ModelConfig:       session.Config.ModelConfig,
			PrivacyConfig: requestmodels.PrivacyConfigRequest{
				DifferentialPrivacy: session.Config.PrivacyConfig.DifferentialPrivacy,
				NoiseMultiplier:     session.Config.PrivacyConfig.NoiseMultiplier,
				L2NormClip:          session.Config.PrivacyConfig.L2NormClip,
				SecureAggregation:   session.Config.PrivacyConfig.SecureAggregation,
			},
		},
		CreatedAt: session.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: session.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if session.CompletedAt != nil {
		completedAt := session.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
		response.CompletedAt = &completedAt
	}

	c.JSON(http.StatusOK, response)
}

func (h *FederatedLearningHandler) ListSessions(c *gin.Context) {
	log := gologger.WithComponent("fl_handler")

	creatorAddress := c.Query("creator")

	sessions, err := h.service.ListSessions(c.Request.Context(), creatorAddress)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list FL sessions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list sessions"})
		return
	}

	var responses []requestmodels.FLSessionResponse
	for _, session := range sessions {
		response := requestmodels.FLSessionResponse{
			ID:              session.ID.String(),
			Name:            session.Name,
			Description:     session.Description,
			ModelType:       session.ModelType,
			Status:          string(session.Status),
			CurrentRound:    session.CurrentRound,
			TotalRounds:     session.TotalRounds,
			MinParticipants: session.MinParticipants,
			CreatorAddress:  session.CreatorAddress,
			Config: requestmodels.FLConfigRequest{
				AggregationMethod: session.Config.AggregationMethod,
				LearningRate:      session.Config.LearningRate,
				BatchSize:         session.Config.BatchSize,
				LocalEpochs:       session.Config.LocalEpochs,
				ClientSelection:   session.Config.ClientSelection,
				ModelConfig:       session.Config.ModelConfig,
				PrivacyConfig: requestmodels.PrivacyConfigRequest{
					DifferentialPrivacy: session.Config.PrivacyConfig.DifferentialPrivacy,
					NoiseMultiplier:     session.Config.PrivacyConfig.NoiseMultiplier,
					L2NormClip:          session.Config.PrivacyConfig.L2NormClip,
					SecureAggregation:   session.Config.PrivacyConfig.SecureAggregation,
				},
			},
			CreatedAt: session.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: session.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}

		if session.CompletedAt != nil {
			completedAt := session.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
			response.CompletedAt = &completedAt
		}

		responses = append(responses, response)
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": responses,
		"count":    len(responses),
	})
}

func (h *FederatedLearningHandler) StartSession(c *gin.Context) {
	log := gologger.WithComponent("fl_handler")

	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionIDStr).Msg("Invalid session ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	if err := h.service.StartSession(c.Request.Context(), sessionID); err != nil {
		log.Error().Err(err).Str("session_id", sessionIDStr).Msg("Failed to start FL session")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session started successfully"})
}

func (h *FederatedLearningHandler) SubmitModelUpdate(c *gin.Context) {
	log := gologger.WithComponent("fl_handler")

	var req requestmodels.SubmitModelUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error().Err(err).Msg("Failed to bind submit model update request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.service.SubmitModelUpdate(c.Request.Context(), &req); err != nil {
		log.Error().Err(err).
			Str("session_id", req.SessionID).
			Str("round_id", req.RoundID).
			Str("runner_id", req.RunnerID).
			Msg("Failed to submit model update")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Model update submitted successfully"})
}

func (h *FederatedLearningHandler) GetRound(c *gin.Context) {
	log := gologger.WithComponent("fl_handler")

	sessionIDStr := c.Param("id")
	roundNumberStr := c.Param("roundNumber")

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionIDStr).Msg("Invalid session ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	roundNumber, err := strconv.Atoi(roundNumberStr)
	if err != nil {
		log.Error().Err(err).Str("round_number", roundNumberStr).Msg("Invalid round number")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid round number"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id":   sessionID.String(),
		"round_number": roundNumber,
		"message":      "Round details endpoint - implementation pending",
	})
}

func (h *FederatedLearningHandler) GetModel(c *gin.Context) {
	log := gologger.WithComponent("fl_handler")

	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionIDStr).Msg("Invalid session ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	model, err := h.service.GetTrainedModel(c.Request.Context(), sessionID)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionIDStr).Msg("Failed to get trained model")
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, model)
}
