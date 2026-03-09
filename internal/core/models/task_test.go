package models

import (
	"encoding/json"
	"testing"
)

func TestTaskValidateAllowsRegistryBackedDockerImage(t *testing.T) {
	task := NewTask()
	task.Title = "hello"
	task.Description = "registry image task"
	task.Type = TaskTypeDocker
	task.Environment = &EnvironmentConfig{
		Type: "docker",
		Config: map[string]interface{}{
			"workdir": "/",
		},
	}

	config, err := json.Marshal(TaskConfig{
		ImageName: "hello-world:latest",
	})
	if err != nil {
		t.Fatalf("failed to marshal task config: %v", err)
	}
	task.Config = config

	if err := task.Validate(); err != nil {
		t.Fatalf("expected registry-backed docker task to validate, got error: %v", err)
	}
}
