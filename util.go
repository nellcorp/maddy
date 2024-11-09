package maddy

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/updatepipe"
)

type (
	SpecialUseUser interface {
		CreateMailboxSpecial(name, specialUseAttr string) error
	}

	Mailboxes struct {
		SentName    string
		TrashName   string
		JunkName    string
		DraftsName  string
		ArchiveName string
	}
)

func getCfgBlockModule(name string) (*ModInfo, error) {
	var mod ModInfo
	for _, m := range mods {
		if m.Instance.InstanceName() == name {
			mod = m
			break
		}
	}
	if mod.Instance == nil {
		return nil, fmt.Errorf("Error: unknown configuration block: %s", name)
	}

	return &mod, nil
}

func closeIfNeeded(i interface{}) {
	if c, ok := i.(io.Closer); ok {
		c.Close()
	}
}

func openUserDB() (module.PlainUserDB, error) {
	moduleName := "local_authdb"
	mod, err := getCfgBlockModule(moduleName)
	if err != nil {
		return nil, err
	}

	userDB, ok := mod.Instance.(module.PlainUserDB)
	if !ok {
		return nil, fmt.Errorf("Error: configuration block %s is not a local credentials store", moduleName)
	}

	if err := mod.Instance.Init(config.NewMap(globals, mod.Cfg)); err != nil {
		return nil, fmt.Errorf("Error: module initialization failed: %w", err)
	}

	return userDB, nil
}

func openStorage() (storage module.Storage, mailboxes Mailboxes, err error) {
	moduleName := "local_mailboxes"
	mod, err := getCfgBlockModule(moduleName)
	if err != nil {
		return
	}

	storage, ok := mod.Instance.(module.Storage)
	if !ok {
		err = fmt.Errorf("Error: configuration block %s is not an IMAP storage", moduleName)
		return
	}

	if err = mod.Instance.Init(config.NewMap(globals, mod.Cfg)); err != nil {
		err = fmt.Errorf("Error: module initialization failed: %w", err)
		return
	}

	if updStore, ok := mod.Instance.(updatepipe.Backend); ok {
		if err := updStore.EnableUpdatePipe(updatepipe.ModePush); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "Failed to initialize update pipe, do not remove messages from mailboxes open by clients: %v\n", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "No update pipe support, do not remove messages from mailboxes open by clients\n")
	}

	for _, child := range mod.Cfg.Children {
		if child.Name == "sent_mailbox" && len(child.Args) > 0 {
			mailboxes.SentName = child.Args[0]
		}
		if child.Name == "trash_mailbox" && len(child.Args) > 0 {
			mailboxes.TrashName = child.Args[0]
		}
		if child.Name == "junk_mailbox" && len(child.Args) > 0 {
			mailboxes.JunkName = child.Args[0]
		}
		if child.Name == "drafts_mailbox" && len(child.Args) > 0 {
			mailboxes.DraftsName = child.Args[0]
		}
		if child.Name == "archive_mailbox" && len(child.Args) > 0 {
			mailboxes.ArchiveName = child.Args[0]
		}
	}

	return
}
