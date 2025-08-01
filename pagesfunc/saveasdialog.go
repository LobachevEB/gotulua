package pagesfunc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SaveAsDialog represents a dialog for saving a file with directory navigation
type SaveAsDialog struct {
	*tview.Flex
	dirList       *tview.List
	fileNameInput *tview.InputField
	currentPath   string
	onSave        func(filePath string) error
	onCancel      func()
	app           *tview.Application
}

// newSaveAsDialog creates a new save as dialog
func newSaveAsDialog(app *tview.Application, initialPath string, onSave func(filePath string) error, onCancel func()) *SaveAsDialog {
	dialog := &SaveAsDialog{
		Flex:          tview.NewFlex(),
		dirList:       tview.NewList(),
		fileNameInput: tview.NewInputField(),
		onSave:        onSave,
		onCancel:      onCancel,
		app:           app,
	}

	// Set up directory list
	dialog.dirList.SetBorder(true).SetTitle(" Directories ")
	dialog.dirList.ShowSecondaryText(false)
	dialog.dirList.SetSelectedBackgroundColor(tcell.ColorBlue)

	// Set up filename input
	dialog.fileNameInput.SetBorder(true).SetTitle(" File Name ")
	dialog.fileNameInput.SetFieldWidth(0)

	// Create buttons
	buttons := tview.NewFlex().SetDirection(tview.FlexRow)
	saveButton := tview.NewButton("Save").SetSelectedFunc(dialog.handleSave)
	cancelButton := tview.NewButton("Cancel").SetSelectedFunc(dialog.handleCancel)
	buttons.AddItem(saveButton, 1, 0, false)
	buttons.AddItem(cancelButton, 1, 0, false)

	// Layout
	rightSide := tview.NewFlex().SetDirection(tview.FlexRow)
	rightSide.AddItem(dialog.fileNameInput, 3, 0, true)
	rightSide.AddItem(buttons, 3, 0, false)

	dialog.Flex.SetDirection(tview.FlexRow)
	dialogContent := tview.NewFlex().
		AddItem(dialog.dirList, 0, 2, false).
		AddItem(rightSide, 0, 1, true)

	dialog.AddItem(dialogContent, 0, 1, true)
	dialog.SetBorder(true).SetTitle(" Save As ")

	// Set initial path and populate directory list
	if initialPath == "" {
		var err error
		initialPath, err = os.Getwd()
		if err != nil {
			initialPath = "."
		}
	}
	dialog.setPath(initialPath)

	// Handle keyboard navigation
	dialog.dirList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			dialog.app.SetFocus(dialog.fileNameInput)
			return nil
		case tcell.KeyEnter:
			if dialog.dirList.GetItemCount() > 0 {
				index := dialog.dirList.GetCurrentItem()
				text, _ := dialog.dirList.GetItemText(index)
				if text == ".." {
					dialog.setPath(filepath.Dir(dialog.currentPath))
				} else {
					dialog.setPath(filepath.Join(dialog.currentPath, text))
				}
			}
			return nil
		case tcell.KeyEscape:
			dialog.handleCancel()
			return nil
		}
		return event
	})

	dialog.fileNameInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			dialog.app.SetFocus(dialog.dirList)
			return nil
		case tcell.KeyEnter:
			dialog.handleSave()
			return nil
		case tcell.KeyEscape:
			dialog.handleCancel()
			return nil
		}
		return event
	})

	return dialog
}

// setPath updates the current path and refreshes the directory list
func (d *SaveAsDialog) setPath(path string) {
	d.currentPath = path
	d.dirList.Clear()

	// Add parent directory entry
	d.dirList.AddItem("..", "", 0, nil)

	// Read directory contents
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	// Add directories
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			d.dirList.AddItem(entry.Name(), "", 0, nil)
		}
	}

	d.SetTitle(fmt.Sprintf(" Save As - %s ", path))
}

func (d *SaveAsDialog) handleSave() {
	fileName := d.fileNameInput.GetText()
	if fileName == "" {
		return
	}

	// Construct full file path
	filePath := filepath.Join(d.currentPath, fileName)

	// Check if file exists
	if _, err := os.Stat(filePath); err == nil {
		// File exists, show confirmation dialog
		confirm := tview.NewModal().
			SetText(fmt.Sprintf("File '%s' already exists. Overwrite?", fileName)).
			AddButtons([]string{"Yes", "No"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				if buttonLabel == "Yes" {
					d.saveFile(filePath)
				}
			})
		d.app.SetRoot(confirm, true)
	} else {
		d.saveFile(filePath)
	}
}

func (d *SaveAsDialog) saveFile(filePath string) {
	if err := d.onSave(filePath); err != nil {
		// Show error modal
		modal := tview.NewModal().
			SetText(fmt.Sprintf("Error saving file: %v", err)).
			AddButtons([]string{"OK"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				d.app.SetRoot(d, true)
			})
		d.app.SetRoot(modal, true)
	} else {
		d.onCancel() // Close dialog on successful save
	}
}

func (d *SaveAsDialog) handleCancel() {
	if d.onCancel != nil {
		d.onCancel()
	}
}
