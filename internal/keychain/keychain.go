package keychain

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const Service = "kmo"

type Store struct {
	Service string
	Account string
}

func Default() Store {
	account := os.Getenv("USER")
	if account == "" {
		account = "kmo-user"
	}
	return Store{Service: Service, Account: account}
}

func (s Store) Get() (string, error) {
	cmd := exec.Command("security", "find-generic-password", "-a", s.Account, "-s", s.Service, "-w")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("read PAT from macOS Keychain: %s", strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

func (s Store) Put(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("refusing to store empty PAT")
	}
	cmd := exec.Command("security", "add-generic-password", "-a", s.Account, "-s", s.Service, "-w", token, "-U")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("store PAT in macOS Keychain: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}
