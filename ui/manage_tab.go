package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/eugenegoncharuk/keyvault-manager/azure"
)

// NewManageTab builds the combined Read + Push "Manage" tab.
//
// Dirty-state rules:
//   - isDirty is set whenever the editor text changes after a successful read.
//   - Clicking Read while isDirty → confirmation dialog ("discard changes?").
//   - A push that fails with a version conflict → error dialog telling the
//     user to re-read the secret.
//
// Threading: all tab-local state and widgets are touched only on the Fyne
// main goroutine. Background az calls hand their results back via fyne.Do.
func NewManageTab(vaultSelect *widget.Select, w fyne.Window) fyne.CanvasObject {
	// ── Tab-local state (main goroutine only) ─────────────────────────────
	var (
		currentVault   string
		currentSecret  string
		currentVersion string
		isDirty        bool
		suppressChange bool
	)

	// ── Widgets ────────────────────────────────────────────────────────────
	secretEntry := widget.NewEntry()
	secretEntry.PlaceHolder = "Enter secret name…"

	versionLabel := widget.NewLabel("Version: —")
	versionLabel.TextStyle = fyne.TextStyle{Monospace: true}

	statusLabel := newStatusLine(w)

	dirtyLabel := widget.NewLabel("")
	dirtyLabel.TextStyle = fyne.TextStyle{Bold: true}

	editor := NewSyntaxEditor()
	editor.OnChanged = func(_ string) {
		if !suppressChange {
			isDirty = true
			dirtyLabel.SetText("● Unsaved changes")
		}
	}

	readBtn := widget.NewButton("🔍  Read", nil)
	readBtn.Importance = widget.HighImportance

	pushBtn := widget.NewButton("🚀  Push Secret", nil)
	pushBtn.Importance = widget.DangerImportance

	clearBtn := widget.NewButton("Clear", nil)

	copyBtn := widget.NewButton("📋  Copy", func() {
		t := editor.GetText()
		if t == "" {
			return
		}
		w.Clipboard().SetContent(t)
		statusLabel.SetText("📋 Copied to clipboard")
	})

	// ── Helpers ────────────────────────────────────────────────────────────
	applyRead := func(vault, secret, version, value string) {
		currentVault = vault
		currentSecret = secret
		currentVersion = version
		suppressChange = true
		editor.SetText(value)
		suppressChange = false
		isDirty = false
		dirtyLabel.SetText("")
		versionLabel.SetText(fmt.Sprintf("Version: %s", version))
		statusLabel.SetText(fmt.Sprintf("✅ '%s' from '%s'", secret, vault))
	}

	// ── performRead ────────────────────────────────────────────────────────
	var performRead func(vault, secret string)
	performRead = func(vault, secret string) {
		readBtn.Disable()
		statusLabel.SetText("⏳ Reading secret…")

		go func() {
			value, version, err := azure.GetSecret(vault, secret)
			fyne.Do(func() {
				readBtn.Enable()
				if err != nil {
					statusLabel.SetError("%v", err)
					return
				}
				applyRead(vault, secret, version, value)
			})
		}()
	}

	// ── Read button ────────────────────────────────────────────────────────
	readBtn.OnTapped = func() {
		vault := vaultSelect.Selected
		secret := secretEntry.Text

		if vault == "" {
			dialog.ShowError(fmt.Errorf("please select a Key Vault in the header bar"), w)
			return
		}
		if secret == "" {
			dialog.ShowError(fmt.Errorf("please enter a secret name"), w)
			return
		}

		if isDirty {
			msg := fmt.Sprintf(
				"You have unsaved changes to '%s'.\n\nDiscard them and read '%s'?",
				currentSecret, secret)
			dialog.ShowConfirm("Unsaved Changes", msg, func(ok bool) {
				if ok {
					performRead(vault, secret)
				}
			}, w)
			return
		}

		performRead(vault, secret)
	}

	// ── Push button ────────────────────────────────────────────────────────
	pushBtn.OnTapped = func() {
		vault := vaultSelect.Selected
		secret := secretEntry.Text
		value := editor.GetText()

		if vault == "" {
			dialog.ShowError(fmt.Errorf("please select a Key Vault in the header bar"), w)
			return
		}
		if secret == "" {
			dialog.ShowError(fmt.Errorf("please enter a secret name"), w)
			return
		}
		if strings.TrimSpace(value) == "" {
			dialog.ShowError(fmt.Errorf("secret value cannot be empty"), w)
			return
		}

		ev := ""
		if vault == currentVault && secret == currentSecret {
			ev = currentVersion
		}

		confirmMsg := fmt.Sprintf("Push secret '%s'\nto vault '%s'?", secret, vault)
		if ev == "" {
			confirmMsg += "\n\n⚠️  No prior read — version-conflict check skipped."
		}

		dialog.ShowConfirm("Confirm Push", confirmMsg, func(ok bool) {
			if !ok {
				return
			}
			pushBtn.Disable()
			statusLabel.SetText("⏳ Pushing…")

			go func() {
				newVersion, err := azure.SetSecret(vault, secret, value, ev)
				fyne.Do(func() {
					pushBtn.Enable()
					if err != nil {
						if strings.Contains(err.Error(), "version conflict") {
							dialog.ShowError(err, w)
							statusLabel.SetText("❌ Version conflict — secret was updated externally. Re-read the secret before pushing.")
							return
						}
						statusLabel.SetError("%v", err)
						return
					}
					currentVault, currentSecret, currentVersion = vault, secret, newVersion
					isDirty = false
					dirtyLabel.SetText("")
					versionLabel.SetText(fmt.Sprintf("Version: %s", newVersion))
					statusLabel.SetText(fmt.Sprintf("✅ Pushed '%s' to '%s'", secret, vault))
				})
			}()
		}, w)
	}

	// ── Clear button ───────────────────────────────────────────────────────
	clearBtn.OnTapped = func() {
		doClear := func() {
			secretEntry.SetText("")
			editor.Clear()
			versionLabel.SetText("Version: —")
			dirtyLabel.SetText("")
			statusLabel.SetText("")
			currentVault, currentSecret, currentVersion = "", "", ""
			isDirty = false
		}
		if isDirty {
			dialog.ShowConfirm("Unsaved Changes",
				fmt.Sprintf("You have unsaved changes to '%s'. Discard them?", currentSecret),
				func(ok bool) {
					if ok {
						doClear()
					}
				}, w)
			return
		}
		doClear()
	}

	// ── Layout ─────────────────────────────────────────────────────────────
	secretRow := container.NewBorder(
		nil, nil,
		widget.NewLabel("Secret Name:"),
		container.NewHBox(readBtn, clearBtn),
		secretEntry,
	)

	infoRow := container.NewBorder(
		nil, nil,
		versionLabel,
		dirtyLabel,
		statusLabel.obj,
	)

	editorHeader := container.NewBorder(
		nil, nil,
		widget.NewLabel("Secret Value:"),
		container.NewHBox(editor.ToggleButton(), copyBtn),
		nil,
	)

	top := container.NewVBox(
		widget.NewSeparator(),
		container.NewPadded(secretRow),
		container.NewPadded(infoRow),
		widget.NewSeparator(),
		container.NewPadded(editorHeader),
	)

	bottom := container.NewVBox(
		widget.NewSeparator(),
		container.NewPadded(pushBtn),
	)

	return container.NewBorder(top, bottom, nil, nil, editor.Body())
}
