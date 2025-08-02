package pagesfunc

import (
	"fmt"
	"gotulua/i18nfunc"
	"gotulua/statefunc"
	"os"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// MainMenu represents the main menu bar for the editor.
type MainMenu struct {
	*tview.Flex
	menuBar      *tview.TextView
	findTextArea *tview.TextArea
	findTextView *tview.TextView
	findFlex     *tview.Flex
	findFunc     func(string, bool)
	menus        []string
	selected     int
	callbacks    []func()
}

// newMainMenu creates a new main menu bar with the given menu items and callbacks.
func newMainMenu(menus []string, findFunc func(string, bool), callbacks []func()) *MainMenu {
	menuBar := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)
	m := &MainMenu{
		Flex:      statefunc.MainMenuFlex,
		menuBar:   menuBar,
		menus:     menus,
		selected:  0,
		callbacks: callbacks,
	}
	m.updateMenuBar()
	m.findFlex = tview.NewFlex().SetDirection(tview.FlexColumn)
	m.findFunc = findFunc
	m.SetInputCapture(m.inputHandler)
	m.AddItem(menuBar, 0, 4, true)
	m.AddItem(m.findFlex, 0, 1, false)
	return m
}

// updateMenuBar updates the visual representation of the menu bar.
func (m *MainMenu) updateMenuBar() {
	m.menuBar.Clear()
	for i, menu := range m.menus {
		if i > 0 {
			fmt.Fprint(m.menuBar, "  ")
		}
		if i == m.selected {
			fmt.Fprintf(m.menuBar, `[black:yellow]%s[-:-]`, menu)
		} else {
			fmt.Fprintf(m.menuBar, `[white]%s[-]`, menu)
		}
	}
}

// inputHandler handles keyboard navigation for the menu bar.
func (m *MainMenu) inputHandler(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyLeft:
		if m.selected > 0 {
			m.selected--
			m.updateMenuBar()
		}
		return nil
	case tcell.KeyRight:
		if m.selected < len(m.menus)-1 {
			m.selected++
			m.updateMenuBar()
		}
		return nil
	case tcell.KeyEnter:
		if m.findTextArea != nil {
			ft := m.findTextArea.GetText()
			m.findFlex.RemoveItem(m.findTextArea)
			m.findTextArea = nil
			m.findTextView = tview.NewTextView().SetDynamicColors(true)
			m.findTextView.SetLabel("Find: ")
			m.findTextView.SetText(ft)
			m.findFlex.AddItem(m.findTextView, 0, 1, true)
			statefunc.App.SetFocus(statefunc.EditorFlex)
			m.findFunc(ft, false)
			return nil
		}
		if m.selected >= 0 && m.selected < len(m.callbacks) && m.callbacks[m.selected] != nil {
			m.callbacks[m.selected]()
		}
		return nil
	case tcell.KeyF3, tcell.KeyF4:
		if m.findTextArea != nil || m.findTextView != nil {
			ft := ""
			if m.findTextArea != nil {
				ft = m.findTextArea.GetText()
				m.findFlex.RemoveItem(m.findTextArea)
				m.findTextArea = nil
				if m.findTextView == nil {
					m.findTextView = tview.NewTextView().SetDynamicColors(true)
					m.findTextView.SetLabel("Find: ")
					m.findTextView.SetText(ft)
					m.findFlex.AddItem(m.findTextView, 0, 1, true)
				}
			} else if m.findTextView != nil {
				ft = m.findTextView.GetText(true)
			}
			statefunc.App.SetFocus(statefunc.EditorFlex)
			m.findFunc(ft, true)
			return nil
		}
	case tcell.KeyEscape:
		if m.findTextArea != nil {
			m.findFlex.RemoveItem(m.findTextArea)
			m.findTextArea = nil
		}
		statefunc.App.SetRoot(statefunc.MainFlex, true)
		return nil
	case tcell.KeyF5:
		statefunc.PushVisual(statefunc.MainFlex)
		statefunc.App.SetRoot(statefunc.RunFlexLevel0, true)
		statefunc.StartScript(statefunc.L, Editor.GetFileName(), statefunc.RunLuaScriptFunc)
		return nil
	case tcell.KeyF1, tcell.KeyF2:
		if statefunc.ShowHelpFunc != nil {
			statefunc.PushVisual(statefunc.MainFlex)
			statefunc.ShowHelpFunc(false, nil)
		}
		return nil
	}
	return event
}

