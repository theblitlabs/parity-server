package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/theblitlabs/parity-server/internal/api/handlers"
	"github.com/theblitlabs/parity-server/internal/api/middleware"
	v1 "github.com/theblitlabs/parity-server/internal/api/v1"
)

func init() {
	// Set Gin to release mode to disable debug logging
	gin.SetMode(gin.ReleaseMode)
}

type Router struct {
	engine   *gin.Engine
	endpoint string
}

func NewRouter(taskHandler *handlers.TaskHandler, runnerHandler *handlers.RunnerHandler, webhookHandler *handlers.WebhookHandler, llmHandler *handlers.LLMHandler, federatedLearningHandler *handlers.FederatedLearningHandler, endpoint string) *Router {
	engine := gin.New()

	engine.Use(gin.Recovery())
	engine.Use(middleware.Logging())

	r := &Router{
		engine:   engine,
		endpoint: endpoint,
	}

	r.registerRoutes(taskHandler, runnerHandler, webhookHandler, llmHandler, federatedLearningHandler)
	return r
}

func (r *Router) registerRoutes(taskHandler *handlers.TaskHandler, runnerHandler *handlers.RunnerHandler, webhookHandler *handlers.WebhookHandler, llmHandler *handlers.LLMHandler, federatedLearningHandler *handlers.FederatedLearningHandler) {
	api := r.engine.Group(r.endpoint)
	v1Group := api.Group("/v1")
	v1.RegisterRoutes(v1Group, taskHandler, runnerHandler, webhookHandler, llmHandler, federatedLearningHandler)
}

func (r *Router) Engine() *gin.Engine {
	return r.engine
}

func (r *Router) AddMiddleware(middleware gin.HandlerFunc) {
	r.engine.Use(middleware)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.engine.ServeHTTP(w, req)
}
