package main

import (
	"context"
	"fmt"
	"go-stats/api"
	"go-stats/bot"
	"go-stats/database"
	"io/fs"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/go-faster/errors"
	"github.com/joho/godotenv"
	"github.com/sasha-s/go-deadlock"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func writeEvents(ctx context.Context, conn driver.Conn, clickCh chan *database.Event, close chan int, log *zap.Logger) {
	query := "INSERT INTO " + (&database.Event{}).TableName()
	tick := time.Tick(time.Second * 5)
	batch, err := conn.PrepareBatch(context.TODO(), query)
	if err != nil {
		log.Error("Error preparing batch", zap.Error(err))
	}

	for {
		select {
		case event := <-clickCh:
			// Append the event to the batch
			err := batch.AppendStruct(event)
			if err != nil {
				log.Error("Error appending event to batch", zap.Error(err))
			}
		case <-tick:
			// Send the batch and prepare a new one
			err := batch.Flush()
			if err != nil {
				log.Error("Error writing events", zap.Error(err))
			}
		case <-close:
			// Send the batch, close the connection and return
			err := batch.Flush()
			if err != nil {
				log.Error("Error writing events", zap.Error(err))
			}
			err = conn.Close()
			if err != nil {
				log.Error("Error closing clickhouse connection", zap.Error(err))
			}
			return
		}
	}
}

func main() {
	godotenv.Load()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx); err != nil {
		panic(err)
	}
}

func run(ctx context.Context) error {
	// Create a new logger
	log, _ := zap.NewDevelopment(
		zap.IncreaseLevel(zapcore.WarnLevel),
		zap.AddStacktrace(zapcore.FatalLevel),
	)
	defer func() { _ = log.Sync() }()

	deadlock.Opts.Disable = false
	deadlock.Opts.DisableLockOrderDetection = false
	deadlock.Opts.LogBuf = os.Stdout
	deadlock.Opts.OnPotentialDeadlock = func() {
		log.Error("Potential deadlock detected")
	}
	deadlock.Opts.PrintAllCurrentGoroutines = true

	// Open the postgres database
	db, err := gorm.Open(postgres.Open(os.Getenv("POSTGRES_DSN")), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return errors.Wrap(err, "Error connecting to db")
	}
	err = db.AutoMigrate(&database.Bot{}, &database.User{}, &database.Chat{}, &database.ChatMember{}, &database.TgUser{})
	if err != nil {
		return errors.Wrap(err, "Error migrating db")
	}
	postgresDb, err := db.DB()
	if err != nil {
		return errors.Wrap(err, "Error getting postgres db")
	}
	postgresDb.SetMaxIdleConns(10)
	postgresDb.SetMaxOpenConns(500)
	defer postgresDb.Close()

	// Open the clickhouse database
	clickOptions, err := clickhouse.ParseDSN(os.Getenv("CLICKHOUSE_DSN"))
	if err != nil {
		return errors.Wrap(err, "Error parsing clickhouse DSN")
	}
	clickDb, err := clickhouse.Open(clickOptions)
	if err != nil {
		return errors.Wrap(err, "Error connecting to clickhouse")
	}
	clickCh := make(chan *database.Event, 1000)
	clickClose := make(chan int)
	defer func() {
		time.Sleep(time.Second)
		clickClose <- 1
	}()
	go writeEvents(ctx, clickDb, clickCh, clickClose, log)

	// Get the API ID
	apiID, err := strconv.Atoi(os.Getenv("APP_ID"))
	if err != nil {
		return errors.Wrap(err, "APP_ID not set or invalid")
	}

	// Get the API hash
	apiHash := os.Getenv("APP_HASH")
	if apiHash == "" {
		return errors.New("no APP_HASH provided")
	}

	// Get token salt
	tokenSalt := os.Getenv("BOT_TOKEN_SALT")
	if tokenSalt == "" {
		log.Warn("TOKEN_SALT not set, using empty string")
	}

	// Open the state database
	stateDb, err := bolt.Open("storage/db.bbolt", fs.ModePerm, bolt.DefaultOptions)
	if err != nil {
		return errors.Wrap(err, "state database")
	}
	defer stateDb.Close()

	// Get the bot IDs
	botIDs := []int64{}
	botQuery := &database.Bot{LoggedIn: true}
	err = db.Model(&botQuery).Where(&botQuery).Pluck("id", &botIDs).Error
	if err != nil {
		return errors.Wrap(err, "Error getting bot IDs")
	}

	// Run each bot in a separate goroutine
	for _, botID := range botIDs {
		go func(id int64) {
			if err := bot.RunBot(ctx, stateDb, apiID, apiHash, id, db, clickCh, log, false); err != nil {
				log.Error(fmt.Sprintf("Error running bot %d: %v\n", id, err))
			}
		}(botID)
		time.Sleep(time.Millisecond * 3)
	}

	// Run the API
	go api.Start(ctx, stateDb, apiID, apiHash, db, clickCh, log)

	// Wait for all bots to finish processing updates
	<-ctx.Done()
	return nil
}
