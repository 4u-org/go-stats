package bot

import (
	"context"
	"crypto/sha256"
	"go-stats/database"
	"os"
	"strconv"
	"strings"

	"go-stats/updates"

	"github.com/gotd/td/telegram"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func HashToken(token string) *[]byte {
	tokenSalt := os.Getenv("BOT_TOKEN_SALT")
	hash := sha256.Sum256([]byte(token + tokenSalt))
	hashSlice := hash[:]
	return &hashSlice
}

func UpdateDb(
	db *gorm.DB,
	source *string,
	botId int64,
	app *string,
	tokenHash *[]byte,
	loggedIn bool,
) error {
	bot := database.Bot{ID: botId}
	tx := db.Where(&bot).First(&bot)
	if tx.Error != nil && tx.Error != gorm.ErrRecordNotFound {
		return errors.Wrap(tx.Error, "Failed to add bot to db")
	}
	if tx.Error == gorm.ErrRecordNotFound {
		bot.App = app
		bot.TokenHash = tokenHash
		bot.LoggedIn = loggedIn
		bot.Source = source
		if err := db.Create(&bot).Error; err != nil {
			return errors.Wrap(err, "Failed to add bot to db")
		}
		return nil
	}
	bot.App = app
	bot.TokenHash = tokenHash
	bot.LoggedIn = loggedIn
	bot.Source = source
	if err := tx.Save(&bot).Error; err != nil {
		return errors.Wrap(err, "Failed to update bot in db")
	}
	return nil
}

func GetFromDb(
	db *gorm.DB,
	source *string,
	botID int64,
) (*database.Bot, error) {
	bot := database.Bot{ID: botID, Source: source}
	tx := db.Where(&bot).First(&bot)
	if tx.Error != nil && tx.Error != gorm.ErrRecordNotFound {
		return nil, errors.Wrap(tx.Error, "Failed to get info")
	}
	if tx.Error == gorm.ErrRecordNotFound {
		return nil, errors.Wrap(tx.Error, "Could not find info about bot in db")
	}
	return &bot, nil
}

func LoginBot(
	ctx context.Context,
	boltDb *bolt.DB,
	apiID int,
	apiHash string,
	token string,
	log *zap.Logger,
	forceAuth bool,
) error {
	botId := strings.Split(token, ":")[0]
	botIdInt, err := strconv.ParseInt(botId, 10, 64)
	if err != nil {
		return errors.Wrap(err, "Invalid token: bot id is not int")
	}

	session := NewBoltSessionStorage(boltDb, botIdInt)

	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		Logger:         log,
		SessionStorage: session,
	})

	return client.Run(ctx, func(ctx context.Context) error {
		// Check auth status.
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return errors.Wrap(err, "Failed to get auth status")
		}

		// Already authorized.
		if status.Authorized {
			if !forceAuth {
				return nil
			}
			log.Warn("Already authorized")
		}

		// Login.
		if _, err := client.Auth().Bot(ctx, token); err != nil {
			return errors.Wrap(err, "failed to login")
		}

		// Refresh auth status.
		status, err = client.Auth().Status(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get auth status after login")
		}
		if !status.Authorized {
			return errors.New("not authorized after login")
		}

		// Login success.
		return nil
	})
}

type TgBot struct {
	ctx      context.Context
	cancel   context.CancelFunc
	client   *telegram.Client
	gaps     *updates.Manager
	botID    int64
	db       *gorm.DB
	namedLog *zap.Logger
}

func NewTgBot(
	ctx context.Context,
	client *telegram.Client,
	gaps *updates.Manager,
	botID int64,
	db *gorm.DB,
	namedLog *zap.Logger,
) *TgBot {
	return &TgBot{
		ctx:      ctx,
		client:   client,
		gaps:     gaps,
		botID:    botID,
		db:       db,
		namedLog: namedLog,
	}
}

func (b *TgBot) Run(forget bool) error {
	ctx, cancel := context.WithCancel(b.ctx)
	b.cancel = cancel

	return b.client.Run(ctx, func(ctx context.Context) error {
		// Check auth status.
		status, err := b.client.Auth().Status(ctx)
		if err != nil {
			return errors.Wrap(err, "Failed to get auth status")
		}

		if !status.Authorized {
			if err := b.db.Where("id = ?", b.botID).Updates(database.Bot{LoggedIn: false}).Error; err != nil {
				b.namedLog.Error("Failed to update bot in db", zap.Error(err))
			}
			return errors.New("Bot not authorized. Use LoginBot method")
		}

		b.namedLog.Info("Bot login restored", zap.String("name", status.User.Username))

		// Notify update manager about authentication.
		return b.gaps.Run(ctx, b.client.API(), status.User.ID, updates.AuthOptions{
			IsBot:  status.User.Bot,
			Forget: forget,
			OnStart: func(ctx context.Context) {
				b.namedLog.Info("Gaps started")
			},
		})
	})
}

func (b *TgBot) Stop() {
	b.cancel()
}
