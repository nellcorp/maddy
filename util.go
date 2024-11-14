package maddy

import (
	"errors"
	"fmt"
	"os"

	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/modify"
	"github.com/foxcpp/maddy/internal/modify/dkim"
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

func getCfgBlockModule(mods []ModInfo, name string) (*ModInfo, error) {
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

func openUserDB() (module.PlainUserDB, error) {
	moduleName := "local_authdb"

	userDBModule, err := module.GetInstance(moduleName)
	if err != nil {
		return nil, err
	}
	userDB, ok := userDBModule.(module.PlainUserDB)
	if !ok {
		return nil, fmt.Errorf("Error: configuration block %s is not a local credentials store", moduleName)
	}

	return userDB, nil
}

func openStorage(mods []ModInfo) (storage module.ManageableStorage, mailboxes Mailboxes, err error) {
	moduleName := "local_mailboxes"
	mod, err := getCfgBlockModule(mods, moduleName)
	if err != nil {
		return nil, Mailboxes{}, nil
	}

	if mod == nil {
		err = fmt.Errorf("Error: configuration block %s is not found", moduleName)
		return nil, Mailboxes{}, err
	}

	storage, ok := mod.Instance.(module.ManageableStorage)
	if !ok {
		err = fmt.Errorf("Error: configuration block %s is not a writable IMAP storage", moduleName)
		return nil, Mailboxes{}, err
	}

	if updStore, ok := mod.Instance.(updatepipe.Backend); ok {
		if err := updStore.EnableUpdatePipe(updatepipe.ModePush); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "ErrorFailed to initialize update pipe, do not remove messages from mailboxes open by clients: %v\n", err)
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

	return storage, mailboxes, nil
}

func openDKIM(mods []ModInfo) (*dkim.Modifier, error) {
	for _, mod := range mods {
		if group, ok := mod.Instance.(*modify.Group); ok {
			for _, modifier := range group.Modifiers {
				if dkimModifier, ok := modifier.(*dkim.Modifier); ok {
					return dkimModifier, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("Error: DKIM modifier not found.")
}
