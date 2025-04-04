package api

import (
	"github.com/gin-gonic/gin"
	"github.com/theblitlabs/parity-server/internal/api/handlers"
	"github.com/theblitlabs/parity-server/internal/api/middleware"
)

func init() {
	// Set Gin to release mode to disable debug logging
	gin.SetMode(gin.ReleaseMode)
}

type Router struct {
	*gin.Engine
	middleware []gin.HandlerFunc
	endpoint   string
}

func NewRouter(taskHandler *handlers.TaskHandler, endpoint string) *Router {
	engine := gin.New()

	r := &Router{
		Engine:     engine,
		middleware: []gin.HandlerFunc{gin.Recovery(), middleware.Logging()},
		endpoint:   endpoint,
	}

	r.registerRoutes(taskHandler)
	return r
}

func (r *Router) registerRoutes(taskHandler *handlers.TaskHandler) {
	for _, m := range r.middleware {
		r.Use(m)
	}

	api := r.Group(r.endpoint)
	tasks := api.Group("/tasks")
	runners := api.Group("/runners")

	// Task routes
	tasks.POST("", taskHandler.CreateTask)
	tasks.GET("", taskHandler.ListTasks)
	tasks.GET("/:id", taskHandler.GetTask)
	tasks.POST("/:id/assign", taskHandler.AssignTask)
	tasks.GET("/:id/reward", taskHandler.GetTaskReward)
	tasks.GET("/:id/result", taskHandler.GetTaskResult)

	// Runner routes
	runners.GET("/tasks/available", taskHandler.ListAvailableTasks)
	runners.POST("/tasks/:id/start", taskHandler.StartTask)
	runners.POST("/tasks/:id/complete", taskHandler.CompleteTask)
	runners.POST("/tasks/:id/result", taskHandler.SaveTaskResult)

	runners.POST("/webhooks", taskHandler.RegisterWebhook)
	runners.DELETE("/webhooks/:device_id", taskHandler.UnregisterWebhook)

	runners.POST("", taskHandler.RegisterRunner)
	runners.POST("/heartbeat", taskHandler.RunnerHeartbeat)
}

func (r *Router) AddMiddleware(middleware gin.HandlerFunc) {
	r.Use(middleware)
}
