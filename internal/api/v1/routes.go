package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/theblitlabs/parity-server/internal/api/handlers"
)

func registerTaskRoutes(router *gin.RouterGroup, taskHandler *handlers.TaskHandler) {
	tasks := router.Group("/tasks")
	{
		tasks.POST("", taskHandler.CreateTask)
		tasks.GET("", taskHandler.ListTasks)
		tasks.GET("/:id", taskHandler.GetTask)
		tasks.GET("/:id/reward", taskHandler.GetTaskReward)
		tasks.GET("/:id/result", taskHandler.GetTaskResult)
		tasks.POST("/:id/verify-hashes", taskHandler.VerifyTaskHashes)
	}
}

func registerRunnerRoutes(router *gin.RouterGroup, taskHandler *handlers.TaskHandler, runnerHandler *handlers.RunnerHandler, webhookHandler *handlers.WebhookHandler) {
	runners := router.Group("/runners")
	{
		runners.POST("", runnerHandler.RegisterRunner)
		runners.POST("/heartbeat", runnerHandler.RunnerHeartbeat)

		runnerTasks := runners.Group("/tasks")
		{
			runnerTasks.GET("/available", runnerHandler.ListAvailableTasks)
			runnerTasks.POST("/:id/start", runnerHandler.StartTask)
			runnerTasks.POST("/:id/complete", runnerHandler.CompleteTask)
			runnerTasks.POST("/:id/result", taskHandler.SaveTaskResult)
		}

		runnerWebhooks := runners.Group("/webhooks")
		{
			runnerWebhooks.POST("", webhookHandler.RegisterWebhook)
			runnerWebhooks.DELETE("", webhookHandler.UnregisterWebhook)
		}
	}
}

func RegisterRoutes(api *gin.RouterGroup, taskHandler *handlers.TaskHandler, runnerHandler *handlers.RunnerHandler, webhookHandler *handlers.WebhookHandler) {
	registerTaskRoutes(api, taskHandler)
	registerRunnerRoutes(api, taskHandler, runnerHandler, webhookHandler)
}
