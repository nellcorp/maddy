package basic_auth

import (
	"crypto/subtle"
	"os"

	echo "github.com/labstack/echo/v4"
)

func AdminBasicAuthValidator(username, password string, c echo.Context) (bool, error) {
	// Be careful to use constant time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(username), []byte(os.Getenv("ADMIN_EMAIL"))) == 1 &&
		subtle.ConstantTimeCompare([]byte(password), []byte(os.Getenv("ADMIN_PASSWORD"))) == 1 {
		return true, nil
	}
	return false, nil
}
