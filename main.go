package main

import (
	"flag"
	"fmt"
	"gotulua/errorhandlefunc"
	"gotulua/helpsysfunc"
	"gotulua/i18nfunc"
	"gotulua/luafunc"
	"gotulua/pagesfunc"
	"gotulua/statefunc"
	"gotulua/uifunc"
	"gotulua/view"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var App = view.NewApp()

func main() {
	// Initialize i18n with default language
	i18nfunc.InitI18n("en")

	App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		widget := App.GetFocus()
		switch widget.(type) {
		case *tview.InputField:
			// Let the InputField handle Esc
			if widget.(*tview.InputField).GetTitle() == "BROWSEINPUT" || widget.(*tview.InputField).GetTitle() == "BROWSEFILTER" {
				return event
			}
		case *tview.Modal:
			return event
		case *tview.TextArea:
			if widget.(*tview.TextArea).GetTitle() == "Find" && event.Key() == tcell.KeyEscape {
				return event
			}
		case *tview.Table:
			t := widget.(*tview.Table)
			tt := t.GetTitle()
			if tt == "lookup" && event.Key() == tcell.KeyEscape {
				return event
			}
		case *tview.Button:
			if event.Key() == tcell.KeyEscape {
				return event
			}
		case *tview.List:
			if event.Key() == tcell.KeyEscape {
				t := widget.(*tview.List)
				tt := t.GetTitle()
				if (tt == "File Menu" || tt == "Edit Menu" || tt == "Search Menu" || tt == "Help Menu" || tt == "Run Menu") && event.Key() == tcell.KeyEscape {
					return event
				}
			}
		}
		if event.Key() == tcell.KeyEscape {
			f := statefunc.PopVisual()
			if f != nil {
				statefunc.App.SetRoot(f, true)
				return event
			}
			App.Stop()
			return nil
		}
		return event
	})
	var err error
	doEdit := flag.Bool("e", false, "Edit mode")
	flag.Parse()
	args := flag.Args()
	var srcFile string
	if len(args) > 0 {
		srcFile = ".\\" + args[0]
	}
	mainFlex := tview.NewFlex()
	runFlexLevel0 := tview.NewFlex()
	pages := tview.NewPages().
		AddPage("main", mainFlex, true, true)
	statefunc.SetState(runFlexLevel0, mainFlex, pages, App)
	uifunc.SetUIData()
	L, _ := luafunc.CreateLuaInterpreter()
	statefunc.SetLuaState(L)
	luafunc.SetupRequireHandler(L, []string{"."})
	statefunc.RunLuaScriptFunc = luafunc.RunLuaScript
	statefunc.ShowHelpFunc = helpsysfunc.ShowHelp
	errorhandlefunc.SetLuaState(L)
	App.EnableMouse(true)
	App.SetRoot(pages, true)
	if *doEdit || srcFile == "" {
		pagesfunc.ShowEditor(srcFile, 0, "")
		statefunc.App.SetFocus(statefunc.MainFlex)
	} else {
		luafunc.RunLuaScript(srcFile)
	}

	err = App.Run()
	if err != nil {
		fmt.Println("Error running Application:", err)
	} else {
		fmt.Println("Application stopped successfully")
	}
}
