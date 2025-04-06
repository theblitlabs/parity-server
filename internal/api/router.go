package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/theblitlabs/parity-server/internal/api/handlers"
	"github.com/theblitlabs/parity-server/internal/api/middleware"
)

func init() {
	// Set Gin to release mode to disable debug logging
	gin.SetMode(gin.ReleaseMode)
}

type Router struct {
	engine   *gin.Engine
	endpoint string
}

func NewRouter(taskHandler *handlers.TaskHandler, endpoint string) *Router {
	engine := gin.New()

	engine.Use(gin.Recovery())
	engine.Use(middleware.Logging())

	r := &Router{
		engine:   engine,
		endpoint: endpoint,
	}

	r.registerRoutes(taskHandler)
	return r
}

func (r *Router) registerRoutes(taskHandler *handlers.TaskHandler) {
	api := r.engine.Group(r.endpoint)
	tasks := api.Group("/tasks")
	runners := api.Group("/runners")

	tasks.POST("", taskHandler.CreateTask)
	tasks.GET("", taskHandler.ListTasks)
	tasks.GET("/:id", taskHandler.GetTask)
	tasks.POST("/:id/assign", taskHandler.AssignTask)
	tasks.GET("/:id/reward", taskHandler.GetTaskReward)
	tasks.GET("/:id/result", taskHandler.GetTaskResult)

	runners.GET("/tasks/available", taskHandler.ListAvailableTasks)
	runners.POST("/tasks/:id/start", taskHandler.StartTask)
	runners.POST("/tasks/:id/complete", taskHandler.CompleteTask)
	runners.POST("/tasks/:id/result", taskHandler.SaveTaskResult)

	runners.POST("/webhooks", taskHandler.RegisterWebhook)
	runners.DELETE("/webhooks", taskHandler.UnregisterWebhook)

	runners.POST("", taskHandler.RegisterRunner)
	runners.POST("/heartbeat", taskHandler.RunnerHeartbeat)
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
