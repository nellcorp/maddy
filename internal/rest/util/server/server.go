package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	tollbooth "github.com/didip/tollbooth/v6"
	limiter "github.com/didip/tollbooth/v6/limiter"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4/middleware"
	"github.com/lithammer/shortuuid"

	echo "github.com/labstack/echo/v4"
)

const rps = 3

// New instantates new Echo server
func New() *echo.Echo {
	e := echo.New()
	csrfConfig := middleware.CSRFConfig{
		Skipper:        CSRFSkipper,
		TokenLength:    32,
		TokenLookup:    "header:" + echo.HeaderXCSRFToken,
		ContextKey:     "csrf",
		CookieName:     "_csrf",
		CookieMaxAge:   86400,
		CookieDomain:   os.Getenv("FQDN"),
		CookieSecure:   true,
		CookieHTTPOnly: true,
	}

	recoverConfig := middleware.RecoverConfig{
		Skipper:           middleware.DefaultSkipper,
		StackSize:         4 << 10, // 4 KB
		DisableStackAll:   false,
		DisablePrintStack: false,
		LogLevel:          0,
		LogErrorFunc:      nil,
	}

	rid := middleware.RequestIDConfig{Generator: RequestIDGenerator}

	e.Use(
		middleware.LoggerWithConfig(middleware.LoggerConfig{Format: "${time_rfc3339} ${status} ${method}:${uri} ip:${remote_ip}\n"}),
		middleware.RecoverWithConfig(recoverConfig), corsHeaders(),
		middleware.CSRFWithConfig(csrfConfig),
		LimitHandler(setRateLimiter()),
		securityHeaders(),
		middleware.RequestIDWithConfig(rid),
		middleware.GzipWithConfig(middleware.GzipConfig{Level: 1, Skipper: GzipSkipper}),
	)

	V := validator.New(validator.WithRequiredStructEnabled())

	e.Validator = &CustomValidator{V}

	e.HideBanner = true
	e.Binder = &CustomBinder{b: &echo.DefaultBinder{}}
	return e
}

// Config represents server specific config
type Config struct {
	Port                string
	ReadTimeoutSeconds  int
	WriteTimeoutSeconds int
	Debug               bool
}

// Start starts echo server
func Start(e *echo.Echo, cfg *Config) {
	s := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		ReadTimeout:  time.Duration(cfg.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeoutSeconds) * time.Second,
	}

	e.Debug = cfg.Debug

	// Start server
	go func() {
		if err := e.StartServer(s); err != nil {
			e.Logger.Info("Shutting down the server", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 10 seconds.
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}

func setRateLimiter() (rateLimiter *limiter.Limiter) {
	requestsPerSecond := rps
	if temp, err := strconv.Atoi(os.Getenv("REQUESTS_PER_SECOND")); err == nil {
		requestsPerSecond = temp
	}

	// create a 1 request/second limiter and, every token bucket in it will expire 1 hour after it was initially set.
	rateLimiter = tollbooth.NewLimiter(float64(requestsPerSecond), &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour, ExpireJobInterval: time.Minute})

	// Configure list of places to look for IP address.
	// By default it's: "RemoteAddr", "X-Forwarded-For", "X-Real-IP"
	// If your application is behind a proxy, set "X-Forwarded-For" first.
	rateLimiter.SetIPLookups([]string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"})

	// Limit only GET and POST requests.
	rateLimiter.SetMethods([]string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})

	// Set a custom message.
	rateLimiter.SetMessage("You have reached maximum request limit.")

	// Set a custom content-type.
	rateLimiter.SetMessageContentType("application/json; charset=utf-8")

	return
}

func setApiKeyRateLimiter() (rateLimiter *limiter.Limiter) {
	requestsPerSecond := rps
	if temp, err := strconv.Atoi(os.Getenv("REQUESTS_PER_SECOND_API_KEY")); err == nil {
		requestsPerSecond = temp
	}

	// create a 1 request/second limiter and, every token bucket in it will expire 1 hour after it was initially set.
	rateLimiter = tollbooth.NewLimiter(float64(requestsPerSecond), &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour, ExpireJobInterval: time.Minute})

	// Configure list of places to look for IP address.
	// By default it's: "RemoteAddr", "X-Forwarded-For", "X-Real-IP"
	// If your application is behind a proxy, set "X-Forwarded-For" first.
	rateLimiter.SetIPLookups([]string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"})

	// Limit only GET and POST requests.
	rateLimiter.SetMethods([]string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})

	// Limit request headers containing certain values.
	rateLimiter.SetHeader("X-Access-Token", []string{})

	// Set a custom message.
	rateLimiter.SetMessage("You have reached maximum request limit.")

	// Set a custom content-type.
	rateLimiter.SetMessageContentType("application/json; charset=utf-8")

	return
}

func CSRFSkipper(c echo.Context) bool {
	if os.Getenv("USE_COOKIES") == "false" || os.Getenv("USE_COOKIES") == "" {
		return true
	}
	return false
}

func GzipSkipper(c echo.Context) bool {
	if strings.Contains(c.Request().URL.Path, "api") {
		return true
	}
	return false
}

func RequestIDGenerator() string {
	return shortuuid.New()
}
