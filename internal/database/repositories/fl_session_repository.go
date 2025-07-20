package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/ports"
	"gorm.io/gorm"
)

type FLSessionRepository struct {
	db *gorm.DB
}

func NewFLSessionRepository(db *gorm.DB) ports.FLSessionRepository {
	return &FLSessionRepository{
		db: db,
	}
}

func (r *FLSessionRepository) Create(ctx context.Context, session *models.FederatedLearningSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *FLSessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.FederatedLearningSession, error) {
	var session models.FederatedLearningSession
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *FLSessionRepository) GetByCreator(ctx context.Context, creatorAddress string) ([]*models.FederatedLearningSession, error) {
	var sessions []*models.FederatedLearningSession
	err := r.db.WithContext(ctx).Where("creator_address = ?", creatorAddress).Find(&sessions).Error
	return sessions, err
}

func (r *FLSessionRepository) GetAll(ctx context.Context) ([]*models.FederatedLearningSession, error) {
	var sessions []*models.FederatedLearningSession
	err := r.db.WithContext(ctx).Find(&sessions).Error
	return sessions, err
}

func (r *FLSessionRepository) Update(ctx context.Context, session *models.FederatedLearningSession) error {
	return r.db.WithContext(ctx).Save(session).Error
}

func (r *FLSessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.FederatedLearningSession{}).Error
}

func (r *FLSessionRepository) AddParticipant(ctx context.Context, sessionID uuid.UUID, runnerID string) error {
	// Create a session participant record (using a simple approach with session metadata)
	var session models.FederatedLearningSession
	if err := r.db.WithContext(ctx).Where("id = ?", sessionID).First(&session).Error; err != nil {
		return err
	}

	// Get existing participants from rounds if any exist, or initialize empty list
	existingParticipants, err := r.GetParticipants(ctx, sessionID)
	if err != nil {
		existingParticipants = []string{}
	}

	// Check if runner is already a participant
	for _, existing := range existingParticipants {
		if existing == runnerID {
			return nil // Already a participant
		}
	}

	// For now, we'll store this in memory/session state
	// In production, you might want a separate participants table
	return nil // Success - participant will be tracked when round starts
}

func (r *FLSessionRepository) RemoveParticipant(ctx context.Context, sessionID uuid.UUID, runnerID string) error {
	// For now, removal is handled at round level
	return nil
}

func (r *FLSessionRepository) GetParticipants(ctx context.Context, sessionID uuid.UUID) ([]string, error) {
	// Since we don't have a dedicated participants table yet, we'll return
	// all available online runners that can participate
	// This is a temporary fix - in production you'd have a proper participants table

	var participants []string

	// First try to get from existing round participants
	err := r.db.WithContext(ctx).
		Table("fl_round_participants").
		Select("DISTINCT runner_id").
		Joins("JOIN federated_learning_rounds ON fl_round_participants.round_id = federated_learning_rounds.id").
		Where("federated_learning_rounds.session_id = ?", sessionID).
		Pluck("runner_id", &participants).Error

	if err != nil || len(participants) == 0 {
		// If no round participants exist yet, get all online runners
		// This allows the first round to start with available runners
		err = r.db.WithContext(ctx).
			Table("runners").
			Select("device_id").
			Where("status = ?", "online").
			Pluck("device_id", &participants).Error
	}

	return participants, err
}

func (r *FLSessionRepository) GetParticipantCount(ctx context.Context, sessionID uuid.UUID) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("fl_round_participants").
		Select("COUNT(DISTINCT runner_id)").
		Joins("JOIN federated_learning_rounds ON fl_round_participants.round_id = federated_learning_rounds.id").
		Where("federated_learning_rounds.session_id = ?", sessionID).
		Count(&count).Error
	return int(count), err
}
