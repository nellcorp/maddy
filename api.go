package maddy

import (
	"log"
	"net/http"
	"os"
	"sync"

	echo "github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/foxcpp/maddy/internal/rest/util/server"

	"github.com/foxcpp/maddy/internal/rest/util/middleware/basic_auth"
)

var (
	globals map[string]interface{}
	mods    []ModInfo
)

func startApi(globalConfig map[string]interface{}, modules []ModInfo, wg *sync.WaitGroup) (err error) {
	defer wg.Done()

	globals = globalConfig
	mods = modules

	if os.Getenv("ADMIN_EMAIL") == "" || os.Getenv("ADMIN_PASSWORD") == "" {
		log.Fatal("ADMIN_EMAIL and ADMIN_PASSWORD environment variables must be set")
	}

	e := server.New()

	NewV1(e)

	server.Start(e, &server.Config{
		Port:                "8080",
		ReadTimeoutSeconds:  30,
		WriteTimeoutSeconds: 30,
		Debug:               true,
	})

	return
}

func NewV1(e *echo.Echo) {
	e.GET("/", healthCheck)
	e.GET("/version", version)

	v1 := e.Group("/v1")
	v1.Use(middleware.BasicAuthWithConfig(middleware.BasicAuthConfig{Validator: basic_auth.AdminBasicAuthValidator}))

	users := v1.Group("/users")
	{
		users.POST("", createUser)
		users.GET("", listUsers)
		users.GET("/:id", getUser)
		users.PATCH("/:id/password", updateUserPassword)
		users.DELETE("/:id", deleteUser)
	}
}

func healthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, "OK")
}

func version(c echo.Context) error {
	return c.JSON(http.StatusOK, Version)
}
