package cli

import (
	"github.com/theblitlabs/parity-server/internal/config"
)

// PushTaskToRunner provides access to the pushTaskToRunner function
func PushTaskToRunner(taskID string, runnerID string, cfg *config.Config) error {
	return pushTaskToRunner(taskID, runnerID, cfg)
}
