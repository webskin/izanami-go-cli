package utils

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// isWSL detects if running in Windows Subsystem for Linux
func isWSL() bool {
	// Check /proc/version for WSL indicators
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	version := strings.ToLower(string(data))
	return strings.Contains(version, "microsoft") || strings.Contains(version, "wsl")
}

// OpenBrowser opens the specified URL in the default browser.
// Returns an error if the platform is unsupported or the browser fails to open.
// In WSL environments, it opens the Windows default browser.
func OpenBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		if isWSL() {
			// In WSL, use cmd.exe to open Windows browser
			cmd = "cmd.exe"
			args = []string{"/c", "start", "", url}
		} else {
			cmd = "xdg-open"
			args = []string{url}
		}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return exec.Command(cmd, args...).Start()
}
