package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/theblitlabs/parity-server/internal/core/services"
)

type dashboardOverviewService interface {
	BuildOverview(ctx context.Context, recentTaskLimit int) (*services.DashboardOverview, error)
}

type DashboardHandler struct {
	service dashboardOverviewService
}

func NewDashboardHandler(service dashboardOverviewService) *DashboardHandler {
	return &DashboardHandler{service: service}
}

func (h *DashboardHandler) GetOverview(c *gin.Context) {
	limit := 20
	if rawLimit := c.DefaultQuery("limit", "20"); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a number"})
			return
		}
		limit = parsedLimit
	}

	overview, err := h.service.BuildOverview(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, overview)
}
