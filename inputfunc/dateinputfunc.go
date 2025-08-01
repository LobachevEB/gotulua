package inputfunc

import (
	"github.com/rivo/tview"
)

var Input *tview.InputField

// TODO: Use it or trash it

func SetDateInput(input *tview.InputField, template string) {
	pr := []rune(template)
	input.SetAcceptanceFunc(func(textToCheck string, lastChar rune) bool {
		for i, r := range textToCheck {
			if i >= len(pr) {
				return false
			}
			if pr[i] == ' ' {
				if r < '0' || r > '9' {
					return false
				}
			} else {
				if r != pr[i] {
					return false
				}
			}
		}
		return true
	})
}
