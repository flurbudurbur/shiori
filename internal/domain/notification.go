package domain

import (
	"context"
	"time"
)

type NotificationRepo interface {
	List(ctx context.Context) ([]Notification, error)
	Find(ctx context.Context, params NotificationQueryParams) ([]Notification, int, error)
	FindByID(ctx context.Context, id int) (*Notification, error)
	Store(ctx context.Context, notification Notification) (*Notification, error)
	Update(ctx context.Context, notification Notification) (*Notification, error)
	Delete(ctx context.Context, notificationID int) error
}

type NotificationSender interface {
	Send(event NotificationEvent, payload NotificationPayload) error
	CanSend(event NotificationEvent) bool
}

// Notification represents a configured notification channel.
type Notification struct {
	ID             int              `json:"id" gorm:"primaryKey;autoIncrement;column:id"`
	UserHashedUUID string           `json:"user_hashed_uuid" gorm:"column:user_hashed_uuid;index"` // Foreign key to User, indexed
	Name           string           `json:"name" gorm:"column:name"`
	Type           NotificationType `json:"type" gorm:"column:type"`
	Enabled        bool             `json:"enabled" gorm:"column:enabled"`
	Events         []string         `json:"events" gorm:"column:events;type:text;serializer:json"` // Store as JSON text
	Token          string           `json:"token" gorm:"column:token"`
	Webhook        string           `json:"webhook" gorm:"column:webhook"`
	Title          string           `json:"title" gorm:"column:title"`
	Icon           string           `json:"icon" gorm:"column:icon"`
	Username       string           `json:"username" gorm:"column:username"`
	Host           string           `json:"host" gorm:"column:host"`
	Password       string           `json:"password" gorm:"column:password"`
	Channel        string           `json:"channel" gorm:"column:channel"`
	Rooms          string           `json:"rooms" gorm:"column:rooms"`
	Targets        string           `json:"targets" gorm:"column:targets"`
	Devices        string           `json:"devices" gorm:"column:devices"`
	CreatedAt      time.Time        `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time        `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
	User           User             `json:"-" gorm:"foreignKey:UserHashedUUID;references:HashedUUID"` // Foreign key to User
}

type NotificationPayload struct {
	Subject   string
	Message   string
	Event     NotificationEvent
	Timestamp time.Time
}

type NotificationType string

const (
	NotificationTypeDiscord    NotificationType = "DISCORD"
	NotificationTypeNotifiarr  NotificationType = "NOTIFIARR"
	NotificationTypeIFTTT      NotificationType = "IFTTT"
	NotificationTypeJoin       NotificationType = "JOIN"
	NotificationTypeMattermost NotificationType = "MATTERMOST"
	NotificationTypeMatrix     NotificationType = "MATRIX"
	NotificationTypePushBullet NotificationType = "PUSH_BULLET"
	NotificationTypePushover   NotificationType = "PUSHOVER"
	NotificationTypeRocketChat NotificationType = "ROCKETCHAT"
	NotificationTypeSlack      NotificationType = "SLACK"
	NotificationTypeTelegram   NotificationType = "TELEGRAM"
)

type NotificationEvent string

const (
	NotificationEventAppUpdateAvailable NotificationEvent = "SERVER_UPDATE_AVAILABLE"
	NotificationEventSyncStarted        NotificationEvent = "SYNC_STARTED"
	NotificationEventSyncSuccess        NotificationEvent = "SYNC_SUCCESS"
	NotificationEventSyncFailed         NotificationEvent = "SYNC_FAILED"
	NotificationEventSyncError          NotificationEvent = "SYNC_ERROR"
	NotificationEventTest               NotificationEvent = "TEST"
)

type NotificationEventArr []NotificationEvent

type NotificationQueryParams struct {
	UserHashedUUID *string // Added to filter by user
	Limit          uint64
	Offset         uint64
	Sort           map[string]string
	Filters        struct {
		Indexers   []string
		PushStatus string
	}
	Search string
}
