package api

import (
	"context"
	"errors"
	"go-stats/bot"
	"go-stats/database"
	"os"

	"github.com/meteran/gnext"
	"github.com/meteran/gnext/docs"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Api struct {
	ctx               context.Context
	boltDb            *bolt.DB
	apiID             int
	apiHash           string
	db                *gorm.DB
	clickCh           chan *database.Event
	log               *zap.Logger
	botConnectionPool *bot.ConnectionPool
}

func NewApi(
	ctx context.Context,
	boltDb *bolt.DB,
	apiID int,
	apiHash string,
	db *gorm.DB,
	clickCh chan *database.Event,
	log *zap.Logger,
	botConnectionPool *bot.ConnectionPool,
) Api {
	return Api{
		ctx:               ctx,
		boltDb:            boltDb,
		apiID:             apiID,
		apiHash:           apiHash,
		db:                db,
		clickCh:           clickCh,
		log:               log,
		botConnectionPool: botConnectionPool,
	}
}

func Start(
	ctx context.Context,
	boltDb *bolt.DB,
	apiID int,
	apiHash string,
	db *gorm.DB,
	clickCh chan *database.Event,
	log *zap.Logger,
	botConnectionPool *bot.ConnectionPool,
) error {
	r := gnext.Router(&docs.Options{Servers: []string{}})
	apiLog := log.Named("api")
	api := NewApi(ctx, boltDb, apiID, apiHash, db, clickCh, apiLog, botConnectionPool)

	r.GET("/ping", api.ping)
	r.GET("/add_bot", api.addBot)
	r.GET("/get_bot", api.getBot)
	r.POST("/insert_users", api.insertUsers)

	host := os.Getenv("API_HOST")
	if host == "" {
		return errors.New("API_HOST env variable is empty")
	}

	r.Run(host) // listen and serve on 0.0.0.0:8080
	return nil
}
