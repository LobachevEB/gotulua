package helpsysfunc

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"gotulua/gormfunc"
	"gotulua/statefunc"
	"gotulua/uifunc"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type FunctionHelp struct {
	Name        string
	Parameters  string
	Description string
	IsHeader    bool // Used for grouping in the help dialog
}

var luaFunctions = []FunctionHelp{}

// extractMethodDoc extracts documentation for a struct method
func extractMethodDoc(structType reflect.Type, methodName string) (FunctionHelp, error) {
	// Find the method
	method, ok := structType.MethodByName(methodName)
	if !ok {
		return FunctionHelp{}, fmt.Errorf("method %s not found", methodName)
	}

	// Get the method's package path and file location
	pc := method.Func.Pointer()
	rfunc := runtime.FuncForPC(pc)
	if rfunc == nil {
		return FunctionHelp{}, fmt.Errorf("could not get function info")
	}

	filename, _ := rfunc.FileLine(0)

	// Parse the source file
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return FunctionHelp{}, fmt.Errorf("could not parse source: %v", err)
	}

	// Create package documentation
	pkg, err := doc.NewFromFiles(fset, []*ast.File{f}, "")
	if err != nil {
		return FunctionHelp{}, fmt.Errorf("could not get docs: %v", err)
	}

	// Find our type
	var typeDoc *doc.Type
	for _, t := range pkg.Types {
		// Get the actual type name, handling pointer types
		typeName := structType.Name()
		if structType.Kind() == reflect.Ptr {
			typeName = structType.Elem().Name()
		}
		if t.Name == typeName {
			typeDoc = t
			break
		}
	}

	if typeDoc == nil {
		return FunctionHelp{}, fmt.Errorf("type %s not found in docs", structType.Name())
	}

	// Find our method
	var methodDoc string
	for _, m := range typeDoc.Methods {
		if m.Name == methodName {
			methodDoc = m.Doc
			break
		}
	}

	// Get parameters info
	params := extractMethodParams(method.Type)

	return FunctionHelp{
		Name:        methodName,
		Parameters:  params,
		Description: methodDoc,
	}, nil
}

// extractMethodParams gets the parameter list from a method type, skipping the receiver
func extractMethodParams(t reflect.Type) string {
	if t == nil {
		return "()"
	}

	var params []string
	// Start from 1 to skip the receiver parameter
	for i := 1; i < t.NumIn(); i++ {
		paramType := t.In(i)
		params = append(params, paramType.String())
	}

	return fmt.Sprintf("(%s)", strings.Join(params, ", "))
}

// registerMethodForHelp registers help documentation for a Table method
func registerMethodForHelp(methodName, area string) error {
	var help FunctionHelp
	var err error
	switch area {
	case "Browse":
		help, err = extractMethodDoc(reflect.TypeOf(&uifunc.TBrowse{}), methodName)
	case "Table":
		help, err = extractMethodDoc(reflect.TypeOf(&gormfunc.Table{}), methodName)
	case "Form":
		help, err = extractMethodDoc(reflect.TypeOf(&uifunc.Form{}), methodName)
	}
	if err != nil {
		return err
	}

	luaFunctions = append(luaFunctions, help)
	return nil
}

// RegisterMethodsForHelp registers help documentation for all Table methods that are exposed to Lua
func RegisterMethodsForHelp(methods []string, area, description string) {
	luaFunctions = append(luaFunctions, FunctionHelp{
		Name:        fmt.Sprintf("Help for %s", area),
		Parameters:  "",
		Description: description,
		IsHeader:    true,
	})
	sort.Strings(methods)
	for _, methodName := range methods {
		if err := registerMethodForHelp(methodName, area); err != nil {
			fmt.Printf("Error registering help for method %s: %v\n", methodName, err)
		}
	}
}

