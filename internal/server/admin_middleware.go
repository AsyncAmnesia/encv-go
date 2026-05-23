package server

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/Soltus/encv-go/internal/auth"
	"github.com/Soltus/encv-go/internal/routes"
)

func JWTAuthMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasSuffix(c.Request.URL.Path, routes.Login) ||
			strings.HasSuffix(c.Request.URL.Path, routes.Logout) {
			c.Next()
			return
		}

		log.Printf("[JWTAuthMiddleware] Processing path: %s", c.Request.URL.Path)

		token := auth.GetTokenFromCookie(c.Request)

		if token != "" {
			c.Request.Header.Set("Authorization", "Bearer "+token)
		}

		if token == "" {
			redirectToLoginGin(c)
			c.Abort()
			return
		}

		_, err := jwtManager.ValidateToken(token)
		if err != nil {
			auth.ClearAuthCookie(c.Writer)
			redirectToLoginGin(c)
			c.Abort()
			return
		}

		claims, _ := jwtManager.ValidateToken(token)
		c.Set("auth_claims", claims)
		c.Set("is_authenticated", true)

		c.Next()
	}
}

func redirectToLoginGin(c *gin.Context) {
	currentURL := c.Request.URL.String()
	if currentURL != "" && !strings.Contains(currentURL, routes.Login) {
		auth.SetRedirectCookie(c.Writer, currentURL)
	}

	c.Redirect(http.StatusFound, routes.Login)
}
