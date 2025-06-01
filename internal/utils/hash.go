package utils

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/theblitlabs/parity-server/internal/core/models"
)

func ComputeCommandHash(command []string) string {
	commandStr := strings.Join(command, " ")
	hash := sha256.Sum256([]byte(commandStr))
	return fmt.Sprintf("%x", hash)
}

func ComputeResultHash(stdout, stderr string, exitCode int) string {
	combined := fmt.Sprintf("%s%s%d", stdout, stderr, exitCode)
	hash := sha256.Sum256([]byte(combined))
	return fmt.Sprintf("%x", hash)
}

func VerifyTaskHashes(task *models.Task, imageHashVerified, commandHashVerified string) bool {
	if task.ImageHash != "" && task.ImageHash != imageHashVerified {
		return false
	}
	if task.CommandHash != "" && task.CommandHash != commandHashVerified {
		return false
	}
	return true
}

func CheckConsensus(results []*models.TaskResult, threshold float64) (string, bool) {
	if len(results) == 0 {
		return "", false
	}

	hashCounts := make(map[string]int)
	for _, result := range results {
		if result.ResultHash != "" {
			hashCounts[result.ResultHash]++
		}
	}

	totalResults := len(results)
	requiredCount := int(float64(totalResults) * threshold)

	for hash, count := range hashCounts {
		if count >= requiredCount {
			return hash, true
		}
	}

	return "", false
}
