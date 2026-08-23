// Command api runs the Medusa HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/imlargo/medusa/cmd/api/bootstrap"
)

// appName identifies the application in logs and in the startup banner.
const appName = "medusa-api"

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
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", appName, err)
		os.Exit(1)
	}
}

// run owns the application lifecycle. It exists so that shutdown always runs:
// os.Exit — and therefore log.Fatal — would skip every deferred call.
func run(ctx context.Context) (err error) {
	app, appErr := bootstrap.New(appName)
	if appErr != nil {
		return appErr
	}

	defer func() {
		err = errors.Join(err, app.Close())
	}()

	return app.Run(ctx)
}
