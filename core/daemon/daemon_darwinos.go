package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	darwinLabel    = "com.networkbooster.agent"
	darwinPlistDir = "Library/LaunchAgents"
)

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, darwinPlistDir, darwinLabel+".plist")
}

func installDarwin(execPath string) error {
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>start</string>
        <string>--compact</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
</dict>
</plist>`, darwinLabel, execPath, logPath("stdout"), logPath("stderr"))

	path := plistPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(plist), 0644); err != nil {
		return err
	}

	// Load the service
	return exec.Command("launchctl", "load", path).Run()
}

func uninstallDarwin() error {
	path := plistPath()
	// Unload first
	exec.Command("launchctl", "unload", path).Run()
	return os.Remove(path)
}

func isInstalledDarwin() bool {
	_, err := os.Stat(plistPath())
	return err == nil
}

func statusDarwin() string {
	out, err := exec.Command("launchctl", "list", darwinLabel).Output()
	if err != nil {
		return "installed but not running"
	}
	if len(out) > 0 {
		return "running"
	}
	return "installed"
}

func logPath(name string) string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".networkbooster", "logs")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, name+".log")
}
