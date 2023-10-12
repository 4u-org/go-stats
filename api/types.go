package api

import "github.com/meteran/gnext"

type Response struct {
	Ok      bool   `json:"ok"`
	Message string `json:"message"`
}

type BotResponse struct {
	Ok       bool   `json:"ok"`
	Message  string `json:"message"`
	App      string `json:"app"`
	LoggedIn bool   `json:"logged_in"`
}

type Bot struct {
	gnext.Query
	App       string `form:"app"`
	Token     string `form:"token"`
	ForceAuth bool   `form:"force_auth"`
}

type GetBotQuery struct {
	gnext.Query
	BotID int64 `form:"bot_id"`
}
