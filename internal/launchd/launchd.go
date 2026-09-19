package launchd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const Label = "com.local.kmo-daemon"

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil { return "", err }
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), nil
}

func InstallAndStart(binary string) error {
	path, err := plistPath(); if err != nil { return err }
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { return err }
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key><array><string>%s</string><string>daemon</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>ProcessType</key><string>Background</string>
</dict>
</plist>
`, Label, binary)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { return err }
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), path).Run()
	if out, err := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), path).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %w: %s", err, string(out))
	}
	_ = exec.Command("launchctl", "kickstart", "-k", fmt.Sprintf("gui/%d/%s", os.Getuid(), Label)).Run()
	return nil
}

func Remove() error {
	path, err := plistPath(); if err != nil { return err }
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), path).Run()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) { return err }
	return nil
}
