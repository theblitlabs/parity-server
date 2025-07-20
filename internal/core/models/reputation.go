package models

import (
	"time"

	"github.com/google/uuid"
)

// RunnerReputation represents the overall reputation of a runner
type RunnerReputation struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	RunnerID      string    `json:"runner_id" gorm:"type:varchar(255);not null;uniqueIndex"`
	WalletAddress string    `json:"wallet_address" gorm:"type:varchar(255)"`

	// Overall Reputation Score (0-1000 points)
	ReputationScore int    `json:"reputation_score" gorm:"type:int;default:500"`  // Starts at 500 (neutral)
	ReputationLevel string `json:"reputation_level" gorm:"type:varchar(50)"`      // Bronze/Silver/Gold/Platinum/Diamond
	Status          string `json:"status" gorm:"type:varchar(50);default:active"` // active/warning/poor/banned

	// Performance Metrics
	TotalTasksCompleted   int     `json:"total_tasks_completed" gorm:"type:int;default:0"`
	TotalTasksFailed      int     `json:"total_tasks_failed" gorm:"type:int;default:0"`
	TaskSuccessRate       float64 `json:"task_success_rate" gorm:"type:decimal(5,4);default:0"`
	AverageCompletionTime float64 `json:"avg_completion_time" gorm:"type:decimal(10,2)"` // seconds

	// Quality Metrics
	AverageQualityScore float64 `json:"avg_quality_score" gorm:"type:decimal(5,2)"` // 0-100
	CodeQualityScore    float64 `json:"code_quality_score" gorm:"type:decimal(5,2)"`
	DataHandlingScore   float64 `json:"data_handling_score" gorm:"type:decimal(5,2)"`

	// Reliability Metrics
	UptimePercentage  float64 `json:"uptime_percentage" gorm:"type:decimal(5,2)"`
	ResponseTimeScore float64 `json:"response_time_score" gorm:"type:decimal(5,2)"`
	ConsistencyScore  float64 `json:"consistency_score" gorm:"type:decimal(5,2)"`

	// Specialization Scores
	DockerExecutionScore   float64 `json:"docker_execution_score" gorm:"type:decimal(5,2)"`
	LLMInferenceScore      float64 `json:"llm_inference_score" gorm:"type:decimal(5,2)"`
	FederatedLearningScore float64 `json:"federated_learning_score" gorm:"type:decimal(5,2)"`

	// Economic Metrics
	TotalEarnings          float64 `json:"total_earnings" gorm:"type:decimal(20,8)"`
	AverageEarningsPerTask float64 `json:"avg_earnings_per_task" gorm:"type:decimal(10,4)"`
	StakeAmount            float64 `json:"stake_amount" gorm:"type:decimal(20,8)"`

	// Social Metrics
	CommunityRating  float64 `json:"community_rating" gorm:"type:decimal(3,2)"` // 1-5 stars
	TotalRatings     int     `json:"total_ratings" gorm:"type:int;default:0"`
	PositiveFeedback int     `json:"positive_feedback" gorm:"type:int;default:0"`
	NegativeFeedback int     `json:"negative_feedback" gorm:"type:int;default:0"`

	// Milestones & Achievements
	Badges              string `json:"badges" gorm:"type:text"`       // JSON array of badges
	Achievements        string `json:"achievements" gorm:"type:text"` // JSON array of achievements
	SpecialRecognitions string `json:"special_recognitions" gorm:"type:text"`

	// Blockchain Integration
	OnChainHash          string    `json:"onchain_hash" gorm:"type:varchar(255)"` // IPFS hash of reputation data
	LastBlockchainUpdate time.Time `json:"last_blockchain_update"`
	BlockchainTxHash     string    `json:"blockchain_tx_hash" gorm:"type:varchar(255)"`

	// Temporal Data
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
	LastActiveAt time.Time `json:"last_active_at"`
}