// AddMainMenuToEditor adds the main menu to the top of the editor layout.
func AddMainMenuToEditor(editor tview.Primitive, statusBar tview.Primitive, app *tview.Application) tview.Primitive {
	menus := []string{
		i18nfunc.T("menu.file", nil),
		i18nfunc.T("menu.run", nil),
		i18nfunc.T("menu.help", nil),
	}
	callbacks := []func(){
		func() { showFileMenu(app) },
		func() { showRunMenu(app) },
		func() { showHelpMenu(app) },
	}
	mainMenu := newMainMenu(menus, Editor.FindText, callbacks)
	statefunc.EditorFlex.AddItem(editor, 0, 1, true)
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(mainMenu, 1, 0, true).
		AddItem(statefunc.EditorFlex, 0, 1, false)
	if statusBar != nil {
		flex.AddItem(statusBar, 1, 0, false)
	}
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyF10:
			if statefunc.MainMenuFlex.HasFocus() {
				statefunc.App.SetFocus(statefunc.EditorFlex)
			} else {
				statefunc.App.SetFocus(statefunc.MainMenuFlex)
			}
		case tcell.KeyCtrlF, tcell.KeyCtrlU:
			if mainMenu.findTextView != nil {
				mainMenu.findFlex.RemoveItem(mainMenu.findTextView)
				mainMenu.findTextView = nil
			}
			if mainMenu.findTextArea == nil {
				mainMenu.findTextArea = tview.NewTextArea()
				mainMenu.findTextArea.SetLabel("Find: ")
				mainMenu.findTextArea.SetWrap(false)
				mainMenu.findTextArea.SetTitle("Find")
				mainMenu.findTextArea.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
					switch event.Key() {
					case tcell.KeyEscape:
						mainMenu.findFlex.RemoveItem(mainMenu.findTextArea)
						mainMenu.findTextArea = nil
						statefunc.App.SetFocus(mainMenu.menuBar)
					}
					return event
				})
				mainMenu.findFlex.AddItem(mainMenu.findTextArea, 0, 1, true)
			}
			statefunc.App.SetFocus(mainMenu.findTextArea)
			go func() {
				statefunc.App.QueueUpdateDraw(func() {
					statefunc.App.SetFocus(mainMenu.findTextArea)
				})
			}()
		}
		return event
	})
	return flex
}

// Example menu callback implementations (can be replaced with real dialogs)
func showFileMenu(app *tview.Application) {
	// Create a Flex to act as a drop-down menu container
	// We'll use a Flex to hold the List, so it can be extended for more complex layouts if needed.
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	list := tview.NewList()
	list.AddItem(i18nfunc.T("action.open", nil), i18nfunc.T("prompt.open", nil), 'o', func() {
		exe := getExeDirectory()
		showOpenFileDialog(exe, func(p string) {
			Editor.OpenFile(p)
			statefunc.App.SetRoot(statefunc.MainFlex, true)
		})
	}).
		AddItem(i18nfunc.T("action.save", nil), i18nfunc.T("prompt.file", nil), 's', func() {
			if Editor.GetFileName() != "" {
				Editor.SaveFile()
			} else {
				showSaveAsDialog(app)
			}
			statefunc.App.SetRoot(statefunc.MainFlex, true)
		}).
		AddItem(i18nfunc.T("action.saveas", nil), i18nfunc.T("prompt.file", nil), 'a', func() {
			showSaveAsDialog(app)
		})

	list.SetBorder(true).SetTitle("File Menu")
	flex.AddItem(list, 0, 1, true)
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			statefunc.App.SetRoot(statefunc.MainFlex, true)
		}
		return event
	})
	statefunc.App.SetRoot(flex, true)
}

