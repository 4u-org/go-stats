package database

import "time"

type Bot struct {
	ID        int64   `gorm:"primaryKey"`
	TokenHash *[]byte `gorm:"type:bytea"`
	App       *string `gorm:"size:64"`
	LoggedIn  bool    `gorm:"default:false"`
}

func (b *Bot) TableName() string {
	return "bots"
}

type User struct {
	ID               int64     `gorm:"primaryKey"`
	BotID            int64     `gorm:"index:idx_bot_user,unique"`
	UserID           int64     `gorm:"index:idx_bot_user,unique"`
	FirstActionTime  time.Time `gorm:"autoCreateTime"`
	LastActionTime   time.Time `gorm:"autoCreateTime"`
	RefererID        string    `gorm:"size:64;default:''"`
	SessionID        int16     `gorm:"default:0"`
	SessionRefererID string    `gorm:"size:64;default:''"`
	Bot              Bot       `gorm:"foreignKey:BotID"`
}

func (u *User) TableName() string {
	return "users"
}

type Chat struct {
	ID              int64     `gorm:"primaryKey"`
	BotID           int64     `gorm:"index:idx_bot_chat,unique"`
	ChatID          int64     `gorm:"index:idx_bot_chat,unique"`
	MigratedFrom    int64     `gorm:"default:0"`
	FirstActionTime time.Time `gorm:"autoCreateTime"`
	LastActionTime  time.Time `gorm:"autoCreateTime"`
	LastUpdateTime  time.Time `gorm:"autoCreateTime"`
	RefererID       string    `gorm:"size:64;default:''"`
	CanWrite        bool      `gorm:"default:false"`
	WasBanned       bool      `gorm:"default:false"`
	Bot             Bot       `gorm:"foreignKey:BotID"`
}

func (c *Chat) TableName() string {
	return "chats"
}

type ChatMember struct {
	ID               int64      `gorm:"primaryKey"`
	ChatID           int64      `gorm:"index:idx_chat_member,unique"`
	UserID           int64      `gorm:"index:idx_chat_member,unique"`
	FirstJoinTime    *time.Time `gorm:"default:null"`
	LastJoinTime     *time.Time `gorm:"default:null"`
	LastLeaveTime    *time.Time `gorm:"default:null"`
	LastActionTime   time.Time  `gorm:"autoCreateTime"`
	IsMember         bool       `gorm:"default:true"`
	FirstJoinActorId int64      `gorm:"default:0"`
	LastJoinActorId  int64      `gorm:"default:0"`
	LastLeaveActorId int64      `gorm:"default:0"`
	JoinUrl          string     `gorm:"size:64;default:''"`
}

func (c *ChatMember) TableName() string {
	return "chatmembers"
}

type TgUser struct {
	UserID       int64     `gorm:"primaryKey"`
	FirstName    string    `gorm:"size:64"`
	LastName     string    `gorm:"size:64;default:null"`
	Username     string    `gorm:"size:32;index;default:null"`
	LanguageCode string    `gorm:"size:2;default:null"`
	IsPremium    bool      `gorm:"default:false"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

func (u *TgUser) TableName() string {
	return "tgusers"
}

type Event struct {
	App                string     `gorm:"type:lowcardinality;not null"`
	BotID              int64      `gorm:"not null"`
	EventType          string     `gorm:"type:lowcardinality;not null"`
	EventSubtype       string     `gorm:"type:lowcardinality;default:''"`
	FromBot            bool       `gorm:"default:false"`
	Data               []string   `gorm:"type:Array(String)"`
	DataLowCardinality []string   `gorm:"type:Array(LowCardinality(String))"`
	DataInt            []int64    `gorm:"type:Array(Int64)"`
	DataFlags          []bool     `gorm:"type:Array(Bool)"`
	ChatID             int64      `gorm:"default:0"`
	UserID             int64      `gorm:"default:0"`
	SessionID          int16      `gorm:"type:Int16;default:-1"`
	ContentID          string     `gorm:"type:lowcardinality;default:''"`
	Language           string     `gorm:"type:lowcardinality;default:''"`
	UserCreatedAt      *time.Time `gorm:"type:DateTime('UTC');null"`
	Referer            string     `gorm:"default:''"`
	SessionReferer     string     `gorm:"default:''"`
	ContentReferer     string     `gorm:"default:''"`
	AbMask             []string   `gorm:"type:Array(LowCardinality(String))"`
	Timestamp          time.Time  `gorm:"type:DateTime('UTC');default:now();not null"`
}

func (e *Event) TableName() string {
	return "bots.eventsgo"
}
