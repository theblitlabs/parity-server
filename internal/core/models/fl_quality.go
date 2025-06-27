package models

import (
	"time"

	"github.com/google/uuid"
)

// ParticipantQualityMetrics tracks the quality and reliability of FL participants
type ParticipantQualityMetrics struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	RunnerID  string    `json:"runner_id" gorm:"type:varchar(255);not null"`
	SessionID uuid.UUID `json:"session_id" gorm:"type:uuid;not null"`

	// Performance Metrics
	AverageTrainingTime float64 `json:"average_training_time"` // milliseconds
	TaskCompletionRate  float64 `json:"task_completion_rate"`  // percentage
	ModelQualityScore   float64 `json:"model_quality_score"`   // 0-100
	DataQualityScore    float64 `json:"data_quality_score"`    // 0-100

	// Reliability Metrics
	UptimePercentage     float64 `json:"uptime_percentage"`     // percentage
	HeartbeatConsistency float64 `json:"heartbeat_consistency"` // variance in heartbeat timing
	ErrorRate            float64 `json:"error_rate"`            // percentage of failed tasks

	// Network Quality
	AverageLatency       float64 `json:"average_latency"`       // milliseconds
	BandwidthUtilization float64 `json:"bandwidth_utilization"` // bytes/second
	ConnectionStability  float64 `json:"connection_stability"`  // 0-100

	// Resource Utilization
	CPUEfficiency      float64 `json:"cpu_efficiency"`      // 0-100
	MemoryEfficiency   float64 `json:"memory_efficiency"`   // 0-100
	StorageUtilization float64 `json:"storage_utilization"` // percentage

	// Quality Score (computed)
	OverallQualityScore float64 `json:"overall_quality_score"` // 0-100
	QualityTrend        string  `json:"quality_trend"`         // improving/stable/declining

	CreatedAt time.Time `json:"created_at" gorm:"type:timestamp"`
	UpdatedAt time.Time `json:"updated_at" gorm:"type:timestamp"`
}

// SessionQualityMetrics tracks overall session quality
type SessionQualityMetrics struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	SessionID uuid.UUID `json:"session_id" gorm:"type:uuid;not null"`

	// Convergence Quality
	ConvergenceRate     float64 `json:"convergence_rate"`     // rounds to convergence
	ModelStability      float64 `json:"model_stability"`      // variance between rounds
	AccuracyImprovement float64 `json:"accuracy_improvement"` // per round

	// Participant Quality
	ParticipantRetention      float64 `json:"participant_retention"`   // percentage
	AverageParticipantQuality float64 `json:"avg_participant_quality"` // 0-100
	ParticipantConsistency    float64 `json:"participant_consistency"` // variance in quality

	// Data Quality
	DataDistribution          string  `json:"data_distribution"`          // iid/non_iid quality
	DataIntegrity             float64 `json:"data_integrity"`             // percentage
	PartitioningEffectiveness float64 `json:"partitioning_effectiveness"` // 0-100

	// Infrastructure Quality
	SystemLatency       float64 `json:"system_latency"`       // milliseconds
	NetworkEfficiency   float64 `json:"network_efficiency"`   // 0-100
	ResourceUtilization float64 `json:"resource_utilization"` // percentage

	// Security & Privacy
	PrivacyCompliance     float64 `json:"privacy_compliance"`      // 0-100
	SecurityScore         float64 `json:"security_score"`          // 0-100
	AnomalyDetectionScore float64 `json:"anomaly_detection_score"` // 0-100

	// Overall Assessment
	InfrastructureQuality float64 `json:"infrastructure_quality"` // 0-100
	SessionHealthScore    float64 `json:"session_health_score"`   // 0-100

	CreatedAt time.Time `json:"created_at" gorm:"type:timestamp"`
	UpdatedAt time.Time `json:"updated_at" gorm:"type:timestamp"`
}

// NetworkQualityMetrics tracks network infrastructure quality
type NetworkQualityMetrics struct {
	ID     uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Region string    `json:"region" gorm:"type:varchar(100)"`

	// Network Performance
	AverageLatency  float64 `json:"average_latency"`  // milliseconds
	LatencyVariance float64 `json:"latency_variance"` // milliseconds
	Throughput      float64 `json:"throughput"`       // bytes/second
	PacketLoss      float64 `json:"packet_loss"`      // percentage

	// Connection Quality
	ConnectionSuccess   float64 `json:"connection_success"`   // percentage
	ConnectionStability float64 `json:"connection_stability"` // percentage
	ReconnectionRate    float64 `json:"reconnection_rate"`    // per hour

	// IPFS/Filecoin Performance
	IPFSDownloadSpeed float64 `json:"ipfs_download_speed"` // bytes/second
	IPFSUploadSpeed   float64 `json:"ipfs_upload_speed"`   // bytes/second
	IPFSAvailability  float64 `json:"ipfs_availability"`   // percentage

	CreatedAt time.Time `json:"created_at" gorm:"type:timestamp"`
	UpdatedAt time.Time `json:"updated_at" gorm:"type:timestamp"`
}

