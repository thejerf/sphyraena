package main

import (
	"fmt"

	"github.com/thejerf/sphyraena/context"
	"github.com/thejerf/sphyraena/sphyrw"
)

// MainPage is a struct that happens to fill out the main.tmpl.
type MainPage struct {
	Title string
}

type LoginHint struct {
	Username string
	Password string
	Title    string
}

func Login(rw *sphyrw.SphyraenaResponseWriter, ctx *context.Context) {
	err := templates.ExecuteTemplate(rw, "login.tmpl",
		LoginHint{*username, *password, "Login to Sample Site"})

	if err != nil {
		fmt.Printf("Error while trying to build login page: %v\n", err)
	}
}
