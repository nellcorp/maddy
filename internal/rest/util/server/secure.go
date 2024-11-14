package server

import (
	"os"
	"strings"

	echo "github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// securityHeaders adds general security headers for basic security measures
func securityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Protects from MimeType Sniffing
			c.Response().Header().Set("X-Content-Type-Options", "nosniff")
			// Prevents browser from prefetching DNS
			c.Response().Header().Set("X-DNS-Prefetch-Control", "off")
			// Denies website content to be served in an iframe
			c.Response().Header().Set("X-Frame-Options", "DENY")
			c.Response().Header().Set("Strict-Transport-Security", "max-age=5184000; includeSubDomains")
			// Prevents Internet Explorer from executing downloads in site's context
			c.Response().Header().Set("X-Download-Options", "noopen")
			// Minimal XSS protection
			c.Response().Header().Set("X-XSS-Protection", "1; mode=block")

			if os.Getenv("CONTENT_SECURITY_POLICY") != "" {
				c.Response().Header().Set("Content-Security-Policy", os.Getenv("CONTENT_SECURITY_POLICY"))
			}
			return next(c)
		}
	}
}

// corsHeaders adds Cross-Origin Resource Sharing support
func corsHeaders() echo.MiddlewareFunc {
	origins := []string{"*"}
	if strings.Replace(os.Getenv("CORS_ALLOW_ORIGINS"), " ", "", -1) != "" {
		origins = strings.Split(os.Getenv("CORS_ALLOW_ORIGINS"), ",")
	}

	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     origins,
		MaxAge:           86400,
		AllowMethods:     []string{"POST", "GET", "PUT", "DELETE", "PATCH", "HEAD"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"Content-Length", "Page", "NextCursor", "PreviousCursor", "TotalResults", "TotalPages"},
		AllowCredentials: true,
	})
}
