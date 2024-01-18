package bot

import (
	"context"
	"go-stats/database"
	"strconv"

	"go-stats/updates"
	updhook "go-stats/updates/hook"

	"github.com/gotd/td/telegram"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ConnectionPool struct {
	ctx     context.Context
	stateDB *bolt.DB
	apiID   int
	apiHash string
	db      *gorm.DB
	clickCH chan *database.Event
	log     *zap.Logger
	bots    map[int64]*TgBot
}

func NewConnectionPool(
	ctx context.Context,
	stateDB *bolt.DB,
	apiID int,
	apiHash string,
	db *gorm.DB,
	clickCH chan *database.Event,
	log *zap.Logger,
) ConnectionPool {
	return ConnectionPool{
		ctx:     ctx,
		stateDB: stateDB,
		apiID:   apiID,
		apiHash: apiHash,
		db:      db,
		clickCH: clickCH,
		log:     log,
		bots:    make(map[int64]*TgBot),
	}
}

func (c *ConnectionPool) AddBot(botID int64) error {
	bot := database.Bot{ID: botID}
	if errBot := c.db.First(&bot).Error; errBot != nil {
		return errors.Wrap(errBot, "Bot not found")
	}

	namedLog := c.log.Named(strconv.FormatInt(botID, 10))
	if botID != 1264915325 {
		namedLog = namedLog.WithOptions(zap.IncreaseLevel(zap.WarnLevel))
	}

	// session := session.FileStorage{Path: "sessions/session_" + strconv.FormatInt(botId, 10)}
	session := NewBoltSessionStorage(c.stateDB, botID)
	// storage := NewBoltState(stateDB)
	accessHasher := NewBoltAccessHasher(c.stateDB)
	handler := NewUpdateDispatcher(botID, bot.Source, bot.App, c.db, c.clickCH, namedLog.WithOptions(zap.IncreaseLevel(zap.WarnLevel)))

	gaps := updates.New(updates.Config{
		// Storage:      storage,
		AccessHasher: accessHasher,
		Handler:      handler, //handler,
		Logger:       namedLog,
	})

	client := telegram.NewClient(c.apiID, c.apiHash, telegram.Options{
		Logger:         namedLog,
		SessionStorage: session,
		UpdateHandler:  gaps,
		Middlewares: []telegram.Middleware{
			updhook.UpdateHook(gaps.Handle),
		},
	})

	handler.addApi(client.API())

	c.bots[botID] = NewTgBot(c.ctx, client, gaps, botID, c.db, namedLog)
	return nil
}

func (c *ConnectionPool) RunBot(botID int64, forget bool) error {
	bot, ok := c.bots[botID]
	if !ok {
		return errors.New("Bot not found")
	}
	return bot.Run(forget)
}

func (c *ConnectionPool) StopBot(botID int64) error {
	bot, ok := c.bots[botID]
	if !ok {
		return errors.New("Bot not found")
	}
	bot.Stop()
	return nil
}

func (c *ConnectionPool) GetApi(botID int64) (*telegram.Client, error) {
	bot, ok := c.bots[botID]
	if !ok {
		return nil, errors.New("Bot not found")
	}
	return bot.client, nil
}
