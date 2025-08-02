package uifunc

import (
	"errors"
	"fmt"
	"gotulua/boolfunc"
	"gotulua/errorhandlefunc"
	"gotulua/gormfunc"
	"gotulua/i18nfunc"
	"gotulua/inputfunc"
	"gotulua/statefunc"
	"gotulua/syncfunc"
	"gotulua/timefunc"
	"gotulua/typesfunc"
	"reflect"
	"strings"

	"github.com/Shopify/go-lua"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TBrowseField represents a field in a browse view, including its properties and behaviors.
//
// Fields:
//   - Name: The internal name of the field.
//   - Caption: The display caption for the field.
//   - IsTableField: Indicates if the field is a direct table field.
//   - IsEditable: Indicates if the field can be edited by the user.
//   - ConstValue: Holds a constant value for the field, if applicable.
//   - Function: The name of a function to call for the field, if applicable.
//   - IsLookup: Indicates if the field is a lookup field.
//   - LookupTable: Pointer to the TBrowse structure used for lookup fields.
//   - LookupFunc: The name of the function used to perform lookup operations for this field.
type TBrowseField struct {
	Name         string
	Caption      string
	IsTableField bool        // Indicates if the field is a table field
	IsEditable   bool        // Indicates if the field is editable
	ConstValue   interface{} // Constant value for the field, if applicable
	Function     string      // Function to call for the field, if applicable
	IsLookup     bool        // Indicates if the field is a lookup field
	LookupBrowse *TBrowse    // Pointer to the TBrowse for lookup fields
	LookupFunc   string
	ExtraType    string //Set if the field type is kind of Date/Time/DateTime/Boolean. Allowed values "", "D", "T", "DT", "B"
}

type TButton struct {
	Caption  string
	Function string
}

type TBrowse struct {
	Title            string
	Table            *gormfunc.Table // Pointer to the gormfunc.Table
	LookupBrowseDest *TBrowse
	LookupFieldDest  *TBrowseField
	TableView        *tview.Table   // Pointer to the tview.Table for displaying the browse view
	NewRowNum        int            // Counter for new rows, initialized to -1
	Fields           []TBrowseField // List of fields to display in the browse view
	Buttons          []TButton      // List of buttons to display in the browse view
	isLookup         bool           // IsLookup indicates whether the current operation is a lookup action.
	lastRowVisited   int
	NearLookup       bool
	Filters          map[string]string
}

// BrowseTableNew creates a new TBrowse instance and adds it to the Lua state.
// It is registered with the Lua interpreter.
//
// Parameters:
//   - L: The current Lua state.
//
// Returns:
//   - int: Returns 1 on success, 0 if the arguments are not valid.
func BrowseTableNew(L *lua.State, isLookup bool) int {
	if L.Top() < 2 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args", map[string]interface{}{
			"Name": "AddBrowse, AddLookup",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	ud := L.ToUserData(1)                 // Get the userdata from Lua
	tw, ok := ud.(*gormfunc.TableWrapper) // Get the table from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.first_arg_not_table", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	table := tw.Table            // Get the gormfunc.Table from the wrapper
	caption, ok := L.ToString(2) // Get the caption from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.second_arg_not_string", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	//isLookup := L.ToBoolean(3)
	// Create a new TBrowse object
	browse := &TBrowse{
		Title:          caption,
		Table:          table,
		NewRowNum:      -1, // Initialize NewRowNum to -1
		isLookup:       isLookup,
		lastRowVisited: -1,
		Filters:        make(map[string]string),
	}
	L.PushUserData(browse) // Push the TBrowse object as userdata
	L.Global("BrowseMT")   // Push the metatable
	L.SetMetaTable(-2)     // Set metatable for userdata
	return 1               // Return the number of results
}

func AddButton(L *lua.State) int {
	browse, ok := L.ToUserData(1).(*TBrowse) // Get the browse from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.first_argument_not_browse", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	caption, ok := L.ToString(2) // Get the caption from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.second_argument_not_string", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	functionValue, ok := L.ToString(3) // Get the function value from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.third_argument_not_string", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	browse.addButton(L, caption, functionValue) // Call the AddButton method on the browse
	return 1
}

// addButton adds a new button to the TBrowse instance.
// This button will use a Lua function (specified by functionValue) to compute its value dynamically.
//
// Parameters:
//   - L: The current Lua state.
//   - caption: The caption to display for the button.
//   - functionValue: The name of the Lua function to use for this button.
func (b *TBrowse) addButton(L *lua.State, caption string, functionValue string) int {
	if functionValue == "" {
		errorhandlefunc.ThrowError(i18nfunc.T("error.function_not_set", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	button := TButton{
		Caption:  caption,
		Function: functionValue,
	}
	b.Buttons = append(b.Buttons, button)
	return 1
}

// addTableField adds a new table-based field to the TBrowse instance.
// This field will use a table field (specified by fieldName) to display its value.
//
// Parameters:
//   - L: The current Lua state.
//   - fieldName: The name of the field to add.
func (b *TBrowse) addTableField(L *lua.State, fieldName, caption string, isEditable bool, extType string) int {
	if fieldName == "" {
		errorhandlefunc.ThrowError(i18nfunc.T("error.field_name_not_set", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	if len(b.Table.GetFieldType(fieldName)) == 0 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.db_field_not_found", map[string]interface{}{
			"Field": fieldName,
			"Table": b.Table.Name,
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	field := TBrowseField{
		Name:         fieldName,
		Caption:      caption,
		IsTableField: true,
		ExtraType:    extType,
		IsEditable:   isEditable,
	}
	b.Fields = append(b.Fields, field) // Add the field to the browse view
	return 1
}

// addFuncField adds a new function-based field to the TBrowse instance.
// This field will use a Lua function (specified by functionValue) to compute its value dynamically.
//
// Parameters:
//   - L: The current Lua state.
//   - fieldName: The name of the field to add.
//   - caption: The caption to display for the field.
//   - functionValue: The name of the Lua function to use for this field.
//
// Returns:
//   - int: Returns 1 on success, 0 if the function value is not set.
func (b *TBrowse) addFuncField(L *lua.State, fieldName, caption string, functionValue string) int {
	if functionValue == "" {
		errorhandlefunc.ThrowError(i18nfunc.T("error.function_not_set", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	field := TBrowseField{
		Name:     fieldName,
		Caption:  caption,
		Function: functionValue, // Set the function for the field
	}
	b.Fields = append(b.Fields, field) // Add the field to the browse view
	return 1
}

func AddField(L *lua.State) int {
	browse, ok := L.ToUserData(1).(*TBrowse)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.first_arg_not_browse", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	description, ok := L.ToString(2)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.second_arg_not_string", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	browse.addField(L, description)
	return 1
}

// addField parses a field description string and adds one or more fields to the TBrowse instance.
// The description string should contain field definitions separated by '|', where each field is
// defined by semicolon-separated key-value pairs (e.g., "n::Name;c::Caption;f::Function;e::true;t::Type").
// Recognized keys are:
//   - n: field name (required)
//   - c: field caption (optional)
//   - f: function name for computed fields (optional)
//   - e: editable flag ("true" or "false", optional)
//   - t: extra type information (optional)
//
// If a function is specified, AddFuncField is called; otherwise, AddTableField is used.
// Returns 1 to indicate success.
func (b *TBrowse) addField(L *lua.State, description string) int {
	//n::Name;c::Pet Name;f::GetPetName|n::Vaccine;c::Vaccine Used;e::true|n::Date;c::Vaccination Date;e::true;t::D
	fields := strings.Split(description, "|")
	for _, field := range fields {
		var name, caption, function, editable, extraType string
		parts := strings.Split(field, ";")
		for _, part := range parts {
			params := strings.Split(part, "::")
			if len(params) == 2 {
				switch params[0] {
				case "n":
					name = params[1]
				case "c":
					caption = params[1]
				case "f":
					function = params[1]
				case "e":
					editable = params[1]
				case "t":
					extraType = params[1]
				}
			}
		}
		if name != "" {
			if function != "" {
				b.addFuncField(L, name, caption, function)
			} else {
				b.addTableField(L, name, caption, editable == "true", extraType)
			}
		}
	}
	return 1
}

// setFieldLookup associates a lookup function with a specific field in the TBrowse instance.
// It allows dynamic lookup or transformation of field values using a Lua function.
//
// Parameters:
//
//	L          - The current Lua state.
//	fieldName  - The name of the field to associate with the lookup function.
//	browse     - The TBrowse instance on which to set the lookup.
//	lookupFunc - The name of the Lua function to use for field lookup.
func (b *TBrowse) setFieldLookup(L *lua.State, fieldName string, browse *TBrowse, lookupFunc string) int {
	f := b.findFieldByName(fieldName)
	if f == nil {
		errorhandlefunc.ThrowError(i18nfunc.T("error.field_not_found", map[string]interface{}{
			"Name": fieldName,
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	f.IsLookup = true
	f.LookupBrowse = browse
	f.LookupFunc = lookupFunc
	return 1
}

func (b *TBrowse) findFieldByName(name string) *TBrowseField {
	for i, f := range b.Fields {
		if f.Name == name {
			return &b.Fields[i]
		}
	}
	return nil
}

func (b *TBrowse) setLookupBrowseDest(bl *TBrowse, f *TBrowseField) {
	b.LookupBrowseDest = bl
	b.LookupFieldDest = f
}

// Show displays the table in a screen, allowing in-place editing,
// navigation, and interaction with the table data. It sets up the table view
// with headers, populates rows from the underlying table, and configures
// keyboard and mouse event handlers for editing, row addition, deletion, and
// lookup functionality. The method also manages selection changes and invokes
// Lua callbacks as needed. If not in lookup mode, the TableView is pushed as
// Lua userdata for further manipulation.
func (b *TBrowse) Show(L *lua.State) int {
	b.TableView = tview.NewTable().SetBorders(true).SetSelectable(true, true).SetFixed(1, 0) // Set borders for the TableView
	//b.TableView.SetBorder(true)                                               // Set a border around the TableView
	//b.TableView.SetBorderPadding(1, 1, 1, 1)                                  //
	b.TableView.SetTitle(b.Title) // Set the title for the TableView
	if len(b.Fields) > 0 {
		for i := range b.Fields {
			b.TableView.SetCell(0, i, tview.NewTableCell(b.Fields[i].Caption).SetSelectable(false))
		}
	} else {
		for i, col := range b.Table.Columns {
			b.TableView.SetCell(0, i, tview.NewTableCell(col).SetSelectable(false)) // Set column headers
		}
	}
	if b.Table.Find() { // Find all rows in the table
		for {
			b.initRow(L)
			if !b.Table.Next() { // Move to the next row
				b.Table.ScrollToBeginning()     // Stand to the first row
				b.TableView.ScrollToBeginning() // Scroll to the beginning of the table
				break                           // Exit the loop if no more rows
			}
		}
	} else {
		b.Table.Init()
		b.initRow(L)
		b.setNewRowMode(1) // Set NewRowNum to the next row index
		b.TableView.ScrollToBeginning()
	}
	// In-place editing
	b.TableView.SetSelectedFunc(func(row, column int) {
		cell := b.TableView.GetCell(row, column)
		if cell == nil {
			return // If the cell is nil, do nothing
		}
		field, ok := cell.GetReference().(TBrowseField) // Get the reference from the cell
		if !ok {
			return
		}
		if !field.IsTableField {
			return // Only allow editing for table fields
		}
		if !b.isNewRowMode() {
			if b.Table.GetField(field.Name, field.ExtraType) == nil {
				return // Field does not exist in the table
			}
		}
		if !field.IsEditable {
			return // Only allow editing for editable fields
		}
		initial := cell.Text

		extType := b.Table.GetFieldType(field.Name)

		showBrowseEdit(field.Caption, initial, extType, func(s string, key tcell.Key) {
			switch key {
			case tcell.KeyEscape:
				statefunc.Pages.SwitchToPage("main")
				return
			case tcell.KeyEnter:
				result := s
				if result != initial {
					if result == "" {
						result = fmt.Sprintf("%v", b.Table.GetDefaultValueForTheField(field.Name))
					}
					if b.isNewRowMode() {
						// If in new row mode, add a new row with the input value
						if !b.Table.AddRow(field.Name, result) { // Add a new row to the table
							return
						}
					} else {
						t := b.Table.GetFieldType(field.Name)
						var err error
						switch t {
						case typesfunc.TypeDate:
							result, err = timefunc.FormatDateTime(result, t, timefunc.ToInternalFormat)
							if err != nil {
								errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeData, false)
								return
							}
						case typesfunc.TypeTime:
							result, err = timefunc.FormatDateTime(result, t, timefunc.ToInternalFormat)
							if err != nil {
								errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeData, false)
								return
							}
						case typesfunc.TypeDateTime:
							result, err = timefunc.FormatDateTime(result, t, timefunc.ToInternalFormat)
							if err != nil {
								errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeData, false)
								return
							}
						case typesfunc.TypeBoolean:
							result, err = boolfunc.FormatBool(result, boolfunc.ToInternalFormat)
							if err != nil {
								errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeData, false)
								return
							}
						}
						if !b.Table.SaveField(field.Name, result) { // Set the field value in the table
							return
						}
					}
					cell.SetText(s)
					b.clearNewRowMode() // Reset NewRowNum to -1 after editing
					b.refreshBrowseLine()
				}
				statefunc.Pages.SwitchToPage("main")
				return
			}
			statefunc.Pages.SwitchToPage("main")

		})
		statefunc.Pages.SwitchToPage("browseedit")
	})
	b.TableView.SetSelectionChangedFunc(func(row, column int) {
		// TODO: add calls of the lua callbacks linked to current line of the browse
		if b.Table.Rows != nil {
			if row > 0 {
				b.Table.Rows.Pos = row - 1 // Set the current row position in the table
			} else {
				b.Table.Rows.Pos = 0
			}
			if b.lastRowVisited != row {
				b.lastRowVisited = row
				for i := 0; i < b.TableView.GetColumnCount(); i++ {
					cell := b.TableView.GetCell(row, i)
					if cell != nil {
						b.runCellFuncIfExists(L, cell)
					}
				}
			}
		} else {
			b.Table.Init()
		}
		// 	cell := tableView.GetCell(row, column)
		// 	//fmt.Printf("User moved to row %d, column %d, cell text: %s\n", row, column, cell.Text)
	})
	b.TableView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter: //KeyCtrlL:
			if b.isLookup {
				syncfunc.SetLookupSuccess(true)
				b.applyLookup(L)
				if !syncfunc.GetLookupSuccess() {
					syncfunc.SetLookupSuccess(true)
					return event
				}
				// pagesfunc.Pages.RemovePage("browselookup")
				// pagesfunc.Pages.SwitchToPage("main")
				BrowseSubitemsFlex.Clear()
				statefunc.App.SetRoot(statefunc.RunFlexLevel0, true)
				b.LookupBrowseDest.checkInsertedLineToShow(L)
				return event
			}

			row, column := b.TableView.GetSelection()
			cell := b.TableView.GetCell(row, column)
			if cell == nil {
				return event // If the cell is nil, do nothing
			}
			field, ok := cell.GetReference().(TBrowseField) // Get the reference from the cell
			if !ok {
				return event
			}
			if field.LookupBrowse == nil {
				return event
			}
			syncfunc.BrowseChId = -1
			b.NearLookup = true
			field.LookupBrowse.Show(L) // Initialize lookup browse
			field.LookupBrowse.setLookupBrowseDest(b, &field)
			showBrowseLookup(field.LookupBrowse.TableView)
			return event
		case tcell.KeyEscape:
			// If Escape is pressed, return to the main view
			if b.isLookup {
				BrowseSubitemsFlex.Clear()
				statefunc.App.SetRoot(statefunc.RunFlexLevel0, true)
				return nil
			} else {
				// statefunc.App.SetRoot(statefunc.RunFlexLevel0, true).SetFocus(statefunc.RunFlexLevel0)
				statefunc.App.SetRoot(statefunc.MainFlex, true).SetFocus(statefunc.MainFlex)
				return nil // Return nil to indicate the event was handled
			}
		case tcell.KeyDown:
			if !b.isLookup {
				row, _ := b.TableView.GetSelection()
				lastRow := b.TableView.GetRowCount() - 1
				if row == lastRow {
					// If the last row is selected, do not allow further down navigation
					if !b.isNewRowMode() {
						// If no new row is being added, return nil to indicate the event was handled
						b.setNewRowMode(lastRow + 1) // Set NewRowNum to the next row index
						b.addNewEmptyRow(L)          // Add a new row if needed
					}
				}
			}
		case tcell.KeyUp:
			if !b.isLookup {
				row, _ := b.TableView.GetSelection()
				lastRow := b.TableView.GetRowCount() - 1
				if row == lastRow {
					if b.isNewRowMode() {
						if lastRow > 1 {
							b.TableView.RemoveRow(lastRow)
						}
						b.clearNewRowMode()
					} else {
						if b.checkBrowseFiltered() {
							b.refreshBrowse(false)
						}
					}
				}
			}
		case tcell.KeyDelete:
			if !b.isLookup && !b.isNewRowMode() {
				Confirm(i18nfunc.T("dialog.remove_row", nil), func(idx bool) {
					if idx {
						b.deleteRow()
					}
				})
			}
		case tcell.KeyF7:
			b.showBrowseFilter()
		}
		return event // Return the event for further processing
	})
	b.TableView.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if !b.isLookup {
			row, _ := b.TableView.GetSelection()
			lastRow := b.TableView.GetRowCount() - 1
			if action == tview.MouseLeftClick {
				if b.isNewRowMode() {
					if row < lastRow {
						if lastRow >= 0 {
							b.TableView.RemoveRow(lastRow)
						}
						b.clearNewRowMode()
					}
				} else {
					x, y := event.Position()
					cellr, _ := b.TableView.CellAt(x, y)
					if cellr < lastRow {
						if b.checkBrowseFiltered() {
							b.refreshBrowse(false)
						}
					}

				}
			}
		}
		return action, event // Default passthrough, customize as needed
	})

	if !b.isLookup {
		// Add the tableView to the Flex layout
		L.PushUserData(b.TableView)               // Push the TableView as userdata
		statefunc.SetRunMode(statefunc.RunAsForm) // Set the run mode to Form
		AddWidget(b.TableView, b.Title, b)
	}
	return 1 // Return the number of results
}

func (b *TBrowse) deleteRow() {
	if b.Table.DeleteRow() {
		row, col := b.TableView.GetSelection()
		b.TableView.RemoveRow(row)
		if row >= b.TableView.GetRowCount() {
			row = b.TableView.GetRowCount() - 1
		}
		b.TableView.Select(row, col)
		if b.TableView.GetRowCount() == 1 {
			//b.TableView.ScrollToBeginning()
			b.Table.Init()
			b.setNewRowMode(1) // Set NewRowNum to the next row index
			b.addNewEmptyRow(statefunc.L)
			//b.InitRow(statefunc.L)
			b.TableView.ScrollToBeginning()
		}
		b.refreshFuncCells(statefunc.L)

	}

}

func (b *TBrowse) checkInsertedLineToShow(L *lua.State) {
	if !b.isLookup && b.TableView.HasFocus() && b.NearLookup {
		//b.NearLookup = false
		id := syncfunc.BrowseChId
		if id > 0 {
			syncfunc.BrowseChId = -1
			b.NearLookup = false
			b.addNewRowByTableRow(b.Table.GetCurrentRecord())
			b.clearNewRowMode()
			b.refreshBrowseLine()
			b.refreshFuncCells(L)
		} else {
			b.refreshBrowseLine()
		}
	}
}

func (b *TBrowse) refreshFuncCells(L *lua.State) {
	row, _ := b.TableView.GetSelection()
	for col := range b.TableView.GetColumnCount() {
		cell := b.TableView.GetCell(row, col)
		ref := cell.GetReference()
		if ref != nil {
			field := ref.(TBrowseField)
			if field.Function != "" { // If the field has a function, call it
				v := b.runFieldFunction(L, field.Function)
				cell := b.TableView.GetCell(row, col)
				cell.SetText(fmt.Sprintf("%v", v))
			}
		}
	}
}

func (b *TBrowse) refreshBrowse(goTop bool) {
	colCount := b.TableView.GetColumnCount()
	rc := b.TableView.GetRowCount()
	if rc > 1 {
		for col := 0; col < colCount; col++ {
			hCell := b.TableView.GetCell(0, col)
			if hCell != nil {
				hCell.SetStyle(tcell.StyleDefault.Normal().Underline(false))
			}
			cell := b.TableView.GetCell(1, col)
			if cell != nil {
				ref := cell.GetReference()
				if ref != nil {
					field := ref.(TBrowseField)
					if b.Filters != nil {
						if b.Filters[field.Name] != "" {
							hCell.SetStyle(tcell.StyleDefault.Normal().Underline(true))
						}
					}
				}
			}
		}
		for i := rc - 1; i > 0; i-- {
			b.TableView.RemoveRow(i)
		}
	}
	if b.Table.Find() {
		for {
			b.initRow(statefunc.L)
			if !b.Table.Next() { // Move to the next row
				if goTop {
					b.Table.ScrollToBeginning()     // Stand to the first row
					b.TableView.ScrollToBeginning() // Scroll to the beginning of the table
				}
				break // Exit the loop if no more rows
			}
		}
	} else {
		b.Table.Init()
		b.initRow(statefunc.L)
		b.setNewRowMode(1) // Set NewRowNum to the next row index
		b.TableView.ScrollToBeginning()
		return
	}

}

func (b *TBrowse) refreshBrowseLine() {
	id := b.getRowId()
	if id == 0 {
		return
	}
	if b.Table.FindByID(id) {
		b.clearNewRowMode()
		b.initRow(statefunc.L)
	}
}

func (b *TBrowse) getRowId() int64 {
	r := b.Table.GetCurrentRecord()
	if r == nil {
		return 0
	}
	if r[gormfunc.PrimaryKeyField] == nil {
		return 0
	}
	_id := r[gormfunc.PrimaryKeyField]
	var id int64
	switch v := _id.(type) {
	case int:
		id = int64(v)
	case int64:
		id = v
	default:
		return 0
	}
	return id
}

func (b *TBrowse) runCellFuncIfExists(L *lua.State, cell *tview.TableCell) {
	ref := cell.GetReference()
	if ref == nil {
		return
	}
	field := ref.(TBrowseField)
	if field.Function == "" {
		return
	}
	result := b.runFieldFunction(L, field.Function)
	cell.SetText(fmt.Sprintf("%v", result))
}

func (b *TBrowse) applyLookup(L *lua.State) {
	if b.LookupFieldDest == nil {
		return
	}
	if b.LookupFieldDest.LookupFunc == "" {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			//errorhandlefunc.ThrowError(r.(string), errorhandlefunc.ErrorTypeScript, true)
			//fmt.Println(r)
			statefunc.CatchErrorShowEditor(r.(string))
			syncfunc.SetLookupSuccess(false)
		}
	}()
	// Push the Lua function first
	L.Global(b.LookupFieldDest.LookupFunc)
	if !L.IsFunction(-1) {
		fmt.Printf("Lua global '%s' is not a function (type: %v)\n", b.LookupFieldDest.LookupFunc, L.TypeOf(-1))
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_a_function", map[string]interface{}{
			"Name": b.LookupFieldDest.LookupFunc,
		}), errorhandlefunc.ErrorTypeScript, true)
		L.Pop(1)
		return
	}
	// Push the first argument (wrapper)
	wrapper := &gormfunc.TableWrapper{Table: b.Table}
	L.PushUserData(wrapper)
	L.PushString("TableMT")
	L.RawGet(lua.RegistryIndex)
	if L.IsNil(-1) {
		L.Pop(1)
		L.Pop(1) // remove wrapper
		L.Pop(1) // remove function
		errorhandlefunc.ThrowError(i18nfunc.T("error.tablemt_metatable_not_found", map[string]interface{}{
			"Name": b.LookupFieldDest.LookupFunc,
		}), errorhandlefunc.ErrorTypeScript, true)
		return
	}
	L.SetMetaTable(-2)
	// Push the second argument (wrapper1)
	wrapper1 := &gormfunc.TableWrapper{Table: b.LookupBrowseDest.Table}
	L.PushUserData(wrapper1)
	L.PushString("TableMT")
	L.RawGet(lua.RegistryIndex)
	if L.IsNil(-1) {
		L.Pop(1)
		L.Pop(1) // remove wrapper1
		L.Pop(1) // remove wrapper
		L.Pop(1) // remove function
		errorhandlefunc.ThrowError(i18nfunc.T("error.tablemt_metatable_not_found", map[string]interface{}{
			"Name": b.LookupFieldDest.LookupFunc,
		}), errorhandlefunc.ErrorTypeScript, true)
		return
	}
	L.SetMetaTable(-2)
	// Now stack: [function, wrapper, wrapper1]
	err := L.ProtectedCall(2, 0, 0)
	if err != nil {
		errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeScript, true)
	}
}

