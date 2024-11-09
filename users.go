package maddy

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/foxcpp/maddy/internal/rest/model"
	echo "github.com/labstack/echo/v4"

	"github.com/foxcpp/maddy/internal/auth/pass_table"
)

func createUser(c echo.Context) error {
	r := model.User{}

	if err := c.Bind(&r); err != nil {
		return err
	}

	if err := userCreate(r.Username, r.Password); err != nil {
		return err
	}

	return c.NoContent(http.StatusCreated)
}

func listUsers(c echo.Context) error {
	list, err := userList()
	if err != nil {
		return err
	}

	results := []string{}
	domain := c.QueryParam("domain")
	if domain != "" {
		for _, user := range list {
			if strings.HasSuffix(user, fmt.Sprintf("@%s", domain)) {
				results = append(results, user)
			}
		}
		return c.JSON(http.StatusOK, results)
	}

	return c.JSON(http.StatusOK, list)
}

func getUser(c echo.Context) error {
	list, err := userList()
	if err != nil {
		return err
	}

	for _, user := range list {
		if user == c.Param("id") {
			return c.JSON(http.StatusOK, model.User{Username: user})
		}
	}

	return c.NoContent(http.StatusNotFound)
}

func deleteUser(c echo.Context) error {
	if err := userDelete(c.Param("id")); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func updateUserPassword(c echo.Context) error {
	r := model.Password{}

	if err := c.Bind(&r); err != nil {
		return err
	}

	be, err := openUserDB()
	if err != nil {
		return err
	}
	defer closeIfNeeded(be)

	if err = be.SetUserPassword(c.Param("id"), r.Password); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func userCreate(username, password string) (err error) {
	be, err := openUserDB()
	if err != nil {
		return
	}
	defer closeIfNeeded(be)

	beHash, ok := be.(*pass_table.Auth)
	if !ok {
		return fmt.Errorf("Hash cannot be used with non-pass_table credentials DB")
	}

	err = beHash.CreateUserHash(username, password, pass_table.HashBcrypt, pass_table.HashOpts{
		BcryptCost: 10,
	})
	if err != nil {
		return err
	}

	err = imapAcctCreate(username)
	if err != nil {
		return
	}

	return
}

func userList() (list []string, err error) {
	list = []string{}
	be, err := openUserDB()
	if err != nil {
		return
	}
	defer closeIfNeeded(be)

	list, err = be.ListUsers()
	if err != nil {
		return
	}

	if len(list) == 0 {
		return []string{}, nil
	}

	return
}

func userDelete(username string) (err error) {
	be, err := openUserDB()
	if err != nil {
		return
	}
	defer closeIfNeeded(be)

	err = be.DeleteUser(username)
	if err != nil {
		return
	}

	err = imapAcctRemove(username)
	if err != nil {
		return
	}
	return
}
