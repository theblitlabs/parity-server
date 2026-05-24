package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/theblitlabs/parity-server/internal/core/services"
)

type dashboardHandlerServiceStub struct {
	overview *services.DashboardOverview
}

func (s *dashboardHandlerServiceStub) BuildOverview(ctx context.Context, recentTaskLimit int) (*services.DashboardOverview, error) {
	return s.overview, nil
}

func TestDashboardHandlerGetOverview(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewDashboardHandler(&dashboardHandlerServiceStub{
		overview: &services.DashboardOverview{
			Summary: services.DashboardSummary{
				Tasks: services.TaskStatusCounts{Total: 2, Pending: 1, Running: 1},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/overview?limit=12", nil)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req

	handler.GetOverview(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload services.DashboardOverview
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if payload.Summary.Tasks.Total != 2 {
		t.Fatalf("total tasks = %d, want 2", payload.Summary.Tasks.Total)
	}
}
