package uifunc

import (
	"gotulua/statefunc"

	"github.com/rivo/tview"
)

func Confirm(text string, callback func(bool)) {
	dialog := tview.NewModal()
	dialog.SetText(text)
	dialog.AddButtons([]string{"OK", "Cancel"})
	dialog.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		statefunc.ShowPreviousVisual()
		callback(buttonIndex == 0)
	})
	statefunc.PushVisual(statefunc.RunFlexLevel0)
	statefunc.RunFlexLevelDialog.Clear()
	statefunc.RunFlexLevelDialog.AddItem(dialog, 0, 1, false)
	statefunc.App.SetRoot(statefunc.RunFlexLevelDialog, true)
	statefunc.App.SetFocus(dialog)
	statefunc.App.ForceDraw() // Ensure the dialog is drawn immediately
}

func Message(text string) {
	dialog := tview.NewModal()
	dialog.SetText(text)
	dialog.AddButtons([]string{"OK"})
	dialog.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if statefunc.IsNowOnInitialTop() && statefunc.IsRunAsScript() {
			statefunc.ShowMainVisual()
			return
		}
		statefunc.ShowPreviousVisual()
	})
	statefunc.PushVisual(statefunc.RunFlexLevel0)
	statefunc.RunFlexLevelDialog.Clear()
	statefunc.RunFlexLevelDialog.AddItem(dialog, 1, 0, false)
	statefunc.App.SetRoot(statefunc.RunFlexLevelDialog, true)
	statefunc.App.SetFocus(dialog)
	statefunc.App.ForceDraw() // Ensure the dialog is drawn immediately
}
