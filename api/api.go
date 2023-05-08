package api

import (
	"context"
	"go-stats/database"

	"github.com/meteran/gnext"
	"github.com/meteran/gnext/docs"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Api struct {
	ctx     context.Context
	stateDb *bolt.DB
	apiID   int
	apiHash string
	db      *gorm.DB
	clickCh chan database.Event
	log     *zap.Logger
}

func NewApi(
	ctx context.Context,
	stateDb *bolt.DB,
	apiID int,
	apiHash string,
	db *gorm.DB,
	clickCh chan database.Event,
	log *zap.Logger,
) Api {
	return Api{
		ctx:     ctx,
		stateDb: stateDb,
		apiID:   apiID,
		apiHash: apiHash,
		db:      db,
		clickCh: clickCh,
		log:     log,
	}
}

func Start(
	ctx context.Context,
	stateDb *bolt.DB,
	apiID int,
	apiHash string,
	db *gorm.DB,
	clickCh chan database.Event,
	log *zap.Logger,
) error {
	r := gnext.Router(&docs.Options{Servers: []string{}})
	apiLog := log.Named("api")
	api := NewApi(ctx, stateDb, apiID, apiHash, db, clickCh, apiLog)

	r.GET("/ping", api.ping)
	r.GET("/add_bot", api.addBot)
	r.Run("127.0.0.1", "8008") // listen and serve on 0.0.0.0:8080
	return nil
}
