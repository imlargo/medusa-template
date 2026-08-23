// Package docs serves generated OpenAPI documentation over Swagger UI.
//
// The spec is passed in rather than imported, which is what keeps this package
// independent of any particular application. Wire it with the spec swag
// generated for your own service:
//
//	import "github.com/your/app/api/docs"
//
//	medusadocs.RegisterDocs(router, medusadocs.Config{
//	    Spec: docs.SwaggerInfo,
//	    Host: cfg.Server.Host,
//	    Port: cfg.Server.Port,
//	})
package docs

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa/tools"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/swag"
)

// RoutePath is where Swagger UI and the JSON spec are mounted.
const RoutePath = "/docs"

// Config describes the documentation endpoint.
type Config struct {
	// Spec is the generated spec, normally the SwaggerInfo variable from your
	// application's api/docs package. When nil the UI is still served and reads
	// whatever spec swag registered globally.
	Spec *swag.Spec

	// Host is the public host of the API, with or without a scheme.
	Host string

	// Port is appended to Host for local addresses, where the port is part of
	// how the API is reached.
	Port int
}

// RegisterDocs mounts Swagger UI at /docs and points it at the JSON spec.
//
// It rewrites the spec's host and scheme so the "Try it out" button targets the
// running instance instead of whatever was baked in at generation time.
func RegisterDocs(router *gin.Engine, cfg Config) {
	host := cfg.Host
	if tools.IsLocalhost(host) {
		host = tools.CleanHostURL(host) + ":" + strconv.Itoa(cfg.Port)
	}

	if cfg.Spec != nil {
		cfg.Spec.Host = host
		cfg.Spec.BasePath = "/"
		cfg.Spec.Schemes = []string{schemeFor(host)}
	}

	specURL := ginSwagger.URL(tools.ToCompleteURL(host) + RoutePath + "/doc.json")
	router.GET(RoutePath+"/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, specURL))
}

// schemeFor picks the scheme the documented API is reached over.
func schemeFor(host string) string {
	if tools.IsHTTPS(host) || !tools.IsLocalhost(host) {
		return "https"
	}
	return "http"
}
