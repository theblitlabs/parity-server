package db

import (
	"context"
	"fmt"
	"sync"

	"github.com/theblitlabs/parity-server/internal/core/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DBManager provides centralized database connection management
type DBManager struct {
	db   *gorm.DB
	lock sync.RWMutex
}

func NewDBManager() *DBManager {
	return &DBManager{}
}

func (m *DBManager) Connect(ctx context.Context, dbURL string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("error opening database: %w", err)
	}

	if err := db.AutoMigrate(&models.Task{}, &models.TaskResult{}, &models.Runner{}); err != nil {
		return fmt.Errorf("error migrating database: %w", err)
	}

	var count int64
	if err := db.Model(&models.Runner{}).Where("last_heartbeat IS NULL").Count(&count).Error; err != nil {
		return fmt.Errorf("error checking for null LastHeartbeat: %w", err)
	}

	if count > 0 {
		if err := db.Model(&models.Runner{}).Where("last_heartbeat IS NULL").Update("last_heartbeat", gorm.Expr("NOW()")).Error; err != nil {
			return fmt.Errorf("error updating null LastHeartbeat values: %w", err)
		}
	}

	m.db = db
	return nil
}

func (m *DBManager) GetDB() *gorm.DB {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.db
}

func (m *DBManager) Close() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.db == nil {
		return nil
	}

	sqlDB, err := m.db.DB()
	if err != nil {
		return fmt.Errorf("error getting SQL DB: %w", err)
	}

	return sqlDB.Close()
}

var (
	instance *DBManager
	once     sync.Once
)

func GetDBManager() *DBManager {
	once.Do(func() {
		instance = NewDBManager()
	})
	return instance
}

func Connect(ctx context.Context, dbURL string) (*gorm.DB, error) {
	dbManager := GetDBManager()
	err := dbManager.Connect(ctx, dbURL)
	if err != nil {
		return nil, err
	}
	return dbManager.GetDB(), nil
}