func (b *TBrowse) convertFldFormatIntToUser(field *TBrowseField) (interface{}, error) {
	v := b.Table.GetCurrentRecord()[field.Name]
	ft := b.Table.GetFieldType(field.Name) // Get the field type
	switch ft {
	case typesfunc.TypeDate, typesfunc.TypeTime, typesfunc.TypeDateTime, typesfunc.TypeBoolean:
		var err error
		var s string
		if v != nil {
			switch val := v.(type) {
			case string:
				s = val
			case int, int64:
				if ft == typesfunc.TypeBoolean {
					s = fmt.Sprintf("%d", val)
					v, err = boolfunc.FormatBool(s, boolfunc.ToUserFormat)
				}
			default:
				t := fmt.Sprintf("%v", reflect.TypeOf(v))
				if t == "*interface {}" {
					var vp *interface{} = v.(*interface{})
					vv := *vp
					s = vv.(string)
				} else {
					// errorhandlefunc.ThrowError(i18nfunc.T("error.value_should_be_string", map[string]interface{}{
					// 	"Value": v,
					// }), errorhandlefunc.ErrorTypeScript, true)
					return nil, errors.New(i18nfunc.T("error.value_should_be_string", map[string]interface{}{
						"Value": v,
					}))
				}
			}
			switch ft {
			case typesfunc.TypeDate, typesfunc.TypeTime, typesfunc.TypeDateTime:
				v, err = timefunc.FormatDateTime(s, ft, timefunc.ToUserFormat)
			case typesfunc.TypeBoolean:
				v, err = boolfunc.FormatBool(s, boolfunc.ToUserFormat)
			}
			if err != nil {
				errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeScript, true)
				return nil, err
			}
		} else {
			v = ""
		}
	case typesfunc.TypeInteger, typesfunc.TypeReal:
		v = fmt.Sprintf("%v", v) // Convert to string for display
	default:
		if v == nil {
			v = ""
		}
	}
	return v, nil
}

