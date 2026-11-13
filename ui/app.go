package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/eugenegoncharuk/keyvault-manager/azure"
)

// Run is the application entry point called from main.go.
func Run(a fyne.App) {
	a.Settings().SetTheme(kvTheme{})

	w := a.NewWindow("🔐 Azure Key Vault Manager")
	w.Resize(fyne.NewSize(1200, 820))
	w.CenterOnScreen()

	// ── Selectors ──────────────────────────────────────────────────────────
	subSelect := widget.NewSelect(nil, nil)
	subSelect.PlaceHolder = "Loading subscriptions…"

	vaultSelect := widget.NewSelect(nil, nil)
	vaultSelect.PlaceHolder = "Select vault…"

	statusBar := newStatusLine(w)

	var loadedSubs []azure.Subscription

	// subGen is bumped on every subscription switch (main goroutine only) so
	// that results from a slower, superseded switch are discarded instead of
	// overwriting the vault list of the subscription now selected.
	var subGen int

	// ── Subscription change ────────────────────────────────────────────────
	subSelect.OnChanged = func(name string) {
		for _, s := range loadedSubs {
			if s.Name != name {
				continue
			}
			subGen++
			gen := subGen
			vaultSelect.ClearSelected()
			vaultSelect.Options = nil
			vaultSelect.PlaceHolder = "Loading vaults…"
			vaultSelect.Refresh()
			statusBar.SetText(fmt.Sprintf("⏳ Switching to '%s'…", name))

			go func(id string) {
				var vaults []string
				err := azure.SetSubscription(id)
				if err == nil {
					vaults, err = azure.ListVaults()
				}
				fyne.Do(func() {
					if gen != subGen {
						return
					}
					if err != nil {
						statusBar.SetError("Loading vaults failed: %v", err)
						vaultSelect.PlaceHolder = "Failed to load vaults"
						vaultSelect.Refresh()
						return
					}
					vaultSelect.Options = vaults
					if len(vaults) > 0 {
						vaultSelect.SetSelected(vaults[0])
					} else {
						vaultSelect.PlaceHolder = "No vaults found"
						vaultSelect.Refresh()
					}
					statusBar.SetText(fmt.Sprintf("✅ %d vault(s) found", len(vaults)))
				})
			}(s.ID)
			return
		}
	}

	// ── Tabs ───────────────────────────────────────────────────────────────
	tabs := container.NewAppTabs(
		container.NewTabItem("⚙️  Manage", NewManageTab(vaultSelect, w)),
		container.NewTabItem("📜  History", NewHistoryTab(vaultSelect, w)),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	// ── Header bar ─────────────────────────────────────────────────────────
	selectorRow := container.NewHBox(
		widget.NewLabel("Subscription:"),
		subSelect,
		widget.NewLabel("   Key Vault:"),
		vaultSelect,
	)
	header := container.NewBorder(nil, nil, selectorRow, nil, statusBar.obj)

	w.SetContent(container.NewBorder(header, nil, nil, nil, tabs))

	// ── Load subscriptions ───────────────────────────────────────────
	go func() {
		subs, err := azure.ListSubscriptions()
		fyne.Do(func() {
			if err != nil {
				statusBar.SetError("Failed to load subscriptions: %v", err)
				subSelect.PlaceHolder = "Failed to load subscriptions"
				subSelect.Refresh()
				return
			}
			loadedSubs = subs
			names := make([]string, len(subs))
			defaultIdx := 0
			for i, s := range subs {
				names[i] = s.Name
				if s.IsDefault {
					defaultIdx = i
				}
			}
			subSelect.Options = names
			subSelect.Refresh()
			if len(names) > 0 {
				subSelect.SetSelected(names[defaultIdx])
			}
		})
	}()

	w.ShowAndRun()
}
