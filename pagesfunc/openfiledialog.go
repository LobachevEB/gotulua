package pagesfunc

import (
	"gotulua/statefunc"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var flex *tview.Flex

// showOpenFileDialog displays a dialog to choose and open a file.
// It lists files matching *.lua and *.* in the given directory.
func showOpenFileDialog(currentDir string, onOpen func(string)) {
	fileList := tview.NewList()
	fileList.SetBorder(true).SetTitle("Open File")

	// Add key handler: Ctrl+Enter closes dialog and returns selected file path
	fileList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			index := fileList.GetCurrentItem()
			mainText, _ := fileList.GetItemText(index)
			// Only trigger if a file is selected (not a label or drive)
			if !strings.HasPrefix(mainText, "[") && strings.HasSuffix(mainText, ".lua") {
				selectedPath := filepath.Join(currentDir, mainText)
				statefunc.App.SetRoot(statefunc.MainFlex, true)
				flex = nil
				onOpen(selectedPath)
				return nil
			}
			return event // consume event
		}
		if event.Key() == tcell.KeyEscape {
			statefunc.App.SetRoot(statefunc.MainFlex, true)
			flex = nil
		}
		return event
	})

	// To choose the disk, select one of the listed drives (e.g., "C:\", "D:\") when the dialog shows "[DRIVES]".
	// The following helper lists available Windows drives:
	getWindowsDrives := func() []string {
		drives := []string{}
		for _, letter := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
			drive := string(letter) + ":\\"
			if _, err := os.Stat(drive); err == nil {
				drives = append(drives, drive)
			}
		}
		return drives
	}

	var refreshList func(dir string)
	refreshList = func(dir string) {
		fileList.Clear()

		// On Windows, if dir is empty or root, show drives
		isWindows := false
		if os.PathSeparator == '\\' {
			isWindows = true
		}

		// Show drives if at the "root" on Windows
		if isWindows && (dir == "" || dir == "\\" || dir == "/" || len(dir) == 2 && dir[1] == ':') {
			fileList.AddItem("[::b][DRIVES][-:-:-]", "", 0, func() {})
			for _, drive := range getWindowsDrives() {
				driveCopy := drive // capture for closure
				fileList.AddItem("[::b][DISK][-:-:-] "+drive, "", 0, func() {
					refreshList(driveCopy)
					currentDir = driveCopy
				})
			}
			return
		}

		files, err := os.ReadDir(dir)
		if err != nil {
			fileList.AddItem("[red]Error reading directory", "", 0, nil)
			// On Windows, allow going back to drive selection if error
			if isWindows {
				fileList.AddItem("[::b]Back to Drives", "", 0, func() {
					refreshList("")
					currentDir = ""
				})
			}
			return
		}
		var entries []os.DirEntry
		for _, f := range files {
			// Show directories and files matching *.lua or *.*
			if f.IsDir() || strings.HasSuffix(strings.ToLower(f.Name()), ".lua") || f.Name() == "." || f.Name() == ".." {
				entries = append(entries, f)
			}
		}
		// Sort: directories first, then files
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].IsDir() && !entries[j].IsDir() {
				return true
			}
			if !entries[i].IsDir() && entries[j].IsDir() {
				return false
			}
			return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
		})

		// Add ".." to go up a directory
		// On Windows, if at drive root (e.g. C:\), ".." goes to drive selection
		if isWindows {
			vol := filepath.VolumeName(dir)
			rest := strings.TrimPrefix(dir, vol)
			rest = strings.Trim(rest, "\\/")
			if vol != "" && (rest == "" || rest == ".") {
				fileList.AddItem("../ (Drives)", "", 0, func() {
					refreshList("")
					currentDir = ""
				})
			} else if dir != "" && dir != "\\" && dir != "/" {
				fileList.AddItem("../", "", 0, func() {
					parent := filepath.Dir(dir)
					refreshList(parent)
					currentDir = parent
				})
			}
		} else {
			// On Unix, only if not at root
			if dir != "/" {
				fileList.AddItem("../", "", 0, func() {
					parent := filepath.Dir(dir)
					refreshList(parent)
					currentDir = parent
				})
			}
		}

		for _, entry := range entries {
			name := entry.Name()
			fullPath := filepath.Join(dir, name)
			if entry.IsDir() {
				fileList.AddItem("[::b][DIR][-:-:-] "+name, "", 0, func(p string) func() {
					return func() {
						refreshList(p)
						currentDir = p
					}
				}(fullPath))
			} else if strings.HasSuffix(strings.ToLower(name), ".lua") || strings.Contains(name, ".") {
				fileList.AddItem(name, "", 0, func(p string) func() {
					return func() {
						onOpen(p)
						statefunc.App.SetRoot(nil, true)
					}
				}(fullPath))
			}
		}
	}

	refreshList(currentDir)

	// Wrap in a Flex for better layout
	flex = tview.NewFlex().
		AddItem(fileList, 0, 1, true)
	statefunc.PushVisual(statefunc.MainFlex)
	statefunc.App.SetRoot(flex, true).SetFocus(fileList)
}
