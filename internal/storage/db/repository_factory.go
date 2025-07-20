package db

import (
	"github.com/theblitlabs/parity-server/internal/core/ports"
	"github.com/theblitlabs/parity-server/internal/database/repositories"
	"gorm.io/gorm"
)

type RepositoryFactory struct {
	db *gorm.DB
}

func NewRepositoryFactory(db *gorm.DB) *RepositoryFactory {
	return &RepositoryFactory{
		db: db,
	}
}

func NewRepositoryFactoryFromManager(manager *DBManager) *RepositoryFactory {
	return &RepositoryFactory{
		db: manager.GetDB(),
	}
}

func (f *RepositoryFactory) TaskRepository() *repositories.TaskRepository {
	return repositories.NewTaskRepository(f.db)
}

func (f *RepositoryFactory) RunnerRepository() *repositories.RunnerRepository {
	return repositories.NewRunnerRepository(f.db)
}

func (f *RepositoryFactory) FLSessionRepository() ports.FLSessionRepository {
	return repositories.NewFLSessionRepository(f.db)
}

func (f *RepositoryFactory) FLRoundRepository() ports.FLRoundRepository {
	return repositories.NewFLRoundRepository(f.db)
}

func (f *RepositoryFactory) FLParticipantRepository() ports.FLParticipantRepository {
	return repositories.NewFLParticipantRepository(f.db)
}

var repositoryFactory *RepositoryFactory

func InitRepositoryFactory(db *gorm.DB) {
	repositoryFactory = NewRepositoryFactory(db)
}

func GetRepositoryFactory() *RepositoryFactory {
	if repositoryFactory == nil {
		dbManager := GetDBManager()
		repositoryFactory = NewRepositoryFactoryFromManager(dbManager)
	}
	return repositoryFactory
}
