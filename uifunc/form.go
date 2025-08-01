package uifunc

import (
	"gotulua/errorhandlefunc"
	"gotulua/i18nfunc"
	"gotulua/inputfunc"
	"gotulua/statefunc"
	"gotulua/timefunc"

	"github.com/Shopify/go-lua"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Form struct {
	Title string
	Form  *tview.Form
}

var Forms map[string]*Form = make(map[string]*Form)

func AddForm(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args", map[string]interface{}{
			"Name": "AddForm",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	caption, ok := L.ToString(1) // Get the caption from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.second_arg_not_string", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	//isLookup := L.ToBoolean(3)
	// Create a new Form object
	form := &Form{
		Title: caption,
		Form:  tview.NewForm(),
	}
	form.Form.SetTitle(caption) // Set the title of the Form
	form.Form.SetBorder(true)   // Optional: Set a border around the form
	// form.Form.SetMouseCapture(mouseCapture)
	// form.Form.SetInputCapture(inputCapture)

	Forms[caption] = form // Store the Form in the Forms map
	L.PushUserData(form)  // Push the Form object as userdata
	L.Global("FormMT")    // Push the metatable
	L.SetMetaTable(-2)    // Set metatable for userdata
	return 1              // Return the number of results
}

func FormShow(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args", map[string]interface{}{
			"Name": "Show",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	form, ok := L.ToUserData(1).(*Form) // Get the Form from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.first_argument_not_form", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	form.show()
	return 0 // Return the number of results
}

// show shows the form on the screen.
func (form *Form) show() {
	statefunc.SetRunMode(statefunc.RunAsForm) // Set the run mode to Form
	AddWidget(form.Form, form.Title, nil)     // Add the Form to the Widgets list
}

func FormHide(L *lua.State) int {
	//TODO: Implement the Hide function
	return 1
}

// addInput adds an input field to the form with the given title, type, and callback function.
//
// The callback function is called when the input field is changed.
func (form *Form) addInput(title string, typeName string, callback string) {
	if InputFields == nil {
		InputFields = []InputField{}
	}
	InputFields = append(InputFields, InputField{
		//InputField: input,
		Caption:  title,
		Type:     typeName,
		callback: callback,
	})
	var af func(text string, ch rune) bool = nil // Acceptance function for the input field
	var defVal string = ""
	var needPH bool // Default value for the input field
	switch typeName {
	case "N":
		af = tview.InputFieldFloat // Set acceptance function for float input
		defVal = "0.0"             // Default value for float input
	case "I":
		af = tview.InputFieldInteger // Set acceptance function for integer input
		defVal = "0"                 // Default value for integer input
	case "B":
		defVal = "false" // Default value for boolean input
	case "D", "T", "DT":
		needPH = true // Date and time inputs need a placeholder
	}
	form.Form.AddInputField(title, defVal, 0, af, nil)
	var input *tview.InputField = CurrForm.Form.GetFormItem(CurrForm.Form.GetFormItemCount() - 1).(*tview.InputField)
	input.SetDoneFunc(onDone)
	input.SetMouseCapture(mouseCapture)
	input.SetInputCapture(inputCapture)
	//input.SetChangedFunc()                                                                            // Set the done function for the InputField
	if needPH {
		switch typeName {
		case "D":
			ph := timefunc.TemplateToPlaceholder(timefunc.DateFormat)
			input.SetPlaceholder(ph)
			input.SetPlaceholderTextColor(tcell.ColorYellow)
			inputfunc.SetDateInput(input, ph)
		case "T":
			ph := timefunc.TemplateToPlaceholder(timefunc.TimeFormat)
			input.SetPlaceholder(ph)
			input.SetPlaceholderTextColor(tcell.ColorYellow)
			inputfunc.SetDateInput(input, ph)
		case "DT":
			ph := timefunc.TemplateToPlaceholder(timefunc.DateTimeFormat)
			input.SetPlaceholder(ph)
			input.SetPlaceholderTextColor(tcell.ColorYellow)
			inputfunc.SetDateInput(input, ph)
		}
	}

	// Attach input capture to block mouse focus change if invalid
	form.Form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Allow all keyboard events (Tab/Enter handled in onDone)
		return event
	})
}

func FormAddButton(L *lua.State) int {
	if L.Top() < 2 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args", map[string]interface{}{
			"Name": "AddButton",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	form, ok := L.ToUserData(1).(*Form) // Get the Form from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.first_argument_not_form", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	buttonText, ok := L.ToString(2) // Get the button text from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.second_arg_not_string", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	buttonFuncName, ok := L.ToString(3) // Get the button function from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.third_arg_not_string", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	statefunc.L.Global(buttonFuncName) // Get the function from the Lua global state
	if !statefunc.L.IsFunction(-1) {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_a_function", map[string]interface{}{
			"Name": buttonFuncName,
		}), errorhandlefunc.ErrorTypeScript, true)
		L.Pop(1)
		return 0
	}
	L.Pop(1)

	form.addButton(buttonText, buttonFuncName) // Add the button to the Form
	return 1                                   // Return the number of results
}

// addButton adds a button to the form with the given text and callback function.
//
// The callback function is called when the button is clicked.
func (form *Form) addButton(buttonText string, callback string) {
	form.Form.AddButton(buttonText, func() {
		defer func() {
			if r := recover(); r != nil {
				errorhandlefunc.ThrowError(r.(string), errorhandlefunc.ErrorTypeScript, true)
			}
		}()
		statefunc.L.Global(callback) // Get the function from the Lua global state
		statefunc.L.Call(0, 0)
	})
}

func mouseCapture(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
	fi, _ := CurrForm.Form.GetFocusedItemIndex()
	if action == tview.MouseLeftClick || action == tview.MouseRightClick || action == tview.MouseLeftDown || action == tview.MouseRightDown {
		if !isCurrentInputValid(CurrForm.Form, fi) {
			// Block mouse focus change if invalid
			return tview.MouseConsumed, nil
		}
	}
	return action, event
}

func inputCapture(event *tcell.EventKey) *tcell.EventKey {
	fi, _ := CurrForm.Form.GetFocusedItemIndex()
	if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab || event.Key() == tcell.KeyEnter {
		if !isCurrentInputValid(CurrForm.Form, fi) {
			errorhandlefunc.ThrowError(i18nfunc.T("error.invalid_input", nil), errorhandlefunc.ErrorTypeData, false)
			// Block keyboard focus change if invalid
			return nil
		}
	}
	// Allow all keyboard events (Tab/Enter handled in onDone)
	return event
}
