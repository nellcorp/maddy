package maddy

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/emersion/go-imap"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/rest/model"
	echo "github.com/labstack/echo/v4"
)

func createImapAccount(c echo.Context) error {
	r := model.User{}

	if err := c.Bind(&r); err != nil {
		return err
	}

	err := imapAcctCreate(r.Username)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusCreated)
}

func listImapAccounts(c echo.Context) error {
	list, err := imapAcctList()
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, list)
}

func deleteImapAccount(c echo.Context) error {
	err := imapAcctRemove(c.Param("username"))
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func imapAcctList() (list []string, err error) {
	list = []string{}

	be, _, err := openStorage()
	if err != nil {
		return
	}
	defer closeIfNeeded(be)

	mbe, ok := be.(module.ManageableStorage)
	if !ok {
		err = errors.New("Error: storage backend does not support accounts management using maddy command")
		return

	}

	list, err = mbe.ListIMAPAccts()
	if err != nil {
		return
	}

	if len(list) == 0 {
		return []string{}, nil
	}

	return
}

func imapAcctCreate(username string) error {
	be, mbox, err := openStorage()
	if err != nil {
		return err
	}
	defer closeIfNeeded(be)

	mbe, ok := be.(module.ManageableStorage)
	if !ok {
		return errors.New("Error: storage backend does not support accounts management using maddy command")
	}

	if err := mbe.CreateIMAPAcct(username); err != nil {
		return err
	}

	act, err := mbe.GetIMAPAcct(username)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	suu, ok := act.(SpecialUseUser)
	if !ok {
		fmt.Fprintf(os.Stderr, "Note: Storage backend does not support SPECIAL-USE IMAP extension")
	}

	createMbox := func(name, specialUseAttr string) error {
		if suu == nil {
			return act.CreateMailbox(name)
		}
		return suu.CreateMailboxSpecial(name, specialUseAttr)
	}

	if err := createMbox(mbox.SentName, imap.SentAttr); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create sent folder: %v", err)
	}
	if err := createMbox(mbox.TrashName, imap.TrashAttr); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create trash folder: %v", err)
	}
	if err := createMbox(mbox.JunkName, imap.JunkAttr); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create junk folder: %v", err)
	}
	if err := createMbox(mbox.DraftsName, imap.DraftsAttr); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create drafts folder: %v", err)
	}
	if err := createMbox(mbox.ArchiveName, imap.ArchiveAttr); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create archive folder: %v", err)
	}

	return nil
}

func imapAcctRemove(username string) error {
	be, _, err := openStorage()
	if err != nil {
		return err
	}
	defer closeIfNeeded(be)

	mbe, ok := be.(module.ManageableStorage)
	if !ok {
		return errors.New("storage backend does not support accounts management using maddy command")
	}

	return mbe.DeleteIMAPAcct(username)
}
