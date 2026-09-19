package keychain

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	Service         = "kmo"
	TelegramService = "kmo-telegram"
)

type Store struct {
	Service string
	Account string
	Label   string
}

func Default() Store {
	return New(Service, "PAT")
}

func Telegram() Store {
	return New(TelegramService, "Telegram bot token")
}

func New(service, label string) Store {
	account := os.Getenv("USER")
	if account == "" {
		account = "kmo-user"
	}
	return Store{Service: service, Account: account, Label: label}
}

func (s Store) secretLabel() string {
	if s.Label != "" {
		return s.Label
	}
	return "secret"
}

func (s Store) Get() (string, error) {
	cmd := exec.Command("security", "find-generic-password", "-a", s.Account, "-s", s.Service, "-w")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("read %s from macOS Keychain: %s", s.secretLabel(), strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

func (s Store) Put(secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return fmt.Errorf("refusing to store empty %s", s.secretLabel())
	}
	cmd := exec.Command("security", "add-generic-password", "-a", s.Account, "-s", s.Service, "-w", secret, "-U")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("store %s in macOS Keychain: %s", s.secretLabel(), strings.TrimSpace(stderr.String()))
	}
	return nil
}
