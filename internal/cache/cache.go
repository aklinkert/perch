package cache

import (
	"errors"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type APICache struct {
	Instance  string `gorm:"primaryKey;size:64"`
	Key       string `gorm:"primaryKey;size:512"`
	ETag      string
	Body      []byte
	FetchedAt time.Time
	TTL       time.Duration
}

type WidgetSnapshot struct {
	WidgetID  string `gorm:"primaryKey;size:128"`
	Payload   []byte
	UpdatedAt time.Time
}

type LayoutState struct {
	ID        string `gorm:"primaryKey;size:32"`
	Payload   []byte
	UpdatedAt time.Time
}

type Store struct {
	db *gorm.DB
}

func Open(path string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(path+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&APICache{}, &WidgetSnapshot{}, &LayoutState{}); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) GetAPI(instance, key string) (*APICache, error) {
	var c APICache
	err := s.db.Where("instance = ? AND key = ?", instance, key).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) PutAPI(c *APICache) error {
	return s.db.Save(c).Error
}

func (s *Store) GetSnapshot(widgetID string) (*WidgetSnapshot, error) {
	var ws WidgetSnapshot
	err := s.db.Where("widget_id = ?", widgetID).First(&ws).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ws, nil
}

func (s *Store) PutSnapshot(ws *WidgetSnapshot) error {
	ws.UpdatedAt = time.Now()
	return s.db.Save(ws).Error
}

const layoutStateID = "default"

func (s *Store) GetLayout() (*LayoutState, error) {
	var ls LayoutState
	err := s.db.Where("id = ?", layoutStateID).First(&ls).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ls, nil
}

func (s *Store) PutLayout(payload []byte) error {
	return s.db.Save(&LayoutState{
		ID:        layoutStateID,
		Payload:   payload,
		UpdatedAt: time.Now(),
	}).Error
}