func RegisterBrowseFunctions() {
	luaFunctions = append(append(luaFunctions, FunctionHelp{Name: "Browse functions description", Parameters: "", Description: "", IsHeader: true}),
		FunctionHelp{
			Name:        "AddField",
			Parameters:  "<description> string",
			Description: "AddField adds a field to the browse. Description is a string that contains field definitions separated by '|', where each field is defined by semicolon-separated key-value pairs (e.g., \"n::Name;c::Caption;f::Function;e::true;t::Type\"). Recognized keys are: \"n\": field name (required); \"c\": field caption (optional); \"f\": function name for computed fields (optional); \"e\": editable flag (\"true\" or \"false\", optional); \"t\": extra type information (optional). If a function is specified, AddFuncField is called; otherwise, AddTableField is used.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "SetFieldLookup",
			Parameters:  "<fieldName> string, <lookupTable> TBrowse, <lookupFunc> string",
			Description: "SetFieldLookup sets the lookup for the field. LookupTable is the lookup browse, LookupFunc is the lookup function.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "AddButton",
			Parameters:  "<caption> string, <function> string",
			Description: "AddButton adds a button to the browse. Caption is the button caption, function is the function to be called when the button is clicked.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "Show",
			Parameters:  "",
			Description: "Show shows the browse.",
			IsHeader:    false,
		},
	)
}

func RegisterTableFunctions() {
	luaFunctions = append(append(luaFunctions, FunctionHelp{Name: "Table functions description", Parameters: "", Description: "", IsHeader: true}),
		FunctionHelp{
			Name:        "Find",
			Parameters:  "",
			Description: "Find retrieves all filtered rows from the table and returns the true or false depending on the success.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "FindLast",
			Parameters:  "",
			Description: "FindLast retrieves the last filtered row from the table and returns the true or false depending on the success.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "FindByID",
			Parameters:  "<id> integer",
			Description: "FindByID retrieves a row from the table by its ID and returns the true or false depending on the success.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "Next",
			Parameters:  "",
			Description: "Next retrieves the next row from the table and returns the true or false depending on the success.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "Prev",
			Parameters:  "",
			Description: "Prev retrieves the previous row from the table and returns the true or false depending on the success.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "Insert",
			Parameters:  "",
			Description: "Insert inserts a new row into the table and returns the true or false depending on the success.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "Update",
			Parameters:  "",
			Description: "Update updates the current row in the table and returns the true or false depending on the success.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "SetFilter",
			Parameters:  "<field> string, [filter] string",
			Description: "SetFilter sets the filter for the table. If filter is not specified, the filter is cleared.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "OrderBy",
			Parameters:  "<field> string",
			Description: "OrderBy orders the table by the specified field.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "SetOnAfterDelete",
			Parameters:  "<function> function",
			Description: "SetOnAfterDelete sets the function to be called after a row is deleted.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "SetOnAfterUpdate",
			Parameters:  "<function> function",
			Description: "SetOnAfterUpdate sets the function to be called after a row is updated.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "SetOnAfterInsert",
			Parameters:  "<function> function",
			Description: "SetOnAfterInsert sets the function to be called after a row is inserted.",
			IsHeader:    false,
		},
	)
}