// QualityAlert represents infrastructure quality alerts
type QualityAlert struct {
	ID         uuid.UUID   `json:"id" gorm:"type:uuid;primaryKey"`
	AlertType  string      `json:"alert_type" gorm:"type:varchar(100)"` // performance/security/reliability
	Severity   string      `json:"severity" gorm:"type:varchar(50)"`    // low/medium/high/critical
	EntityType string      `json:"entity_type" gorm:"type:varchar(50)"` // participant/session/network
	EntityID   string      `json:"entity_id" gorm:"type:varchar(255)"`  // ID of the entity
	Message    string      `json:"message" gorm:"type:text"`
	Details    interface{} `json:"details" gorm:"type:jsonb"`
	Status     string      `json:"status" gorm:"type:varchar(50)"` // active/resolved/acknowledged
	CreatedAt  time.Time   `json:"created_at" gorm:"type:timestamp"`
	ResolvedAt *time.Time  `json:"resolved_at" gorm:"type:timestamp"`
}

// QualitySLA defines service level agreements for FL infrastructure
type QualitySLA struct {
	ID   uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Name string    `json:"name" gorm:"type:varchar(255)"`

	// Performance SLAs
	MaxTrainingTime   int64   `json:"max_training_time"`   // milliseconds
	MinTaskCompletion float64 `json:"min_task_completion"` // percentage
	MinUptime         float64 `json:"min_uptime"`          // percentage
	MaxLatency        float64 `json:"max_latency"`         // milliseconds

	// Quality SLAs
	MinModelQuality    float64 `json:"min_model_quality"`    // 0-100
	MinDataQuality     float64 `json:"min_data_quality"`     // 0-100
	MinConvergenceRate float64 `json:"min_convergence_rate"` // rounds
	MaxErrorRate       float64 `json:"max_error_rate"`       // percentage

	// Security SLAs
	MinSecurityScore     float64 `json:"min_security_score"`     // 0-100
	MinPrivacyCompliance float64 `json:"min_privacy_compliance"` // 0-100
	MaxAnomalyThreshold  float64 `json:"max_anomaly_threshold"`  // 0-100

	IsActive  bool      `json:"is_active" gorm:"default:true"`
	CreatedAt time.Time `json:"created_at" gorm:"type:timestamp"`
	UpdatedAt time.Time `json:"updated_at" gorm:"type:timestamp"`
}

// QualityReport provides periodic quality assessment reports
type QualityReport struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	ReportType string    `json:"report_type" gorm:"type:varchar(100)"` // daily/weekly/monthly
	Period     string    `json:"period" gorm:"type:varchar(100)"`      // 2024-01-01 to 2024-01-07

	// Overall Metrics
	OverallHealthScore   float64 `json:"overall_health_score"`  // 0-100
	InfrastructureUptime float64 `json:"infrastructure_uptime"` // percentage
	TotalSessions        int     `json:"total_sessions"`
	SuccessfulSessions   int     `json:"successful_sessions"`

	// Performance Summary
	AverageSessionTime  float64 `json:"average_session_time"` // minutes
	AverageParticipants float64 `json:"average_participants"`
	TotalDataProcessed  int64   `json:"total_data_processed"` // bytes

	// Quality Trends
	QualityTrend     string `json:"quality_trend"`     // improving/stable/declining
	PerformanceTrend string `json:"performance_trend"` // improving/stable/declining
	SecurityTrend    string `json:"security_trend"`    // improving/stable/declining

	// Incidents & Alerts
	TotalAlerts    int `json:"total_alerts"`
	CriticalAlerts int `json:"critical_alerts"`
	ResolvedAlerts int `json:"resolved_alerts"`

	// Recommendations
	Recommendations []string `json:"recommendations" gorm:"type:jsonb"`
	ActionItems     []string `json:"action_items" gorm:"type:jsonb"`

	GeneratedAt time.Time `json:"generated_at" gorm:"type:timestamp"`
	CreatedAt   time.Time `json:"created_at" gorm:"type:timestamp"`
}

// Helper functions for quality scoring
func CalculateOverallQualityScore(performance, reliability, networkQuality, resourceEfficiency float64) float64 {
	weights := map[string]float64{
		"performance":         0.3,
		"reliability":         0.3,
		"network_quality":     0.2,
		"resource_efficiency": 0.2,
	}

	return (performance * weights["performance"]) +
		(reliability * weights["reliability"]) +
		(networkQuality * weights["network_quality"]) +
		(resourceEfficiency * weights["resource_efficiency"])
}

func DetermineQualityTrend(currentScore, previousScore float64) string {
	threshold := 5.0 // 5% change threshold

	change := ((currentScore - previousScore) / previousScore) * 100

	if change > threshold {
		return "improving"
	} else if change < -threshold {
		return "declining"
	}
	return "stable"
}