func (b *TBrowse) initRow(L *lua.State) {
	if len(b.Fields) > 0 {
		// If fields are defined, use them to populate the table
		for j, field := range b.Fields {
			if field.IsTableField { // If the field is a table field, get the value from the table
				// Get the field value from the table
				v, err := b.convertFldFormatIntToUser(&field)
				if err != nil {
					errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeScript, true)
					return
				}
				var s string
				if v != nil {
					switch v.(type) {
					case string, int, int64, float64, bool:
						s = fmt.Sprintf("%v", v)
					default:
						t := fmt.Sprintf("%v", reflect.TypeOf(v))
						if t == "*interface {}" {
							var vp *interface{} = v.(*interface{})
							vv := *vp
							s = vv.(string)
						} else {
							errorhandlefunc.ThrowError(i18nfunc.T("error.value_should_be_string", map[string]interface{}{
								"Value": v,
							}), errorhandlefunc.ErrorTypeScript, true)
							return
						}
					}
				}
				i := b.Table.Rows.Pos                                                                      // Get the current row index
				b.TableView.SetCell(i+1, j, tview.NewTableCell(s).SetSelectable(true).SetReference(field)) // Set cell values
			} else {
				result := b.runFieldFunction(L, field.Function)
				i := b.Table.Rows.Pos                                                                                              // Get the current row index
				b.TableView.SetCell(i+1, j, tview.NewTableCell(fmt.Sprintf("%v", result)).SetSelectable(true).SetReference(field)) // Set cell values
			}
		}
	} else {
		for j, col := range b.Table.Columns {
			dtType := b.Table.GetFieldType(col) // Get the field type for the column
			// if j < len(b.Fields) {
			// 	dtType = b.Fields[j].ExtraType
			// }
			b.Table.GetField(col, dtType)
			i := b.Table.Rows.Pos                                                                     // Get the current row index
			v := b.Table.Rows.Rows[i][col]                                                            // Get the field value for the column
			b.TableView.SetCell(i+1, j, tview.NewTableCell(fmt.Sprintf("%v", v)).SetSelectable(true)) // Set cell values
		}
	}
}

