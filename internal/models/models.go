package models

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	GitLabID  int `gorm:"uniqueIndex"`
	Username  string
	Email     string
	AvatarURL string
}

type ScanHistory struct {
	gorm.Model
	UserID     uint
	GroupID    string
	Keywords   string
	Branch     string
	MatchCount int
}

type ScanJob struct {
	gorm.Model
	Status       string `gorm:"default:'pending'"` // pending, running, completed, failed
	Results      string `gorm:"type:text"`         // JSON serialized results
	ErrorMessage string `gorm:"type:text"`
	GroupID      string
}