func RegisterCommonFunctions() {
	luaFunctions = append(append(luaFunctions, FunctionHelp{Name: "Common functions description", Parameters: "", Description: "", IsHeader: true}),
		FunctionHelp{
			Name:        "DBOpen",
			Parameters:  "<path> string",
			Description: "Opens a database connection. Returns a database object.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "DBClose",
			Parameters:  "<db> Database object",
			Description: "Closes a database connection.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "DBOpenTable",
			Parameters:  "<db> Database object, <tableName> string",
			Description: "Opens a table connection. Returns a table object.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "DBCreate",
			Parameters:  "<path> string",
			Description: "Creates a database. Returns a database object.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "DBCreateTable",
			Parameters:  "<db> Database object, <tableName> string, <description> string, <openIfExists> bool",
			Description: "Creates a table. Returns a table object. Description is a string that contains field definitions separated by '|', where each field is defined by semicolon-separated key-value pairs (e.g., \"n::Name;t::Type;l::Length\").",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "DBCreateTableTemp",
			Parameters:  "<db> Database object, <tableName> string, <description> string, <openIfExists> bool",
			Description: "Creates a temporary table. Returns a table object.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "DBDropTable",
			Parameters:  "<db> Database object, <tableName> string",
			Description: "Drops a table.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "DBAlterTable",
			Parameters:  "<db> Database object, <tableName> string, <structure> string",
			Description: "Alters a table. Returns a table object.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "SetDateFormat",
			Parameters:  "<format> string",
			Description: "Sets the date format.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "SetTimeFormat",
			Parameters:  "<format> string",
			Description: "Sets the time format.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "SetDateTimeFormat",
			Parameters:  "<format> string",
			Description: "Sets the date time format.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "Date",
			Parameters:  "",
			Description: "Returns the current date in the format specified by SetDateFormat.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "Time",
			Parameters:  "",
			Description: "Returns the current time in the format specified by SetTimeFormat.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "DateTime",
			Parameters:  "",
			Description: "Returns the current date and time in the format specified by SetDateTimeFormat.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "DateDiff",
			Parameters:  "<date1> string, <date2> string, <mode> string",
			Description: "Calculates the difference between two dates. mode can be 'd', 'D', 'm', 'M', 'y', 'Y'.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "TimeDiff",
			Parameters:  "<time1> string, <time2> string, <mode> string",
			Description: "Calculates the difference between two times. mode can be 'h', 'H', 'm', 'M', 's', 'S'.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "DateAdd",
			Parameters:  "<date> string, <year> int, <month> int, <day> int",
			Description: "Adds a specified number of years, months, and days to a date. year, month, day can be positive or negative.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "TimeAdd",
			Parameters:  "<time> string, <hour> int, <minute> int, <second> int",
			Description: "Adds a specified number of hours, minutes, and seconds to a time. hour, minute, second can be positive or negative.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "AddBrowse",
			Parameters:  "<table> Table object, <caption> string",
			Description: "Adds a general browse to the table.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "AddLookup",
			Parameters:  "<table> Table object, <caption> string",
			Description: "Adds a lookup browse to the table.",
			IsHeader:    false,
		},
		// FunctionHelp{
		// 	Name:        "AddForm",
		// 	Parameters:  "<caption> string",
		// 	Description: "Adds a form.",
		// 	IsHeader:    false,
		// },
		FunctionHelp{
			Name:        "Confirm",
			Parameters:  "<message> string",
			Description: "Shows a confirmation dialog.",
			IsHeader:    false,
		},
		FunctionHelp{
			Name:        "Message",
			Parameters:  "<message> string",
			Description: "Shows a message dialog.",
			IsHeader:    false,
		},
		// FunctionHelp{
		// 	Name:        "AddMenuItems",
		// 	Parameters:  "String in format 'Menu 1 caption, lua function;Menu 2 caption, lua function;...'",
		// 	Description: "Shows a message dialog.",
		// 	IsHeader:    false,
		// },
		// FunctionHelp{
		// 	Name:        "getLastError",
		// 	Parameters:  "",
		// 	Description: "Returns the last error message.",
		// 	IsHeader:    false,
		// },
		// FunctionHelp{
		// 	Name:        "clearErrors",
		// 	Parameters:  "",
		// 	Description: "Clears the error message.",
		// 	IsHeader:    false,
		// },
	)

}

var currentDialog tview.Primitive

func ShowHelp(fromEditor bool, callback func(functionName string)) {
	list := tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true)

	for _, fn := range luaFunctions {
		desc := fmt.Sprintf("Parameters: %s\n%s", fn.Parameters, fn.Description)
		desc = strings.ReplaceAll(desc, "]", "[[]")
		fname := fn.Name
		if fn.IsHeader {
			fname = "[red::]" + fname + "[-::]"
		}
		list.AddItem(fname, desc, 0, nil)
	}

	list.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if callback != nil {
			// Return function name with parameter placeholders
			params := strings.Trim(luaFunctions[index].Parameters, "()")
			paramList := strings.Split(params, ", ")
			placeholders := make([]string, len(paramList))
			for i := range paramList {
				if strings.Contains(paramList[i], "{}") {
					paramList[i] = "value"
				}
				if paramList[i] != "" {
					placeholders[i] = fmt.Sprintf("%s", paramList[i])
				} else {
					placeholders[i] = ""
				}
			}
			functionCall := fmt.Sprintf("%s(%s)", mainText, strings.Join(placeholders, ", "))
			callback(functionCall)
			closeDialog(fromEditor)
		}
	})

	list.SetBorder(true).SetTitle("Lua Function Help")

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			closeDialog(fromEditor)
			return nil
		}
		return event
	})

	showDialog(list, 120, 40)
}

// showDialog displays a dialog with the given content and dimensions
func showDialog(content tview.Primitive, width, height int) {
	modal := tview.NewFlex().
		AddItem(content, 0, 1, true)

	currentDialog = modal
	statefunc.App.SetRoot(modal, true)
}

// closeDialog closes the current dialog and restores the previous view
func closeDialog(fromEditor bool) {
	if currentDialog != nil {
		if fromEditor {
			statefunc.App.SetRoot(statefunc.EditorFlex, true)
		} else {
			statefunc.App.SetRoot(statefunc.MainFlex, true)
		}
		currentDialog = nil
	}
}
