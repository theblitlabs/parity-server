package api

import (
	"github.com/gorilla/mux"
	"github.com/theblitlabs/parity-server/internal/api/handlers"
	"github.com/theblitlabs/parity-server/internal/api/middleware"
)

type Router struct {
	*mux.Router
	middleware []mux.MiddlewareFunc
	endpoint   string
}

func NewRouter(taskHandler *handlers.TaskHandler, endpoint string) *Router {
	r := &Router{
		Router:     mux.NewRouter(),
		middleware: []mux.MiddlewareFunc{middleware.Logging},
		endpoint:   endpoint,
	}

	apiRouter := r.Router.PathPrefix("/").Subrouter()
	for _, m := range r.middleware {
		apiRouter.Use(m)
	}

	r.registerRoutes(apiRouter, taskHandler)
	return r
}

func (r *Router) registerRoutes(router *mux.Router, taskHandler *handlers.TaskHandler) {
	api := router.PathPrefix(r.endpoint).Subrouter()
	tasks := api.PathPrefix("/tasks").Subrouter()
	runners := api.PathPrefix("/runners").Subrouter()

	tasks.HandleFunc("", taskHandler.CreateTask).Methods("POST")
	tasks.HandleFunc("", taskHandler.ListTasks).Methods("GET")
	tasks.HandleFunc("/{id}", taskHandler.GetTask).Methods("GET")
	tasks.HandleFunc("/{id}/assign", taskHandler.AssignTask).Methods("POST")
	tasks.HandleFunc("/{id}/reward", taskHandler.GetTaskReward).Methods("GET")
	tasks.HandleFunc("/{id}/result", taskHandler.GetTaskResult).Methods("GET")

	runners.HandleFunc("/tasks/available", taskHandler.ListAvailableTasks).Methods("GET")
	runners.HandleFunc("/tasks/{id}/start", taskHandler.StartTask).Methods("POST")
	runners.HandleFunc("/tasks/{id}/complete", taskHandler.CompleteTask).Methods("POST")
	runners.HandleFunc("/tasks/{id}/result", taskHandler.SaveTaskResult).Methods("POST")

	runners.HandleFunc("/webhooks", taskHandler.RegisterWebhook).Methods("POST")
	runners.HandleFunc("/webhooks/{device_id}", taskHandler.UnregisterWebhook).Methods("DELETE")

	// Add new runner registration and heartbeat endpoints
	runners.HandleFunc("", taskHandler.RegisterRunner).Methods("POST")
	runners.HandleFunc("/heartbeat", taskHandler.RunnerHeartbeat).Methods("POST")
}

func (r *Router) AddMiddleware(middleware mux.MiddlewareFunc) {
	r.Use(middleware)
}
