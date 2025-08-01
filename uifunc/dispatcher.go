package uifunc

import (
	"fmt"
	"gotulua/statefunc"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Widget struct {
	WidgetTitle string
	Widget      tview.Primitive // Use tview.Primitive to allow any tview widget
	Region      string
	Browse      *TBrowse
}

var Widgets []Widget
var info *tview.TextView = nil
var btnInfo *tview.TextView = nil
var currRegion string = ""
var currBtnRegion string = ""

func AddWidget(widget tview.Primitive, title string, browse *TBrowse) {
	w := Widget{
		WidgetTitle: title,
		Widget:      widget,
		Region:      strconv.Itoa(len(Widgets)),
		Browse:      browse,
	}
	Widgets = append(Widgets, w)
	if info == nil {
		info = createInfo(w) // Create the info TextView if it doesn't exist
	}
	showWidget(Widgets[0].WidgetTitle)                   // Show the first widget by default
	statefunc.App.SetRoot(statefunc.RunFlexLevel0, true) // If not found, just reset the root to the main layout
	statefunc.App.SetFocus(statefunc.RunFlexLevel0)      // Set focus to the main layout
	statefunc.App.ForceDraw()                            // Force redraw the application

}

func ClearWidgets() {
	Widgets = []Widget{}            // Clear the Widgets slice
	statefunc.RunFlexLevel0.Clear() // Clear the current layout
	info = nil                      // Reset the info TextView
}

func showWidget(title string) {
	for i, w := range Widgets {
		if w.WidgetTitle == title {
			showCurrentWidget(i) // Show the widget by its index
			return
		}
	}
}

func showCurrentWidget(w int) {
	if len(Widgets) > 0 {
		statefunc.RunFlexLevel0.Clear()
		switch Widgets[w].Widget.(type) {
		case *tview.Table:
			if Widgets[w].Browse.Buttons != nil {
				flex := tview.NewFlex().SetDirection(tview.FlexRow)
				flex.AddItem(Widgets[w].Widget, 0, 1, true)
				buttFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
				buttFlex.AddItem(setBrowseButtons(Widgets[w].Browse), 0, 1, true)
				flex.AddItem(buttFlex, 1, 0, true).AddItem(tview.NewFlex(), 1, 0, false)
				flex.SetTitle(" Ctrl+N/Ctrl+P - Next/Previous, F7 - Set Filter, Enter - Edit ")
				flex.SetBorder(true)
				flex.SetBorderPadding(1, 1, 1, 1)
				statefunc.RunFlexLevel0.AddItem(flex, 0, 1, true)
			} else {
				flex := tview.NewFlex().SetDirection(tview.FlexRow)
				flex.AddItem(Widgets[w].Widget, 0, 1, true)
				flex.SetTitle(" Ctrl+N/Ctrl+P - Next/Previous, F7 - Set Filter, Enter - Edit ")
				flex.SetBorder(true)
				flex.SetBorderPadding(1, 1, 1, 1)
				statefunc.RunFlexLevel0.AddItem(flex, 0, 1, true)
			}
		default:
			statefunc.RunFlexLevel0.AddItem(Widgets[w].Widget, 0, 1, true)
		}
		currRegion = Widgets[w].Region                     // Update the current region
		setInfo(Widgets[w])                                // Set the info TextView with the current widget
		statefunc.RunFlexLevel0.AddItem(info, 1, 0, false) // Add the info TextView to the layout
		if len(Widgets) > 1 {
			statefunc.RunFlexLevel0.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Key() {
				case tcell.KeyCtrlN:
					if w+1 < len(Widgets) {
						showCurrentWidget(w + 1) // Show the next widget
					} else {
						showCurrentWidget(0) // Wrap around to the first widget
					}
				case tcell.KeyCtrlP:
					if w-1 >= 0 {
						showCurrentWidget(w - 1) // Show the previous widget
					} else {
						showCurrentWidget(len(Widgets) - 1) // Wrap around to the last widget
					}
				}
				return event
			})
		}
		statefunc.App.SetRoot(statefunc.RunFlexLevel0, true) // Set the root to the Flex layout
		statefunc.App.SetFocus(Widgets[w].Widget)            // Set focus to the widget
	}
}

func createInfo(w Widget) *tview.TextView {
	// The bottom row has some info on where we are.
	info := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false).
		SetHighlightedFunc(func(added, removed, remaining []string) {
			if len(added) == 0 {
				return
			}
			if currRegion != added[0] {
				region, _ := strconv.Atoi(added[0]) // Convert the added region to an integer
				showCurrentWidget(region)           // Show the widget corresponding to the added region
			}
		})
	return info

}

func setInfo(w Widget) {
	if info == nil {
		info = createInfo(w) // Create the info TextView if it doesn't exist
	}
	info.Clear() // Clear the previous content
	for _, wd := range Widgets {
		fmt.Fprintf(info, `["%s"][white:black]%s[white][""]  `, wd.Region, wd.WidgetTitle)
	}
	info.Highlight(w.Region).ScrollToHighlight() // Highlight the current widget
}

func createBrowseButtons(b *TBrowse) *tview.TextView {
	// The bottom row has some info on where we are.
	btnInfo := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false).
		SetHighlightedFunc(func(added, removed, remaining []string) {
			if len(added) == 0 {
				btnInfo.Highlight("")
				return
			}
			if currBtnRegion != added[0] {
				go func() {
					statefunc.App.QueueUpdateDraw(func() {
						btnInfo.Highlight(added[0])
					})
					statefunc.App.QueueUpdateDraw(func() {
						btnInfo.Highlight("")
					})
				}()
				ind, _ := strconv.Atoi(added[0])
				b.onButtonPress(b.Buttons[ind].Function)
			}
		})
	return btnInfo

}

func setBrowseButtons(b *TBrowse) *tview.TextView {
	if btnInfo == nil {
		btnInfo = createBrowseButtons(b) // Create the info TextView if it doesn't exist
	}
	btnInfo.Clear() // Clear the previous content
	for i, btn := range b.Buttons {
		fmt.Fprintf(btnInfo, `["%d"][white:black]%s[white][""]  `, i, btn.Caption)
	}
	btnInfo.ScrollToHighlight() // Highlight the current widget
	return btnInfo
}
