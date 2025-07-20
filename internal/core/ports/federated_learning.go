package ports

import (
	"context"

	"github.com/google/uuid"
	requestmodels "github.com/theblitlabs/parity-server/internal/api/models"
	"github.com/theblitlabs/parity-server/internal/core/models"
)

type FLSessionRepository interface {
	Create(ctx context.Context, session *models.FederatedLearningSession) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.FederatedLearningSession, error)
	GetByCreator(ctx context.Context, creatorAddress string) ([]*models.FederatedLearningSession, error)
	GetAll(ctx context.Context) ([]*models.FederatedLearningSession, error)
	Update(ctx context.Context, session *models.FederatedLearningSession) error
	Delete(ctx context.Context, id uuid.UUID) error
	AddParticipant(ctx context.Context, sessionID uuid.UUID, runnerID string) error
	RemoveParticipant(ctx context.Context, sessionID uuid.UUID, runnerID string) error
	GetParticipants(ctx context.Context, sessionID uuid.UUID) ([]string, error)
	GetParticipantCount(ctx context.Context, sessionID uuid.UUID) (int, error)
}

type FLRoundRepository interface {
	Create(ctx context.Context, round *models.FederatedLearningRound) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.FederatedLearningRound, error)
	GetBySession(ctx context.Context, sessionID uuid.UUID) ([]*models.FederatedLearningRound, error)
	GetBySessionAndRound(ctx context.Context, sessionID uuid.UUID, roundNumber int) (*models.FederatedLearningRound, error)
	Update(ctx context.Context, round *models.FederatedLearningRound) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type FLParticipantRepository interface {
	Create(ctx context.Context, participant *models.FLRoundParticipant) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.FLRoundParticipant, error)
	GetByRound(ctx context.Context, roundID uuid.UUID) ([]*models.FLRoundParticipant, error)
	GetByRoundAndRunner(ctx context.Context, roundID uuid.UUID, runnerID string) (*models.FLRoundParticipant, error)
	Update(ctx context.Context, participant *models.FLRoundParticipant) error
	Delete(ctx context.Context, id uuid.UUID) error
	CountCompleted(ctx context.Context, roundID uuid.UUID) (int, error)
	CountTotal(ctx context.Context, roundID uuid.UUID) (int, error)
	GetByStatus(ctx context.Context, roundID uuid.UUID, status models.FLParticipantStatus) ([]*models.FLRoundParticipant, error)
}

type FederatedLearningService interface {
	CreateSession(ctx context.Context, req *requestmodels.CreateFLSessionRequest) (*models.FederatedLearningSession, error)
	GetSession(ctx context.Context, sessionID uuid.UUID) (*models.FederatedLearningSession, error)
	ListSessions(ctx context.Context, creatorAddress string) ([]*models.FederatedLearningSession, error)
	StartSession(ctx context.Context, sessionID uuid.UUID) error
	StartNextRound(ctx context.Context, sessionID uuid.UUID) error
	SubmitModelUpdate(ctx context.Context, req *requestmodels.SubmitModelUpdateRequest) error
	CheckRoundCompletion(ctx context.Context, sessionID, roundID uuid.UUID) error
	AggregateRound(ctx context.Context, sessionID, roundID uuid.UUID) error
	CompleteSession(ctx context.Context, sessionID uuid.UUID) error
	GetTrainedModel(ctx context.Context, sessionID uuid.UUID) (map[string]interface{}, error)
}
