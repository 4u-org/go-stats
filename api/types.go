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
	Source    string `form:"source"`
	App       string `form:"app"`
	Token     string `form:"token"`
	ForceAuth bool   `form:"force_auth"`
}

type GetBotQuery struct {
	gnext.Query
	BotID  int64  `form:"bot_id"`
	Source string `form:"source"`
}

type InsertUsersQuery struct {
	gnext.Body
	BotID      int64 `json:"bot_id"`
	ForceCheck bool  `json:"force_check"`
	Users      []int `json:"user_ids"`
}
