package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/theblitlabs/parity-server/internal/api/handlers"
)

func registerTaskRoutes(router *gin.RouterGroup, handler *handlers.TaskHandler) {
	tasks := router.Group("/tasks")
	{
		tasks.POST("", handler.CreateTask)        
		tasks.GET("", handler.ListTasks)        
		tasks.GET("/:id", handler.GetTask)
		
		tasks.POST("/:id/assign", handler.AssignTask)
		tasks.GET("/:id/reward", handler.GetTaskReward)  
		tasks.GET("/:id/result", handler.GetTaskResult)  
	}
}

func registerRunnerRoutes(router *gin.RouterGroup, handler *handlers.TaskHandler) {
	runners := router.Group("/runners")
	{
		runners.POST("", handler.RegisterRunner)          
		runners.POST("/heartbeat", handler.RunnerHeartbeat) 
		
		runnerTasks := runners.Group("/tasks")
		{
			runnerTasks.GET("/available", handler.ListAvailableTasks)  
			runnerTasks.POST("/:id/start", handler.StartTask)         
			runnerTasks.POST("/:id/complete", handler.CompleteTask)   
			runnerTasks.POST("/:id/result", handler.SaveTaskResult)   
		}
		
		runnerWebhooks := runners.Group("/webhooks")
		{
			runnerWebhooks.POST("", handler.RegisterWebhook)      
			runnerWebhooks.DELETE("", handler.UnregisterWebhook)  
		}
	}
}

func RegisterRoutes(api *gin.RouterGroup, taskHandler *handlers.TaskHandler) {
	registerTaskRoutes(api, taskHandler)
	registerRunnerRoutes(api, taskHandler)
} 