package bot

import (
	"context"
	"crypto/sha256"
	"go-stats/database"
	"os"
	"strconv"
	"strings"

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

func RunBot(
	ctx context.Context,
	boltDb *bolt.DB,
	apiID int,
	apiHash string,
	botId int64,
	db *gorm.DB,
	clickCh chan *database.Event,
	log *zap.Logger,
	forget bool,
) error {
	bot := database.Bot{ID: botId}
	if errBot := db.First(&bot).Error; errBot != nil {
		return errors.Wrap(errBot, "Bot not found")
	}

	namedLog := log.Named(strconv.FormatInt(botId, 10))
	if botId != 1264915325 {
		namedLog = namedLog.WithOptions(zap.IncreaseLevel(zap.WarnLevel))
		return nil
	}

	// session := session.FileStorage{Path: "sessions/session_" + strconv.FormatInt(botId, 10)}
	session := NewBoltSessionStorage(boltDb, botId)
	storage := NewBoltState(boltDb)
	accessHasher := NewBoltAccessHasher(boltDb)
	handler := NewUpdateDispatcher(botId, bot.App, db, clickCh, namedLog.WithOptions(zap.IncreaseLevel(zap.WarnLevel)))

	gaps := updates.New(updates.Config{
		Storage:      storage,
		AccessHasher: accessHasher,
		Handler:      handler, //handler,
		Logger:       namedLog,
	})

	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		Logger:         namedLog,
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
				namedLog.Error("Failed to update bot in db", zap.Error(err))
			}
			return errors.New("Bot not authorized. Use LoginBot method")
		}

		namedLog.Info("Bot login restored", zap.String("name", status.User.Username))

		// Notify update manager about authentication.
		return gaps.Run(ctx, client.API(), status.User.ID, updates.AuthOptions{
			IsBot:  status.User.Bot,
			Forget: forget,
			OnStart: func(ctx context.Context) {
				namedLog.Info("Gaps started")
			},
		})
	})
}
