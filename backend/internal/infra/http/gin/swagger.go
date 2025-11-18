package ginserver

import (
	_ "embed"
	"net/http"
	"strings"

	gin "github.com/gin-gonic/gin"
)

//go:embed swagger/openapi.json
var swaggerSpec []byte

//go:embed swagger/index.html
var swaggerHTML string

func registerSwaggerRoutes(router gin.IRoutes) {
	router.GET("/swagger/doc.json", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", swaggerSpec)
	})
	router.GET("/swagger", func(c *gin.Context) {
		html := strings.ReplaceAll(swaggerHTML, "{{SPEC_URL}}", "/swagger/doc.json")
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})
}
