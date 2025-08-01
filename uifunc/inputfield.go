package uifunc

import (
	"errors"
	"gotulua/errorhandlefunc"
	"gotulua/i18nfunc"
	"gotulua/statefunc"
	"gotulua/timefunc"
	"strconv"

	"github.com/Shopify/go-lua"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type InputField struct {
	InputField *tview.InputField
	Caption    string
	callback   string
	Type       string
	Value      interface{}
}

// type Widget struct {
// 	WidgetTitle string
// 	Widget      tview.Primitive // Use tview.Primitive to allow any tview widget
// 	AcceptFocus bool            // Indicates if the widget accepts focus
// }

var CurrForm *Form // Current form being edited

var InputFields []InputField

//var Widgets []Widget

func AddInputField(L *lua.State) int {
	var err error = nil
	if L.Top() < 4 {
		err = errors.Join(errors.New(i18nfunc.T("error.input_field_args", nil)))
	}
	form, ok := L.ToUserData(1).(*Form) // Get the form from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.first_argument_not_form", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	CurrForm = form            // Set the current form to the one passed from Lua
	title, ok := L.ToString(2) // Get the input field title from Lua
	if !ok {
		err = errors.Join(errors.New(i18nfunc.T("error.input_field_title", nil)))
	}
	typeName, ok := L.ToString(3) // Get the input field type from Lua
	if !ok {
		err = errors.Join(errors.New(i18nfunc.T("error.input_field_type", nil)))
	}
	if typeName != "I" && typeName != "N" && typeName != "S" && typeName != "B" && typeName != "D" && typeName != "T" && typeName != "DT" {
		err = errors.Join(errors.New(i18nfunc.T("error.input_field_type_invalid", nil)))
	}
	callback, ok := L.ToString(4)
	if !ok {
		err = errors.Join(errors.New(i18nfunc.T("error.input_field_callback", nil)))
	}
	if err != nil {
		errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	form.addInput(title, typeName, callback)
	return 1 // Return the number of results
}

// Validation helper: returns true if current input is valid
func isCurrentInputValid(form *tview.Form, fi int) bool {
	if fi < 0 {
		return true
	}
	inp, ok := form.GetFormItem(fi).(*tview.InputField)
	if !ok {
		return true
	}
	text := inp.GetText()
	vType := InputFields[fi].Type
	switch vType {
	case "D", "T", "DT":
		if text != "" {
			var m string
			switch vType {
			case "D":
				m = timefunc.DateFormat
			case "T":
				m = timefunc.TimeFormat
			default:
				m = timefunc.DateTimeFormat
			}
			_, err := timefunc.FormatDateTime(text, m, timefunc.ToInternalFormat)
			return err == nil
		}
	case "N":
		_, err := strconv.ParseFloat(text, 64)
		return err == nil
	case "I":
		_, err := strconv.Atoi(text)
		return err == nil
	case "B":
		_, err := strconv.ParseBool(text)
		return err == nil
	}
	return true
}

func onDone(key tcell.Key) {
	var inp *tview.InputField
	fi, _ := CurrForm.Form.GetFocusedItemIndex()
	if fi < 0 {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			errorhandlefunc.ThrowError(r.(string), errorhandlefunc.ErrorTypeScript, true)
		}
	}()
	inp = CurrForm.Form.GetFormItem(fi).(*tview.InputField) // Get the currently focused InputField
	if inp == nil {
		errorhandlefunc.ThrowError(i18nfunc.T("error.input_field_not_exists", map[string]interface{}{
			"Name": fi,
		}), errorhandlefunc.ErrorTypeScript, true)
		return
	}

	if key == tcell.KeyEsc || key == tcell.KeyEscape || key == tcell.KeyESC {
		inp.SetText("") // Clear the input field on Escape key
		return
	}
	title := inp.GetTitle()
	text := inp.GetText()
	vType := InputFields[fi].Type // Get the text from the InputField
	eventname := InputFields[fi].callback
	statefunc.L.Global(eventname)
	if !statefunc.L.IsFunction(-1) {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_a_function", map[string]interface{}{
			"Name": eventname,
		}), errorhandlefunc.ErrorTypeScript, true)
		return
	}
	statefunc.L.PushString(title)
	switch vType {
	case "S":
		statefunc.L.PushString(text)
	case "D", "T", "DT":
		if text != "" {
			var m string
			switch vType {
			case "D":
				m = timefunc.DateFormat
			case "T":
				m = timefunc.TimeFormat
			default:
				m = timefunc.DateTimeFormat
			}
			_, err := timefunc.FormatDateTime(text, m, timefunc.ToInternalFormat)
			if err != nil {
				return
			}
		}
		statefunc.L.PushString(text)
	case "N":
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return
		}
		statefunc.L.PushNumber(f)
	case "I":
		i, err := strconv.Atoi(text)
		if err != nil {
			return
		}
		statefunc.L.PushInteger(i)
	case "B":
		b, err := strconv.ParseBool(text)
		if err != nil {
			return
		}
		inp.SetLabelColor(tcell.ColorDefault)
		statefunc.L.PushBoolean(b)
	default:
		errorhandlefunc.ThrowError(i18nfunc.T("error.input_field_invalid_type", map[string]interface{}{
			"Name": inp.GetTitle(),
		}), errorhandlefunc.ErrorTypeScript, true)
		CurrForm.Form.SetFocus(fi) // Keep focus on the field
		return
	}
	statefunc.L.Call(2, 0)
}
