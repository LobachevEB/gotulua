package pagesfunc

import (
	"gotulua/statefunc"

	"github.com/rivo/tview"
)

func ErrorMessage(text string) {
	dialog := tview.NewModal()
	dialog.SetText(text)
	dialog.AddButtons([]string{"OK"})
	dialog.SetTextColor(tview.Styles.PrimaryTextColor)
	dialog.SetBackgroundColor(tview.Styles.ContrastBackgroundColor)
	dialog.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		statefunc.RunFlexLevelDialog.Clear()
		statefunc.ShowRunVisual()
	})
	statefunc.PushVisual(statefunc.RunFlexLevel0)
	statefunc.RunFlexLevelDialog.Clear()
	statefunc.RunFlexLevelDialog.AddItem(dialog, 1, 0, false)
	statefunc.App.SetRoot(statefunc.RunFlexLevelDialog, true)
	statefunc.App.SetFocus(dialog)
	statefunc.App.ForceDraw() // Ensure the dialog is drawn immediately
}
