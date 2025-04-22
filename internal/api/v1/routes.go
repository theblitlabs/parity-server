package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/theblitlabs/parity-server/internal/api/handlers"
)

// RegisterRoutes registers all v1 routes
func RegisterRoutes(api *gin.RouterGroup, taskHandler *handlers.TaskHandler) {
	tasks := api.Group("/tasks")
	{
		tasks.POST("", taskHandler.CreateTask)
		tasks.GET("", taskHandler.ListTasks)
		tasks.GET("/:id", taskHandler.GetTask)
		tasks.POST("/:id/assign", taskHandler.AssignTask)
		
		tasks.GET("/:id/reward", taskHandler.GetTaskReward)
		tasks.GET("/:id/result", taskHandler.GetTaskResult)
	}

	runners := api.Group("/runners")
	{
		runners.POST("", taskHandler.RegisterRunner)
		runners.POST("/heartbeat", taskHandler.RunnerHeartbeat)

		runners.GET("/tasks/available", taskHandler.ListAvailableTasks)
		runners.POST("/tasks/:id/start", taskHandler.StartTask)
		runners.POST("/tasks/:id/complete", taskHandler.CompleteTask)
		runners.POST("/tasks/:id/result", taskHandler.SaveTaskResult)

		runners.POST("/webhooks", taskHandler.RegisterWebhook)
		runners.DELETE("/webhooks", taskHandler.UnregisterWebhook)
	}
} 