// ReputationEvent tracks individual events that affect reputation
type ReputationEvent struct {
	ID            uuid.UUID           `json:"id" gorm:"type:uuid;primaryKey"`
	RunnerID      string              `json:"runner_id" gorm:"type:varchar(255);not null;index"`
	EventType     ReputationEventType `json:"event_type" gorm:"type:varchar(100);not null"`     // task_completed, task_failed, quality_bonus, etc.
	EventCategory string              `json:"event_category" gorm:"type:varchar(100);not null"` // performance, quality, reliability, social

	// Impact
	ScoreDelta    int `json:"score_delta" gorm:"type:int"` // Change in reputation (+/-)
	PreviousScore int `json:"previous_score" gorm:"type:int"`
	NewScore      int `json:"new_score" gorm:"type:int"`

	// Context
	TaskID      *uuid.UUID             `json:"task_id,omitempty" gorm:"type:uuid"`
	SessionID   *uuid.UUID             `json:"session_id,omitempty" gorm:"type:uuid"`
	Details     string                 `json:"details" gorm:"type:text"`     // JSON metadata
	Description string                 `json:"description" gorm:"type:text"` // Human-readable description
	Metadata    map[string]interface{} `json:"metadata" gorm:"type:text"`    // Additional metadata as JSON

	// Public Visibility
	IsPublic       bool   `json:"is_public" gorm:"type:boolean;default:true"`
	DisplayMessage string `json:"display_message" gorm:"type:varchar(500)"` // Human-readable event description

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// ReputationSnapshot stores historical reputation data for blockchain
type ReputationSnapshot struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	RunnerID     string    `json:"runner_id" gorm:"type:varchar(255);not null;index"`
	SnapshotType string    `json:"snapshot_type" gorm:"type:varchar(50)"` // daily, weekly, milestone

	// Snapshot Data
	ReputationScore int    `json:"reputation_score"`
	ReputationLevel string `json:"reputation_level"`
	QualityMetrics  string `json:"quality_metrics" gorm:"type:text"`  // JSON object
	PerformanceData string `json:"performance_data" gorm:"type:text"` // JSON object

	// Blockchain Storage
	IPFSHash         string `json:"ipfs_hash" gorm:"type:varchar(255)"`          // Hash of stored data
	BlockchainDealID string `json:"blockchain_deal_id" gorm:"type:varchar(255)"` // Blockchain storage deal
	OnChainTxHash    string `json:"onchain_tx_hash" gorm:"type:varchar(255)"`    // Transaction recording the hash

	// Verification
	DataHash   string `json:"data_hash" gorm:"type:varchar(255)"` // SHA256 of snapshot data
	IsVerified bool   `json:"is_verified" gorm:"type:boolean;default:false"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// ReputationLeaderboard represents public leaderboard rankings
type ReputationLeaderboard struct {
	ID              uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	LeaderboardType string    `json:"leaderboard_type" gorm:"type:varchar(100)"` // overall, docker, llm, fl, weekly, monthly
	Period          string    `json:"period" gorm:"type:varchar(50)"`            // all_time, monthly, weekly

	// Rankings (JSON array)
	Rankings string `json:"rankings" gorm:"type:text"` // JSON array of runner rankings

	// Metadata
	TotalRunners int       `json:"total_runners"`
	LastUpdated  time.Time `json:"last_updated" gorm:"autoUpdateTime"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// ReputationLevel constants
const (
	ReputationLevelBronze   = "Bronze"   // 0-199
	ReputationLevelSilver   = "Silver"   // 200-399
	ReputationLevelGold     = "Gold"     // 400-599
	ReputationLevelPlatinum = "Platinum" // 600-799
	ReputationLevelDiamond  = "Diamond"  // 800-1000
)

// ReputationStatus constants
const (
	ReputationStatusActive  = "active"
	ReputationStatusWarning = "warning"
	ReputationStatusPoor    = "poor"
	ReputationStatusBanned  = "banned"
)

// ReputationEventType defines the type of reputation events
type ReputationEventType string

// Event types for reputation changes
const (
	ReputationEventTypeTaskCompleted     ReputationEventType = "task_completed"
	ReputationEventTypeTaskFailed        ReputationEventType = "task_failed"
	ReputationEventTypeHighQuality       ReputationEventType = "high_quality_work"
	ReputationEventTypePoorQuality       ReputationEventType = "poor_quality_work"
	ReputationEventTypeFastCompletion    ReputationEventType = "fast_completion"
	ReputationEventTypeSlowCompletion    ReputationEventType = "slow_completion"
	ReputationEventTypeHighUptime        ReputationEventType = "high_uptime"
	ReputationEventTypeDowntime          ReputationEventType = "downtime"
	ReputationEventTypePositiveFeedback  ReputationEventType = "positive_feedback"
	ReputationEventTypeNegativeFeedback  ReputationEventType = "negative_feedback"
	ReputationEventTypeMilestoneReached  ReputationEventType = "milestone_reached"
	ReputationEventTypeBadgeEarned       ReputationEventType = "badge_earned"
	ReputationEventTypePenalty           ReputationEventType = "penalty"
	ReputationEventTypeBonus             ReputationEventType = "bonus"
	ReputationEventTypeMaliciousBehavior ReputationEventType = "malicious_behavior"
	ReputationEventTypeSlashing          ReputationEventType = "slashing"
	ReputationEventTypePeerReport        ReputationEventType = "peer_report"
)

// Legacy string constants for backward compatibility
const (
	EventTaskCompleted    = "task_completed"
	EventTaskFailed       = "task_failed"
	EventHighQuality      = "high_quality_work"
	EventPoorQuality      = "poor_quality_work"
	EventFastCompletion   = "fast_completion"
	EventSlowCompletion   = "slow_completion"
	EventHighUptime       = "high_uptime"
	EventDowntime         = "downtime"
	EventPositiveFeedback = "positive_feedback"
	EventNegativeFeedback = "negative_feedback"
	EventMilestoneReached = "milestone_reached"
	EventBadgeEarned      = "badge_earned"
	EventPenalty          = "penalty"
	EventBonus            = "bonus"
)

// Helper methods
func (r *RunnerReputation) CalculateReputationLevel() string {
	switch {
	case r.ReputationScore >= 800:
		return ReputationLevelDiamond
	case r.ReputationScore >= 600:
		return ReputationLevelPlatinum
	case r.ReputationScore >= 400:
		return ReputationLevelGold
	case r.ReputationScore >= 200:
		return ReputationLevelSilver
	default:
		return ReputationLevelBronze
	}
}

func (r *RunnerReputation) GetSpecializationScores() map[string]float64 {
	return map[string]float64{
		"docker_execution":   r.DockerExecutionScore,
		"llm_inference":      r.LLMInferenceScore,
		"federated_learning": r.FederatedLearningScore,
	}
}

func (r *RunnerReputation) GetPublicProfile() map[string]interface{} {
	return map[string]interface{}{
		"runner_id":         r.RunnerID,
		"reputation_score":  r.ReputationScore,
		"reputation_level":  r.ReputationLevel,
		"task_success_rate": r.TaskSuccessRate,
		"total_tasks":       r.TotalTasksCompleted,
		"avg_quality":       r.AverageQualityScore,
		"specializations":   r.GetSpecializationScores(),
		"community_rating":  r.CommunityRating,
		"uptime_percentage": r.UptimePercentage,
		"badges":            r.Badges,
		"last_active":       r.LastActiveAt,
	}
}

// NetworkStats represents overall network health and statistics
type NetworkStats struct {
	TotalRunners   int       `json:"total_runners"`
	ActiveRunners  int       `json:"active_runners"`
	BannedRunners  int       `json:"banned_runners"`
	NetworkHealth  string    `json:"network_health"` // healthy, degraded, critical
	TotalTasks     int       `json:"total_tasks"`
	CompletedTasks int       `json:"completed_tasks"`
	FailedTasks    int       `json:"failed_tasks"`
	AverageQuality float64   `json:"average_quality"`
	LastUpdated    time.Time `json:"last_updated"`
}