func (b *TBrowse) runFieldFunction(L *lua.State, function string) interface{} {
	defer func() {
		if r := recover(); r != nil {
			errorhandlefunc.ThrowError(r.(string), errorhandlefunc.ErrorTypeScript, true)
		}
	}()
	L.Global(function) // Get the function from the Lua global environment
	if !L.IsFunction(-1) {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_a_function", map[string]interface{}{
			"Name": function,
		}), errorhandlefunc.ErrorTypeScript, true)
		return nil
	}
	wrapper := &gormfunc.TableWrapper{Table: b.Table} // Create a wrapper for the Table
	L.PushUserData(wrapper)                           // Push the wrapper as userdata
	L.PushString("TableMT")
	L.RawGet(lua.RegistryIndex)
	if L.IsNil(-1) {
		L.Pop(1)
		L.PushString("TableMT metatable not found")
		L.Error()
		return 0
	}
	L.SetMetaTable(-2)

	err := L.ProtectedCall(1, 1, 0) // Call the Lua function with the Table as an argument
	if err != nil {
		errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeScript, false)
		return nil
	}
	result := L.ToValue(-1) // Get the result of the function call
	L.Pop(1)                // Pop the result from the stack
	return result
}

func (b *TBrowse) addNewEmptyRow(L *lua.State) int {
	for i, field := range b.Fields {
		// Create a new cell for each field
		cell := tview.NewTableCell(fmt.Sprintf("%v", b.Table.GetDefaultValueForTheField(field.Name))).SetSelectable(true).SetReference(field)
		//cell.SetTextColor(tcell.ColorYellow) // Set the text color for new rows
		b.TableView.SetCell(b.NewRowNum, i, cell)
	}
	//b.Table.AddRow()
	return 0
}

