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
