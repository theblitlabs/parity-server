package repositories

import (
	"context"

	"github.com/theblitlabs/parity-server/internal/core/models"
	"gorm.io/gorm"
)

type ReputationRepository struct {
	db *gorm.DB
}

func NewReputationRepository(db *gorm.DB) *ReputationRepository {
	return &ReputationRepository{db: db}
}

// Runner Reputation Management
func (r *ReputationRepository) CreateRunnerReputation(ctx context.Context, reputation *models.RunnerReputation) error {
	return r.db.WithContext(ctx).Create(reputation).Error
}

func (r *ReputationRepository) GetRunnerReputation(ctx context.Context, runnerID string) (*models.RunnerReputation, error) {
	var reputation models.RunnerReputation
	err := r.db.WithContext(ctx).Where("runner_id = ?", runnerID).First(&reputation).Error
	if err != nil {
		return nil, err
	}
	return &reputation, nil
}

func (r *ReputationRepository) UpdateRunnerReputation(ctx context.Context, reputation *models.RunnerReputation) error {
	return r.db.WithContext(ctx).Save(reputation).Error
}

func (r *ReputationRepository) GetTopRunners(ctx context.Context, limit int) ([]*models.RunnerReputation, error) {
	var reputations []*models.RunnerReputation
	err := r.db.WithContext(ctx).
		Order("reputation_score DESC").
		Limit(limit).
		Find(&reputations).Error
	return reputations, err
}

func (r *ReputationRepository) GetRunnersByLevel(ctx context.Context, level string) ([]*models.RunnerReputation, error) {
	var reputations []*models.RunnerReputation
	err := r.db.WithContext(ctx).
		Where("reputation_level = ?", level).
		Order("reputation_score DESC").
		Find(&reputations).Error
	return reputations, err
}

// Reputation Events
func (r *ReputationRepository) CreateReputationEvent(ctx context.Context, event *models.ReputationEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *ReputationRepository) GetRunnerEvents(ctx context.Context, runnerID string, limit int) ([]*models.ReputationEvent, error) {
	var events []*models.ReputationEvent
	err := r.db.WithContext(ctx).
		Where("runner_id = ?", runnerID).
		Order("created_at DESC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

func (r *ReputationRepository) GetPublicEvents(ctx context.Context, limit int) ([]*models.ReputationEvent, error) {
	var events []*models.ReputationEvent
	err := r.db.WithContext(ctx).
		Where("is_public = ?", true).
		Order("created_at DESC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

func (r *ReputationRepository) GetEventsByType(ctx context.Context, eventType string, limit int) ([]*models.ReputationEvent, error) {
	var events []*models.ReputationEvent
	err := r.db.WithContext(ctx).
		Where("event_type = ?", eventType).
		Order("created_at DESC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

// Reputation Snapshots
func (r *ReputationRepository) CreateReputationSnapshot(ctx context.Context, snapshot *models.ReputationSnapshot) error {
	return r.db.WithContext(ctx).Create(snapshot).Error
}

func (r *ReputationRepository) GetLatestSnapshot(ctx context.Context, runnerID string) (*models.ReputationSnapshot, error) {
	var snapshot models.ReputationSnapshot
	err := r.db.WithContext(ctx).
		Where("runner_id = ?", runnerID).
		Order("created_at DESC").
		First(&snapshot).Error
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *ReputationRepository) GetSnapshotHistory(ctx context.Context, runnerID string, limit int) ([]*models.ReputationSnapshot, error) {
	var snapshots []*models.ReputationSnapshot
	err := r.db.WithContext(ctx).
		Where("runner_id = ?", runnerID).
		Order("created_at DESC").
		Limit(limit).
		Find(&snapshots).Error
	return snapshots, err
}

func (r *ReputationRepository) GetSnapshotsByType(ctx context.Context, snapshotType string, limit int) ([]*models.ReputationSnapshot, error) {
	var snapshots []*models.ReputationSnapshot
	err := r.db.WithContext(ctx).
		Where("snapshot_type = ?", snapshotType).
		Order("created_at DESC").
		Limit(limit).
		Find(&snapshots).Error
	return snapshots, err
}

// Leaderboards
func (r *ReputationRepository) CreateLeaderboard(ctx context.Context, leaderboard *models.ReputationLeaderboard) error {
	return r.db.WithContext(ctx).Create(leaderboard).Error
}

func (r *ReputationRepository) GetLeaderboard(ctx context.Context, leaderboardType, period string) (*models.ReputationLeaderboard, error) {
	var leaderboard models.ReputationLeaderboard
	err := r.db.WithContext(ctx).
		Where("leaderboard_type = ? AND period = ?", leaderboardType, period).
		Order("created_at DESC").
		First(&leaderboard).Error
	if err != nil {
		return nil, err
	}
	return &leaderboard, nil
}

func (r *ReputationRepository) UpdateLeaderboard(ctx context.Context, leaderboard *models.ReputationLeaderboard) error {
	return r.db.WithContext(ctx).Save(leaderboard).Error
}

func (r *ReputationRepository) GetAllLeaderboards(ctx context.Context) ([]*models.ReputationLeaderboard, error) {
	var leaderboards []*models.ReputationLeaderboard
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&leaderboards).Error
	return leaderboards, err
}

// Analytics
func (r *ReputationRepository) GetReputationStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total runners
	var totalRunners int64
	if err := r.db.WithContext(ctx).Model(&models.RunnerReputation{}).Count(&totalRunners).Error; err != nil {
		return nil, err
	}
	stats["total_runners"] = totalRunners

	// Average reputation score
	var avgScore float64
	if err := r.db.WithContext(ctx).Model(&models.RunnerReputation{}).
		Select("AVG(reputation_score)").Scan(&avgScore).Error; err != nil {
		return nil, err
	}
	stats["average_reputation_score"] = avgScore

	// Reputation levels distribution
	var levelCounts []struct {
		Level string
		Count int64
	}
	if err := r.db.WithContext(ctx).Model(&models.RunnerReputation{}).
		Select("reputation_level as level, COUNT(*) as count").
		Group("reputation_level").
		Scan(&levelCounts).Error; err != nil {
		return nil, err
	}
	stats["level_distribution"] = levelCounts

	return stats, nil
}

func (r *ReputationRepository) GetRunnerRankings(ctx context.Context, specialty string) ([]*models.RunnerReputation, error) {
	query := r.db.WithContext(ctx).Order("reputation_score DESC")

	if specialty != "" {
		query = query.Where("docker_execution_score > ? OR average_quality_score > ?", 80.0, 80.0)
	}

	var reputations []*models.RunnerReputation
	err := query.Find(&reputations).Error
	return reputations, err
}

func (r *ReputationRepository) SearchRunners(ctx context.Context, query string, filters map[string]interface{}) ([]*models.RunnerReputation, error) {
	dbQuery := r.db.WithContext(ctx)

	if query != "" {
		dbQuery = dbQuery.Where("runner_id LIKE ? OR wallet_address LIKE ?", "%"+query+"%", "%"+query+"%")
	}

	for key, value := range filters {
		switch key {
		case "min_reputation_score":
			if score, ok := value.(float64); ok {
				dbQuery = dbQuery.Where("reputation_score >= ?", score)
			}
		case "reputation_level":
			if level, ok := value.(string); ok {
				dbQuery = dbQuery.Where("reputation_level = ?", level)
			}
		case "status":
			if status, ok := value.(string); ok {
				dbQuery = dbQuery.Where("status = ?", status)
			}
		}
	}

	var reputations []*models.RunnerReputation
	err := dbQuery.Order("reputation_score DESC").Find(&reputations).Error
	return reputations, err
}
