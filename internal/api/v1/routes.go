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

func registerLLMRoutes(router *gin.RouterGroup, llmHandler *handlers.LLMHandler) {
	llm := router.Group("/llm")
	{
		llm.GET("/models", llmHandler.GetAvailableModels)
		llm.POST("/prompts", llmHandler.SubmitPrompt)
		llm.GET("/prompts", llmHandler.ListPrompts)
		llm.GET("/prompts/:id", llmHandler.GetPrompt)
		llm.POST("/prompts/:id/complete", llmHandler.CompletePrompt)
		llm.GET("/billing/metrics", llmHandler.GetBillingMetrics)
	}
}

func registerFederatedLearningRoutes(router *gin.RouterGroup, flHandler *handlers.FederatedLearningHandler) {
	fl := router.Group("/federated-learning")
	{
		fl.POST("/sessions", flHandler.CreateSession)
		fl.GET("/sessions", flHandler.ListSessions)
		fl.GET("/sessions/:id", flHandler.GetSession)
		fl.POST("/sessions/:id/start", flHandler.StartSession)
		fl.GET("/sessions/:id/model", flHandler.GetModel)
		fl.POST("/model-updates", flHandler.SubmitModelUpdate)
		fl.GET("/sessions/:id/rounds/:roundNumber", flHandler.GetRound)
	}
}

func RegisterRoutes(api *gin.RouterGroup, taskHandler *handlers.TaskHandler, runnerHandler *handlers.RunnerHandler, webhookHandler *handlers.WebhookHandler, llmHandler *handlers.LLMHandler, flHandler *handlers.FederatedLearningHandler) {
	registerTaskRoutes(api, taskHandler)
	registerRunnerRoutes(api, taskHandler, runnerHandler, webhookHandler)
	registerLLMRoutes(api, llmHandler)
	registerFederatedLearningRoutes(api, flHandler)
}
