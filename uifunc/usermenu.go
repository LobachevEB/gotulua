package uifunc

import (
	"gotulua/errorhandlefunc"
	"gotulua/i18nfunc"
	"gotulua/statefunc"
	"strings"

	"github.com/Shopify/go-lua"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// MenuItem represents a single menu item with its caption and associated Lua function
type MenuItem struct {
	Caption     string
	LuaFunction string
	Enabled     bool
}

// UserMenu represents the vertical menu structure
type UserMenu struct {
	*tview.Flex
	items   []MenuItem
	buttons []*tview.Button
}

var MainUserMenu *UserMenu

// NewUserMenu creates a new vertical menu
func NewUserMenu(L *lua.State) int {
	menu := &UserMenu{
		Flex:    tview.NewFlex().SetDirection(tview.FlexRow),
		items:   make([]MenuItem, 0),
		buttons: make([]*tview.Button, 0),
	}
	MainUserMenu = menu
	MainUserMenu.SetBorder(true)
	MainUserMenu.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			statefunc.PopVisual()
			statefunc.App.SetRoot(statefunc.MainFlex, true)
		case tcell.KeyUp:
			statefunc.App.SetFocus(MainUserMenu.buttons[0])
			return nil
		case tcell.KeyDown:
			statefunc.App.SetFocus(MainUserMenu.buttons[len(MainUserMenu.buttons)-1])
			return nil
		case tcell.KeyLeft:
			statefunc.App.SetFocus(MainUserMenu.buttons[0])
			return nil
		case tcell.KeyRight:
			statefunc.App.SetFocus(MainUserMenu.buttons[len(MainUserMenu.buttons)-1])
			return nil
		}
		return event
	})
	statefunc.RunFlexLevelUserMenu.AddItem(MainUserMenu, 0, 1, true)
	statefunc.App.SetRoot(statefunc.RunFlexLevelUserMenu, true)
	return 1
}

// AddMenuItem adds a new menu item to the menu
func AddMenuItem(caption, luaFunc string) int {
	m := MainUserMenu
	if m == nil {
		r := NewUserMenu(statefunc.L)
		if r != 1 {
			return 0
		}
		m = MainUserMenu
	}
	item := MenuItem{
		Caption:     caption,
		LuaFunction: luaFunc,
		Enabled:     true,
	}
	m.items = append(m.items, item)

	// Create a new button for this menu item
	button := tview.NewButton(caption)
	button.SetBackgroundColor(tcell.ColorDefault)
	//button.SetBorder(true)

	// Set up the button's selected function
	idx := len(m.items) - 1 // Capture the current index
	button.SetSelectedFunc(func() {
		if m.items[idx].Enabled {
			executeMenuItem(idx)
		}
	})

	// Add button to our slice and to the flex layout
	m.buttons = append(m.buttons, button)
	if len(m.items) == 1 {
		statefunc.App.SetFocus(button)
	}
	m.AddItem(button, 1, 0, true)
	return 1
}

func AddMenuItems(items string) int {
	// items is a string of the form "caption1,function1;caption2,function2;..."
	// split the string into a slice of strings
	itemsSlice := strings.Split(items, ";")
	for _, item := range itemsSlice {
		items := strings.Split(item, ",")
		if len(items) != 2 {
			errorhandlefunc.ThrowError(i18nfunc.T("error.menu_invalid_item", map[string]interface{}{
				"Item": item,
			}), errorhandlefunc.ErrorTypeScript, true)
			return 0
		}
		AddMenuItem(items[0], items[1])
	}
	return 1
}

// executeMenuItem executes the Lua function associated with the menu item
func executeMenuItem(index int) {
	m := MainUserMenu
	if m == nil {
		return
	}
	if index < 0 || index >= len(m.items) {
		return
	}

	item := m.items[index]

	// Get the Lua function
	statefunc.L.Global(item.LuaFunction)
	if !statefunc.L.IsFunction(-1) {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_a_function", map[string]interface{}{
			"Name": item.LuaFunction,
		}), errorhandlefunc.ErrorTypeScript, true)
		return
	}

	// Switch to application base run flex
	statefunc.RunFlexLevel0.Clear()
	statefunc.App.SetRoot(statefunc.RunFlexLevel0, true)
	statefunc.PushVisual(statefunc.RunFlexLevelUserMenu)
	err := statefunc.L.ProtectedCall(0, 0, 0)
	if err != nil {
		statefunc.L.SetTop(0)
		errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeScript, false)
	}

}

// DisableMenuItem disables a menu item by its caption
func DisableMenuItem(caption string) int {
	m := MainUserMenu
	if m == nil {
		return 0
	}

	for i, item := range m.items {
		if item.Caption == caption {
			m.items[i].Enabled = false
			// Update button appearance
			m.buttons[i].SetLabelColor(tcell.ColorGray)
			return 1
		}
	}

	errorhandlefunc.ThrowError(i18nfunc.T("error.menu_item_not_found", map[string]interface{}{
		"Caption": caption,
	}), errorhandlefunc.ErrorTypeScript, true)
	return 0
}

// EnableMenuItem enables a menu item by its caption
func EnableMenuItem(caption string) int {
	m := MainUserMenu
	if m == nil {
		return 0
	}

	for i, item := range m.items {
		if item.Caption == caption {
			m.items[i].Enabled = true
			// Restore default button appearance
			m.buttons[i].SetLabelColor(tcell.ColorWhite)
			return 1
		}
	}

	errorhandlefunc.ThrowError(i18nfunc.T("error.menu_item_not_found", map[string]interface{}{
		"Caption": caption,
	}), errorhandlefunc.ErrorTypeScript, true)
	return 0
}

// RemoveMenuItem removes a menu item by its caption
func RemoveMenuItem(caption string) int {
	m := MainUserMenu
	if m == nil {
		return 0
	}

	for i, item := range m.items {
		if item.Caption == caption {
			// Remove the button from the flex layout
			m.RemoveItem(m.buttons[i])

			// Remove the item and button from our slices
			m.items = append(m.items[:i], m.items[i+1:]...)
			m.buttons = append(m.buttons[:i], m.buttons[i+1:]...)
			return 1
		}
	}

	errorhandlefunc.ThrowError(i18nfunc.T("error.menu_item_not_found", map[string]interface{}{
		"Caption": caption,
	}), errorhandlefunc.ErrorTypeScript, true)
	return 0
}
