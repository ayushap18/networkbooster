package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const serviceName = "networkbooster.service"

func unitPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", serviceName)
}

func installLinux(execPath string) error {
	unit := fmt.Sprintf(`[Unit]
Description=NetworkBooster Bandwidth Optimizer
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=%s start --compact
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, execPath)

	path := unitPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(unit), 0644); err != nil {
		return err
	}

	// Reload and enable
	exec.Command("systemctl", "--user", "daemon-reload").Run()
	return exec.Command("systemctl", "--user", "enable", "--now", serviceName).Run()
}

func uninstallLinux() error {
	exec.Command("systemctl", "--user", "disable", "--now", serviceName).Run()
	path := unitPath()
	os.Remove(path)
	exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

func isInstalledLinux() bool {
	_, err := os.Stat(unitPath())
	return err == nil
}

func statusLinux() string {
	out, err := exec.Command("systemctl", "--user", "is-active", serviceName).Output()
	if err != nil {
		return "installed but not running"
	}
	return string(out)
}
