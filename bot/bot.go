package bot

import (
	"context"
	"crypto/sha256"
	"go-stats/database"
	"os"
	"strconv"
	"strings"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/updates"
	updhook "github.com/gotd/td/telegram/updates/hook"
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
		if err := db.Create(&bot).Error; err != nil {
			return errors.Wrap(err, "Failed to add bot to db")
		}
		return nil
	}
	bot.App = app
	bot.TokenHash = tokenHash
	bot.LoggedIn = loggedIn
	if err := tx.Save(&bot).Error; err != nil {
		return errors.Wrap(err, "Failed to update bot in db")
	}
	return nil
}

func LoginBot(
	ctx context.Context,
	apiID int,
	apiHash string,
	token string,
	log *zap.Logger,
) error {
	botId := strings.Split(token, ":")[0]
	session := session.FileStorage{Path: "sessions/session_" + botId}

	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		Logger:         log,
		SessionStorage: &session,
	})

	return client.Run(ctx, func(ctx context.Context) error {
		// Check auth status.
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return errors.Wrap(err, "Failed to get auth status")
		}

		// Already authorized.
		if status.Authorized {
			return nil
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

func RunBot(
	ctx context.Context,
	stateDb *bolt.DB,
	apiID int,
	apiHash string,
	botId int64,
	db *gorm.DB,
	clickCh chan database.Event,
	log *zap.Logger,
	forget bool,
) error {
	bot := database.Bot{ID: botId}
	if errBot := db.First(&bot).Error; errBot != nil {
		return errors.Wrap(errBot, "Bot not found")
	}

	// session := session.FileStorage{Path: "sessions/session_" + strconv.FormatInt(botId, 10)}
	session := NewBoltStorage(stateDb, botId)
	storage := NewBoltState(stateDb)
	accessHasher := NewBoltAccessHasher(stateDb)
	handler := NewUpdateDispatcher(botId, bot.App, db, clickCh, log)

	gaps := updates.New(updates.Config{
		Storage:      storage,
		AccessHasher: accessHasher,
		Handler:      handler, //handler,
		Logger:       log.Named(strconv.FormatInt(botId, 10)),
	})

	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		Logger:         log,
		SessionStorage: session,
		UpdateHandler:  gaps,
		Middlewares: []telegram.Middleware{
			updhook.UpdateHook(gaps.Handle),
		},
	})

	handler.addApi(client.API())

	return client.Run(ctx, func(ctx context.Context) error {
		// Check auth status.
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return errors.Wrap(err, "Failed to get auth status")
		}

		if !status.Authorized {
			if err := db.Where("id = ?", botId).Updates(database.Bot{LoggedIn: false}).Error; err != nil {
				log.Error("Failed to update bot in db", zap.Error(err))
			}
			return errors.New("Bot not authorized. Use LoginBot method")
		}

		log.Info("Bot login restored", zap.String("name", status.User.Username))

		// Notify update manager about authentication.
		return gaps.Run(ctx, client.API(), status.User.ID, updates.AuthOptions{
			IsBot:  status.User.Bot,
			Forget: forget,
			OnStart: func(ctx context.Context) {
				log.Info("Gaps started")
			},
		})
	})
}