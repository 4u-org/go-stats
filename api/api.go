package api

import (
	"context"
	"errors"
	"go-stats/database"
	"os"

	"github.com/meteran/gnext"
	"github.com/meteran/gnext/docs"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Api struct {
	ctx     context.Context
	boltDb  *bolt.DB
	apiID   int
	apiHash string
	db      *gorm.DB
	clickCh chan *database.Event
	log     *zap.Logger
}

func NewApi(
	ctx context.Context,
	boltDb *bolt.DB,
	apiID int,
	apiHash string,
	db *gorm.DB,
	clickCh chan *database.Event,
	log *zap.Logger,
) Api {
	return Api{
		ctx:     ctx,
		boltDb:  boltDb,
		apiID:   apiID,
		apiHash: apiHash,
		db:      db,
		clickCh: clickCh,
		log:     log,
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
) error {
	r := gnext.Router(&docs.Options{Servers: []string{}})
	apiLog := log.Named("api")
	api := NewApi(ctx, boltDb, apiID, apiHash, db, clickCh, apiLog)

	r.GET("/ping", api.ping)
	r.GET("/add_bot", api.addBot)

	host := os.Getenv("API_HOST")
	if host == "" {
		return errors.New("API_HOST env variable is empty")
	}

	r.Run(host) // listen and serve on 0.0.0.0:8080
	return nil
}
