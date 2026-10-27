// Package apidocs serves the embedded OpenAPI document and Swagger UI.
package apidocs

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed openapi.json
var specification []byte

const ui = `<!DOCTYPE html>
<html lang="ja"><head><meta charset="utf-8"><title>Kotonoha API</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css"></head>
<body><div id="swagger-ui"></div>
<script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
<script>SwaggerUIBundle({url: './openapi.json', dom_id: '#swagger-ui', deepLinking: true});</script>
</body></html>`

// Register adds documentation endpoints without changing API authentication.
func Register(engine *gin.Engine) {
	engine.GET("/swagger", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "swagger/") })
	engine.GET("/swagger/", func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(ui)) })
	engine.GET("/swagger/openapi.json", func(c *gin.Context) { c.Data(http.StatusOK, "application/json; charset=utf-8", specification) })
}
