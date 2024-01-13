package api

import (
	"fmt"
	"go-stats/bot"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/meteran/gnext"
	"go.uber.org/zap"
)

func (a *Api) ping(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "pong",
	})
}

func (a *Api) getBot(q *GetBotQuery) (*BotResponse, gnext.Status) {
	// Extract bot id from token
	bot, err := bot.GetFromDb(a.db, &q.Source, q.BotID)

	if err != nil {
		return &BotResponse{
			Ok:      false,
			Message: fmt.Sprintf("Could not get info: %s", err),
		}, http.StatusBadRequest
	}

	return &BotResponse{Ok: true, App: *bot.App, LoggedIn: bot.LoggedIn}, http.StatusOK
}

func (a *Api) addBot(q *Bot) (*Response, gnext.Status) {
	// Extract bot id from token
	botId := strings.Split(q.Token, ":")[0]
	botIdInt, err := strconv.ParseInt(botId, 10, 64)
	if err != nil {
		return &Response{
			Ok:      false,
			Message: "Invalid token: bot id is not int",
		}, http.StatusBadRequest
	}

	// Login bot
	if err := bot.LoginBot(a.ctx, a.boltDb, a.apiID, a.apiHash, q.Token, a.log, q.ForceAuth); err != nil {
		a.log.Info("Error logging in bot", zap.Error(err))
		return &Response{
			Ok:      false,
			Message: fmt.Sprintf("Error logging in bot: %s", err),
		}, http.StatusBadRequest
	}

	// Get app name
	app := &q.App
	// Hash bot token
	tokenHash := bot.HashToken(q.Token)

	// Add bot to database
	if err := bot.UpdateDb(a.db, &q.Source, botIdInt, app, tokenHash, true); err != nil {
		a.log.Info("Error adding bot to database", zap.Error(err))
		return &Response{
			Ok:      false,
			Message: fmt.Sprintf("Error adding bot to database: %s", err),
		}, http.StatusBadRequest
	}

	// Start bot
	go bot.RunBot(a.ctx, a.boltDb, a.apiID, a.apiHash, botIdInt, a.db, a.clickCh, a.log, true)

	return &Response{Ok: true}, http.StatusOK
}
