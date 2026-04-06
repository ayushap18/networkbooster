package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Install registers networkbooster as an OS service.
func Install() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not find executable: %w", err)
	}
	execPath, err = filepath.Abs(execPath)
	if err != nil {
		return err
	}

	switch runtime.GOOS {
	case "darwin":
		return installDarwin(execPath)
	case "linux":
		return installLinux(execPath)
	default:
		return fmt.Errorf("daemon install not supported on %s", runtime.GOOS)
	}
}

// Uninstall removes the networkbooster OS service.
func Uninstall() error {
	switch runtime.GOOS {
	case "darwin":
		return uninstallDarwin()
	case "linux":
		return uninstallLinux()
	default:
		return fmt.Errorf("daemon uninstall not supported on %s", runtime.GOOS)
	}
}

// IsInstalled checks if the service is registered.
func IsInstalled() bool {
	switch runtime.GOOS {
	case "darwin":
		return isInstalledDarwin()
	case "linux":
		return isInstalledLinux()
	default:
		return false
	}
}

// Status returns the service status string.
func Status() string {
	if !IsInstalled() {
		return "not installed"
	}
	switch runtime.GOOS {
	case "darwin":
		return statusDarwin()
	case "linux":
		return statusLinux()
	default:
		return "unknown"
	}
}
