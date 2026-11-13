package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/eugenegoncharuk/keyvault-manager/azure"
)

// NewHistoryTab builds the "History" tab.
// It is self-contained — no dependency on the Manage tab's state.
func NewHistoryTab(vaultSelect *widget.Select, w fyne.Window) fyne.CanvasObject {
	secretEntry := widget.NewEntry()
	secretEntry.PlaceHolder = "Enter secret name…"

	statusLabel := newStatusLine(w)

	var versions []azure.SecretVersion
	// The vault/secret the current version list belongs to. Row clicks use
	// these, not the live inputs, which may have been edited since loading.
	var loadedVault, loadedSecret string

	// ── Version table ──────────────────────────────────────────────────────
	versionTable := widget.NewTable(
		func() (rows, cols int) { return len(versions) + 1, 4 },
		func() fyne.CanvasObject {
			lbl := widget.NewLabel("")
			lbl.TextStyle = fyne.TextStyle{Monospace: true}
			return lbl
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			lbl := cell.(*widget.Label)
			if id.Row == 0 {
				lbl.TextStyle = fyne.TextStyle{Bold: true}
				switch id.Col {
				case 0:
					lbl.SetText("Version")
				case 1:
					lbl.SetText("Updated (UTC)")
				case 2:
					lbl.SetText("Status")
				case 3:
					lbl.SetText("Full Version ID")
				}
				return
			}
			if id.Row-1 >= len(versions) {
				lbl.SetText("")
				return
			}
			v := versions[id.Row-1]
			lbl.TextStyle = fyne.TextStyle{Monospace: true}
			switch id.Col {
			case 0:
				s := v.Version
				if len(s) > 8 {
					s = s[:8] + "…"
				}
				lbl.SetText(s)
			case 1:
				lbl.SetText(v.Updated.UTC().Format("2006-01-02  15:04:05"))
			case 2:
				if v.Enabled {
					lbl.SetText("enabled")
				} else {
					lbl.SetText("disabled")
				}
			case 3:
				lbl.SetText(v.Version)
			}
			lbl.Refresh()
		},
	)
	versionTable.SetColumnWidth(0, 100)
	versionTable.SetColumnWidth(1, 185)
	versionTable.SetColumnWidth(2, 75)
	versionTable.SetColumnWidth(3, 350)

	// ── Value preview panel ────────────────────────────────────────────────
	valuePreview := widget.NewRichText()
	valuePreview.Wrapping = fyne.TextWrapBreak

	previewStatus := newStatusLine(w)
	previewStatus.SetText("← Select a version to preview its value")

	copyPreviewBtn := widget.NewButton("📋  Copy", nil)
	copyPreviewBtn.Disable()

	var previewText string
	var previewSeq int // bumped on each preview request; main goroutine only

	copyPreviewBtn.OnTapped = func() {
		if previewText == "" {
			return
		}
		w.Clipboard().SetContent(previewText)
		previewStatus.SetText("📋 Copied to clipboard")
	}

	// ── Row click: load that version's value ──────────────────────────────
	versionTable.OnSelected = func(id widget.TableCellID) {
		if id.Row < 1 || id.Row-1 >= len(versions) {
			return
		}
		v := versions[id.Row-1]
		vault, secret := loadedVault, loadedSecret

		previewStatus.SetText(fmt.Sprintf("⏳ Loading version %s…", v.Version))
		copyPreviewBtn.Disable()

		previewSeq++
		seq := previewSeq
		go func() {
			value, err := azure.GetSecretByVersion(vault, secret, v.Version)
			segs := HighlightContent(value)
			fyne.Do(func() {
				// Ignore results superseded by a later row click.
				if seq != previewSeq {
					return
				}
				if err != nil {
					previewStatus.SetError("%v", err)
					return
				}
				previewText = value
				valuePreview.Segments = segs
				valuePreview.Refresh()
				copyPreviewBtn.Enable()
				ts := v.Updated.UTC().Format("2006-01-02 15:04:05 UTC")
				previewStatus.SetText(fmt.Sprintf("Version %s  ·  %s", v.Version, ts))
			})
		}()
	}

	// ── Load History button ────────────────────────────────────────────────
	loadBtn := widget.NewButton("📜  Load History", nil)
	loadBtn.Importance = widget.HighImportance

	loadBtn.OnTapped = func() {
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

		loadBtn.Disable()
		statusLabel.SetText("⏳ Loading history…")
		valuePreview.Segments = nil
		valuePreview.Refresh()
		previewText = ""
		previewStatus.SetText("← Select a version to preview its value")
		copyPreviewBtn.Disable()

		previewSeq++ // invalidate any in-flight preview of the old list
		go func() {
			vers, err := azure.ListSecretVersions(vault, secret)
			fyne.Do(func() {
				loadBtn.Enable()
				if err != nil {
					statusLabel.SetError("%v", err)
					return
				}
				versions = vers
				loadedVault, loadedSecret = vault, secret
				versionTable.UnselectAll()
				versionTable.Refresh()
				statusLabel.SetText(fmt.Sprintf("✅ %d version(s) for '%s'", len(vers), secret))
			})
		}()
	}

	// ── Layout ─────────────────────────────────────────────────────────────
	secretRow := container.NewBorder(
		nil, nil,
		widget.NewLabel("Secret Name:"),
		loadBtn,
		secretEntry,
	)

	top := container.NewVBox(
		widget.NewSeparator(),
		container.NewPadded(secretRow),
		container.NewPadded(statusLabel.obj),
		widget.NewSeparator(),
	)

	previewHeader := container.NewBorder(
		nil, nil,
		widget.NewLabel("Value:"),
		copyPreviewBtn,
		previewStatus.obj,
	)
	previewPanel := container.NewBorder(
		container.NewPadded(previewHeader),
		nil, nil, nil,
		container.NewScroll(valuePreview),
	)

	split := container.NewHSplit(versionTable, previewPanel)
	split.Offset = 0.45

	return container.NewBorder(top, nil, nil, nil, split)
}
