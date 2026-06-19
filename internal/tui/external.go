package tui

import (
	"os/exec"
	"runtime"
	"strings"
)

func isPrintableKey(value string) bool {
	runes := []rune(value)
	if len(runes) != 1 {
		return false
	}
	return runes[0] >= 32 && runes[0] != 127
}

func defaultOpenExternal(target string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", target).Run()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Run()
	default:
		return exec.Command("xdg-open", target).Run()
	}
}

func defaultCopyToClipboard(value string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default:
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		}
	}
	cmd.Stdin = strings.NewReader(value)
	return cmd.Run()
}
