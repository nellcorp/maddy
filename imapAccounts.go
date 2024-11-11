package maddy

import (
	"fmt"
	"net/http"
	"os"

	"github.com/emersion/go-imap"
	echo "github.com/labstack/echo/v4"
)

func createImapAccount(c echo.Context) error {
	err := imapAcctCreate(c.Param("id"))
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusCreated)
}

func deleteImapAccount(c echo.Context) error {
	err := imapAcctRemove(c.Param("id"))
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func imapAcctCreate(username string) error {
	if err := imapDb.CreateIMAPAcct(username); err != nil {
		return err
	}

	act, err := imapDb.GetIMAPAcct(username)
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

	if err := createMbox(mailboxes.SentName, imap.SentAttr); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create sent folder: %v", err)
	}
	if err := createMbox(mailboxes.TrashName, imap.TrashAttr); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create trash folder: %v", err)
	}
	if err := createMbox(mailboxes.JunkName, imap.JunkAttr); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create junk folder: %v", err)
	}
	if err := createMbox(mailboxes.DraftsName, imap.DraftsAttr); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create drafts folder: %v", err)
	}
	if err := createMbox(mailboxes.ArchiveName, imap.ArchiveAttr); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create archive folder: %v", err)
	}

	return nil
}

func imapAcctRemove(username string) error {
	return imapDb.DeleteIMAPAcct(username)
}