func (b *TBrowse) addNewRowByTableRow(row gormfunc.Record) int {
	for i, field := range b.Fields {
		// Create a new cell for each field
		var value string
		val := row[field.Name]
		if val == nil {
			val = ""
		} else {
			t := fmt.Sprintf("%v", reflect.TypeOf(val))
			var v any
			if t == "*interface {}" {
				v = *val.(*interface{})
				//value = v.(string)
			} else {
				v = val
			}
			switch x := v.(type) {
			case string:
				value = x
			case int:
				value = fmt.Sprintf("%v", x)
			case int64:
				value = fmt.Sprintf("%v", x)
			case float64:
				value = fmt.Sprintf("%v", x)
			case bool:
				value = fmt.Sprintf("%v", x)
			case *string:
				value = *x
			case *int:
				value = fmt.Sprintf("%v", *x)
			case *int64:
				value = fmt.Sprintf("%v", *x)
			case *float64:
				value = fmt.Sprintf("%v", *x)
			case *bool:
				value = fmt.Sprintf("%v", *x)
			default:
				value = fmt.Sprintf("%v", val)
			}
		}
		cell := tview.NewTableCell(value).SetSelectable(true).SetReference(field)
		//cell.SetTextColor(tcell.ColorYellow) // Set the text color for new rows
		b.TableView.SetCell(b.NewRowNum, i, cell)
	}
	//b.Table.AddRow()
	return 0
}

