package api

import "github.com/meteran/gnext"

type Response struct {
	Ok      bool   `json:"ok"`
	Message string `json:"message"`
}

type Bot struct {
	gnext.Query
	App   string `form:"app"`
	Token string `form:"token"`
}
