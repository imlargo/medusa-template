package main

import (
	"context"
	"log"

	"github.com/imlargo/medusa/cmd/api/bootstrap"
)

// @title Medusa
// @version 1.0
// @description Medusa example api

// @contact.name Default
// @contact.url https://default.dev
// @license.name MIT
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

// @securityDefinitions.apiKey ApiKey
// @in header
// @name X-API-Key
func main() {
	app, err := bootstrap.New("medusa-api")
	if err != nil {
		log.Fatal(err)
	}
	defer app.Container.Cleanup()

	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