func (b *TBrowse) isNewRowMode() bool {
	return b.NewRowNum >= 0
}

func (b *TBrowse) setNewRowMode(num int) {
	b.NewRowNum = num
}

func (b *TBrowse) clearNewRowMode() {
	b.NewRowNum = -1
}

func AddTableField(L *lua.State) int {
	if L.Top() < 4 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "AddTableField",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	browse, ok := L.ToUserData(1).(*TBrowse) // Get the browse from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.first_argument_not_browse", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	fieldName, ok := L.ToString(2) // Get the field name from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.first_argument_not_string", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	caption, ok := L.ToString(3) // Get the caption from Lua if provided
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.second_argument_not_string", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	isEditable := L.ToBoolean(4) // Get the isEditable flag from Lua
	var addType string
	if L.Top() > 4 {
		addType, ok = L.ToString(5) // Get the DTType from Lua if provided
		if ok {
			if addType != typesfunc.TypeDate && addType != typesfunc.TypeTime && addType != typesfunc.TypeDateTime && addType != "B" {
				errorMsg := fmt.Sprintf("Allowed values of DateTime type \"%s\", \"%s\", \"%s\", \"%s\"", typesfunc.TypeDate, typesfunc.TypeTime, typesfunc.TypeDateTime, "B")
				errorhandlefunc.ThrowError(errorMsg, errorhandlefunc.ErrorTypeScript, true)
				return 0
			}
		}
	}
	browse.addTableField(L, fieldName, caption, isEditable, addType) // Call the AddField method on the browse
	return 1                                                         // Return the number of results
}

