package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"pr5_minenko/internal/config"
	"pr5_minenko/internal/model"
)

func Connect(cfg config.Config) (*gorm.DB, error) {
	gormLogger := logger.New(log.New(os.Stdout, "\n", log.LstdFlags), logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
		Colorful:                  false,
	})

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger:         gormLogger,
		TranslateError: true,
		NowFunc:        func() time.Time { return time.Now().UTC().Truncate(time.Microsecond) },
	})
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	log.Printf("[GORM] Connected to PostgreSQL successfully (host=%s:%s, db=%s)", cfg.DBHost, cfg.DBPort, cfg.DBName)
	return db, nil
}

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&model.User{}, &model.Incident{}); err != nil {
		return err
	}
	log.Println("[GORM] Auto-migration completed: 'users', 'incidents' tables are up to date.")
	return nil
}
