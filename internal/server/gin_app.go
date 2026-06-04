package server

import (
	"time"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func NewGinApp(cfg *config.Config) *gin.Engine {
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Forwarded-Prefix", "X-Forwarded-Host", "X-Forwarded-Proto"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.Use(ConfigMiddleware(cfg))

	return r
}

func ConfigMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := config.NewContext(c.Request.Context(), cfg)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