func AddFuncField(L *lua.State) int {
	if L.Top() < 4 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "AddFuncField",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	browse, ok := L.ToUserData(1).(*TBrowse) // Get the browse from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.first_argument_not_browse", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	fieldName, ok := L.ToString(2) // Get the field name from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.first_argument_not_string", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	caption, ok := L.ToString(3) // Get the caption from Lua if provided
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.second_argument_not_string", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	functionValue, ok := L.ToString(4) // Get the function value from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.fifth_argument_not_string", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	browse.addFuncField(L, fieldName, caption, functionValue) // Call the AddField method on the browse
	return 1                                                  // Return the number of results
}

func SetFieldLookup(L *lua.State) int {
	if L.Top() != 4 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.wrong_args_count", map[string]interface{}{
			"Name":     "SetFieldLookup",
			"Expected": 4,
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	browse, ok := L.ToUserData(1).(*TBrowse)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.first_argument_not_browse", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	fieldName, ok := L.ToString(2)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.second_argument_not_string", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	lookupTable, ok := L.ToUserData(3).(*TBrowse)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.third_argument_not_browse", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	lookupFunc, ok := L.ToString(4)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.fourth_argument_not_string", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	return browse.setFieldLookup(L, fieldName, lookupTable, lookupFunc)
}

func showBrowseEdit(label, text, extType string, callback func(s string, key tcell.Key)) {
	var input *tview.InputField
	input = tview.NewInputField().SetText(text).
		SetDoneFunc(func(key tcell.Key) {
			BrowseSubitemsFlex.RemoveItem(input)
			statefunc.App.SetRoot(statefunc.RunFlexLevel0, true)
			callback(input.GetText(), key)
		})
	input.SetLabel(label)
	input.SetTitle("BROWSEINPUT")
	var ph string
	switch extType {
	case typesfunc.TypeDate:
		ph = timefunc.TemplateToPlaceholder(timefunc.DateFormat)
	case typesfunc.TypeTime:
		ph = timefunc.TemplateToPlaceholder(timefunc.TimeFormat)
	case typesfunc.TypeDateTime:
		ph = timefunc.TemplateToPlaceholder(timefunc.DateTimeFormat)
	case typesfunc.TypeBoolean:
		inputfunc.SetBoolInput(input)
		input.SetPlaceholder("true/false")
		input.SetPlaceholderTextColor(tcell.ColorYellow)
		inputfunc.SetDateInput(input, ph)
	case typesfunc.TypeInteger:
		input.SetPlaceholder("0")
		input.SetPlaceholderTextColor(tcell.ColorYellow)
		input.SetAcceptanceFunc(tview.InputFieldInteger)
	case typesfunc.TypeReal:
		input.SetPlaceholder("0.0")
		input.SetPlaceholderTextColor(tcell.ColorYellow)
		input.SetAcceptanceFunc(tview.InputFieldFloat)
	}
	if extType == typesfunc.TypeDate || extType == typesfunc.TypeTime || extType == typesfunc.TypeDateTime {
		input.SetPlaceholder(ph)
		input.SetPlaceholderTextColor(tcell.ColorYellow)
		inputfunc.SetDateInput(input, ph)

	}
	BrowseSubitemsFlex.AddItem(input, 0, 1, true)
	statefunc.App.SetRoot(BrowseSubitemsFlex, true)
}

