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
	r := model.CreateUserDto{}

	if err := c.Bind(&r); err != nil {
		return err
	}

	if err := userCreate(r.Username, r.Password, r.CreateMailboxes); err != nil {
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
	deleteMailbox := c.QueryParam("delete_mailbox") == "true"
	if err := userDelete(c.Param("id"), deleteMailbox); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func updateUserPassword(c echo.Context) error {
	r := model.Password{}

	if err := c.Bind(&r); err != nil {
		return err
	}

	if err := userDb.SetUserPassword(c.Param("id"), r.Password); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func userCreate(username, password string, createMailboxes bool) (err error) {
	beHash, ok := userDb.(*pass_table.Auth)
	if !ok {
		return fmt.Errorf("Hash cannot be used with non-pass_table credentials DB")
	}

	userParts := strings.Split(username, "@")
	if len(userParts) != 2 {
		return fmt.Errorf("Invalid username format")
	}
	domain := userParts[1]

	users, err := userDb.ListUsers()
	if err != nil {
		return err
	}

	found := false
	for _, user := range users {
		if strings.HasSuffix(user, domain) {
			found = true
			break
		}
	}

	err = beHash.CreateUserHash(username, password, pass_table.HashBcrypt, pass_table.HashOpts{
		BcryptCost: 10,
	})
	if err != nil {
		return err
	}

	if !found && dkimModule != nil {
		if err = dkimModule.AddKey(domain); err != nil {
			return err
		}
	}

	if createMailboxes {
		err = imapAcctCreate(username)
		if err != nil {
			return err
		}
	}

	return err
}

func userList() (list []string, err error) {
	list, err = userDb.ListUsers()
	if err != nil {
		return
	}

	if len(list) == 0 {
		return []string{}, nil
	}

	return
}

func userDelete(username string, deleteMailbox bool) (err error) {
	err = userDb.DeleteUser(username)
	if err != nil {
		return
	}

	if deleteMailbox {
		err = imapAcctRemove(username)
		if err != nil {
			return
		}
	}

	return
}
