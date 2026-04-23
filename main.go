package main

import (
	"os"
	"strings"

	"fyne.io/fyne/v2/app"
	"github.com/eugenegoncharuk/keyvault-manager/ui"
)

func main() {
	// macOS app bundles do not inherit the user's interactive shell $PATH.
	// Inject common Homebrew paths so that `exec.Command("az")` works when
	// launched from Finder or Spotlight.
	path := os.Getenv("PATH")
	if !strings.Contains(path, "/opt/homebrew/bin") {
		os.Setenv("PATH", "/opt/homebrew/bin:/usr/local/bin:"+path)
	}

	a := app.NewWithID("com.eugenegoncharuk.keyvaultmanager")
	ui.Run(a)
}
