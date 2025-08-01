package inputfunc

import "github.com/rivo/tview"

func SetBoolInput(input *tview.InputField) {
	//input.SetAcceptanceFunc(tview.InputFieldCheckbox)
	input.SetAutocompleteFunc(func(currentText string) (entries []string) {
		return []string{"true", "false"}
	})
}
