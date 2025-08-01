package pagesfunc

import (
	"gotulua/editorfunc"
	"gotulua/statefunc"
)

var Editor *editorfunc.LuaEditor

func ShowEditor(path string, line int, statusMsg string) {
	Editor = editorfunc.NewLuaEditor(statefunc.App, "", path, nil)
	if line > 0 {
		Editor.GoToAndHighlightLine(line)
	}
	Editor.SetMouseSupport()
	flex := AddMainMenuToEditor(Editor, Editor.GetStatusBar(), statefunc.App)
	statefunc.MainFlex.AddItem(flex, 0, 1, true)
}

func SwitchToEditor(path string, line int, statusMsg string, isError bool) {
	if line > 0 {
		Editor.GoToAndHighlightLine(line)
	}
	if isError {
		Editor.SetErrorStatus(statusMsg)
		Editor.SetHighlightedLine(line, editorfunc.IsErrorHighlight)
	} else {
		Editor.SetStatus(statusMsg)
	}
	statefunc.App.SetRoot(statefunc.MainFlex, true)
}
