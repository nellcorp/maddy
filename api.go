package maddy

import (
	"net/http"
	"os"
	"sync"

	echo "github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/modify/dkim"
	"github.com/foxcpp/maddy/internal/rest/util/server"

	"github.com/foxcpp/maddy/internal/rest/util/middleware/basic_auth"
)

var (
	userDb     module.PlainUserDB
	imapDb     module.ManageableStorage
	mailboxes  Mailboxes
	dkimModule *dkim.Modifier
)

func startApi(mods []ModInfo, wg *sync.WaitGroup) (err error) {
	defer wg.Done()

	userDb, err = openUserDB()
	if err != nil {
		return err
	}

	imapDb, mailboxes, err = openStorage(mods)
	if err != nil {
		return err
	}

	var dkimErr error
	if dkimModule, dkimErr = openDKIM(mods); dkimErr != nil {
		log.Printf("DKIM module not found, DKIM signing will be disabled: %v\n", dkimErr)
	}

	if os.Getenv("ADMIN_EMAIL") == "" || os.Getenv("ADMIN_PASSWORD") == "" {
		log.Println("ADMIN_EMAIL and ADMIN_PASSWORD environment variables must be set")
		os.Exit(1)
	}

	e := server.New()

	NewV1(e)

	server.Start(e, &server.Config{
		Port:                "8080",
		ReadTimeoutSeconds:  30,
		WriteTimeoutSeconds: 30,
		Debug:               true,
	})

	return err
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
		users.POST("/:id/password", updateUserPassword)
		users.DELETE("/:id", deleteUser)
	}

	mailboxes := v1.Group("/users/:id/mailboxes")
	{
		mailboxes.POST("", createImapAccount)
		mailboxes.DELETE("", deleteImapAccount)
	}
}

func healthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, "OK")
}

func version(c echo.Context) error {
	return c.JSON(http.StatusOK, Version)
}