func showSaveAsDialog(app *tview.Application) {
	dlg := newSaveAsDialog(statefunc.App, ".", func(p string) error {
		Editor.SetFileName(p)
		Editor.SaveFile()
		statefunc.App.SetRoot(statefunc.MainFlex, true)
		return nil
	}, func() {
		statefunc.App.SetRoot(statefunc.MainFlex, true)
	})
	statefunc.App.SetRoot(dlg, true)
}

func getExeDirectory() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// RunLuaScriptFunc is an optional callback that external packages (e.g., luafunc) can
// assign to execute a Lua script.  We keep it here to avoid an import cycle: pagesfunc
// no longer needs to import luafunc.
//var RunLuaScriptFunc func(string) error

func showRunMenu(app *tview.Application) {
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	list := tview.NewList().
		AddItem(i18nfunc.T("action.run", nil), i18nfunc.T("prompt.run", nil), 'r', func() {
			if statefunc.RunLuaScriptFunc != nil {
				statefunc.PushVisual(statefunc.MainFlex)
				statefunc.App.SetRoot(statefunc.RunFlexLevel0, true)
				statefunc.StartScript(statefunc.L, Editor.GetFileName(), statefunc.RunLuaScriptFunc)
				//_ = RunLuaScriptFunc(Editor.GetFileName())
			}
		})
	list.SetBorder(true).SetTitle(i18nfunc.T("menu.run.title", nil))
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			statefunc.App.SetRoot(statefunc.MainFlex, true)
		}
		return event
	})
	flex.AddItem(list, 0, 1, true)
	statefunc.App.SetRoot(flex, true)
}

func showEditMenu(app *tview.Application) {
	modal := tview.NewModal().
		SetText(fmt.Sprintf("%s\n\n[%s] [%s] [%s] [%s] [%s]",
			i18nfunc.T("menu.edit.title", nil),
			i18nfunc.T("action.undo", nil),
			i18nfunc.T("action.redo", nil),
			i18nfunc.T("action.cut", nil),
			i18nfunc.T("action.copy", nil),
			i18nfunc.T("action.paste", nil))).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(i int, s string) {
			statefunc.Pages.RemovePage("mainmenu_modal")
		})
	statefunc.Pages.AddPage("mainmenu_modal", modal, true, true)
}

func showSearchMenu(app *tview.Application) {
	modal := tview.NewModal().
		SetText(fmt.Sprintf("%s\n\n[%s] [%s]",
			i18nfunc.T("menu.search.title", nil),
			i18nfunc.T("action.find", nil),
			i18nfunc.T("action.replace", nil))).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(i int, s string) {
			statefunc.Pages.RemovePage("mainmenu_modal")
		})
	statefunc.Pages.AddPage("mainmenu_modal", modal, true, true)
}

//var ShowHelpFunc func(func(string))

func showHelpMenu(app *tview.Application) {
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	list := tview.NewList().
		AddItem(i18nfunc.T("menu.help", nil), i18nfunc.T("prompt.help", nil), 'h', func() {
			statefunc.PushVisual(statefunc.MainFlex)
			if statefunc.ShowHelpFunc != nil {
				statefunc.ShowHelpFunc(false, nil)
			}
		})
	list.SetBorder(true).SetTitle(i18nfunc.T("menu.run.title", nil))
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			statefunc.App.SetRoot(statefunc.MainFlex, true)
		}
		return event
	})
	flex.AddItem(list, 0, 1, true)
	statefunc.App.SetRoot(flex, true)

}