func (b *TBrowse) setBrowseFilter(s string, key tcell.Key) {
	if key != tcell.KeyEnter {
		return
	}
	field := b.getCurrentField()
	if field == nil {
		return
	}
	if !field.IsTableField {
		return
	}
	b.Filters[field.Name] = s
	b.Table.SetFilter(field.Name, s)
	//b.Table.Find()
	b.refreshBrowse(true)
}

func (b *TBrowse) checkBrowseFiltered() bool {
	for _, v := range b.Filters {
		if v != "" {
			return true
		}
	}
	return false
}

func (b *TBrowse) showBrowseFilter() bool {
	field := b.getCurrentField()
	if field == nil {
		return false
	}
	if !field.IsTableField {
		return false
	}

	var input *tview.InputField
	var flt string
	if b.Filters[field.Name] != "" {
		flt = b.Filters[field.Name]
	} else {
		v, err := b.convertFldFormatIntToUser(field)
		if err != nil {
			errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeScript, true)
			return false
		}
		flt = v.(string)
	}
	if flt == "" {
		t := b.Table.GetFieldType(field.Name)
		switch t {
		case typesfunc.TypeBoolean:
			flt = "false"
		case typesfunc.TypeDate, typesfunc.TypeTime, typesfunc.TypeDateTime:
			flt = "''"
		}
	}
	input = tview.NewInputField().SetText(flt).
		SetDoneFunc(func(key tcell.Key) {
			BrowseSubitemsFlex.RemoveItem(input)
			statefunc.App.SetRoot(statefunc.RunFlexLevel0, true)
			b.setBrowseFilter(input.GetText(), key)
		})
	input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return event
	})
	input.SetLabel(field.Caption)
	input.SetTitle("BROWSEFILTER")
	BrowseSubitemsFlex.AddItem(input, 0, 1, true)
	statefunc.App.SetRoot(BrowseSubitemsFlex, true)
	return true
}

func (b *TBrowse) getCurrentField() *TBrowseField {
	row, column := b.TableView.GetSelection()
	cell := b.TableView.GetCell(row, column)
	if cell == nil {
		return nil
	}
	field, ok := cell.GetReference().(TBrowseField) // Get the reference from the cell
	if !ok {
		return nil
	}
	return &field
}

func (b *TBrowse) onButtonPress(function string) {
	b.runButtonCallback(function)
}

func (b *TBrowse) runButtonCallback(function string) {
	defer func() {
		if r := recover(); r != nil {
			errorhandlefunc.ThrowError(r.(string), errorhandlefunc.ErrorTypeScript, true)
		}
	}()
	statefunc.L.Global(function) // Get the function from the Lua global environment
	if !statefunc.L.IsFunction(-1) {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_a_function", map[string]interface{}{
			"Name": function,
		}), errorhandlefunc.ErrorTypeScript, true)
		return
	}
	wrapper := &gormfunc.TableWrapper{Table: b.Table} // Create a wrapper for the Table
	statefunc.L.PushUserData(wrapper)                 // Push the wrapper as userdata
	statefunc.L.PushString("TableMT")
	statefunc.L.RawGet(lua.RegistryIndex)
	if statefunc.L.IsNil(-1) {
		statefunc.L.Pop(1)
		statefunc.L.PushString("TableMT metatable not found")
		statefunc.L.Error()
		return
	}
	statefunc.L.SetMetaTable(-2)

	statefunc.L.Call(1, 0) // Call the Lua function with the Table as an argument
}

func showBrowseLookup(browse *tview.Table) {
	browse.SetTitle("lookup")
	BrowseSubitemsFlex.AddItem(browse, 0, 1, true)
	statefunc.App.SetRoot(BrowseSubitemsFlex, true)
}
