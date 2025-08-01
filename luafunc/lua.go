package luafunc

import (
	"fmt"
	"gotulua/errorhandlefunc"
	"gotulua/gormfunc"
	"gotulua/helpsysfunc"
	"gotulua/i18nfunc"
	"gotulua/statefunc"
	"gotulua/timefunc"
	"gotulua/uifunc"
	"reflect"

	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Shopify/go-lua"
	"gorm.io/gorm"
)

type FuncDescr struct {
	Name        string
	Description string
}

// SetupRequireHandler sets up a custom require function that can dynamically load Lua modules
func SetupRequireHandler(L *lua.State, scriptPaths []string) {
	// Create package.preload table if it doesn't exist
	L.Global("package")
	if L.TypeOf(-1) != lua.TypeTable {
		L.Pop(1)
		L.NewTable()
		L.SetGlobal("package")
		L.Global("package")
	}

	// Check for preload table
	L.PushString("preload")
	L.RawGet(-2)
	if L.TypeOf(-1) != lua.TypeTable {
		L.Pop(1)
		L.NewTable()
		L.PushString("preload")
		L.PushValue(-2)
		L.RawSet(-4)
	}
	L.Pop(2) // pop preload table and package table

	// Register our custom require function
	RegisterGoFunction(L, "require", func(L *lua.State) int {
		if L.Top() < 1 {
			L.PushString("require needs a module name")
			L.Error()
			return 0
		}

		moduleName, ok := L.ToString(1)
		if !ok {
			L.PushString("module name must be a string")
			L.Error()
			return 0
		}

		// Check if module is already loaded
		L.Global("package")
		L.PushString("loaded")
		L.RawGet(-2)
		L.PushString(moduleName)
		L.RawGet(-2)
		if !L.IsNil(-1) {
			return 1 // module already loaded
		}
		L.Pop(3) // pop the loaded value, loaded table, and package table

		// Try to find the module file
		var moduleFile string
		for _, basePath := range scriptPaths {
			// Try different possible file paths
			possiblePaths := []string{
				filepath.Join(basePath, moduleName+".lua"),
				filepath.Join(basePath, strings.Replace(moduleName, ".", "/", -1)+".lua"),
			}

			for _, path := range possiblePaths {
				if _, err := os.Stat(path); err == nil {
					moduleFile = path
					break
				}
			}
			if moduleFile != "" {
				break
			}
		}

		if moduleFile == "" {
			L.PushString(fmt.Sprintf("module '%s' not found in search paths", moduleName))
			L.Error()
			return 0
		}

		// Load and execute the module
		if err := lua.DoFile(L, moduleFile); err != nil {
			L.PushString(fmt.Sprintf("error loading module '%s': %v", moduleName, err))
			L.Error()
			return 0
		}

		// If module didn't return a value, create an empty table
		if L.Top() == 0 {
			L.NewTable()
		}

		// Store the result in package.loaded
		L.Global("package")
		L.PushString("loaded")
		L.RawGet(-2)
		L.PushString(moduleName)
		L.PushValue(-5) // Push the module result
		L.RawSet(-3)    // package.loaded[moduleName] = result

		return 1 // Return the module result
	})
}

// Example Go functions that can be called from any Lua script
func getCurrentTime(L *lua.State) int {
	L.PushString(time.Now().Format(time.RFC3339))
	return 1
}

func addNumbers(L *lua.State) int {
	// Check we have exactly 2 arguments
	if L.Top() != 2 {
		L.PushString("addNumbers requires exactly 2 numbers")
		L.Error()
		return 0
	}

	// Get and validate arguments
	if !L.IsNumber(1) || !L.IsNumber(2) {
		L.PushString("both arguments must be numbers")
		L.Error()
		return 0
	}

	a, ok1 := L.ToNumber(1)
	b, ok2 := L.ToNumber(2)
	if !ok1 || !ok2 {
		L.PushString("error converting arguments to numbers")
		L.Error()
		return 0
	}

	// Return result
	L.PushNumber(a + b)
	return 1
}

// callModuleFunction calls a function from a module by its string name
func callModuleFunction(L *lua.State) int {
	// Check arguments
	if L.Top() < 2 {
		L.PushString("callModuleFunction requires at least 2 arguments: module and function name")
		L.Error()
		return 0
	}

	// Get module
	if !L.IsTable(1) {
		L.PushString("first argument must be a module (table)")
		L.Error()
		return 0
	}

	// Get function name
	funcName, ok := L.ToString(2)
	if !ok {
		L.PushString("second argument must be a string (function name)")
		L.Error()
		return 0
	}

	// Get the function from the module
	L.PushValue(1) // Push module table
	L.PushString(funcName)
	L.RawGet(-2) // Get module[funcName]

	if !L.IsFunction(-1) {
		L.PushString(fmt.Sprintf("function '%s' not found in module", funcName))
		L.Error()
		return 0
	}

	// Move function before arguments
	L.Insert(1)
	// Remove module table
	L.Remove(2)

	// Call the function with remaining arguments (if any)
	if err := L.ProtectedCall(L.Top()-1, 1, 0); err != nil {
		L.PushString(fmt.Sprintf("error calling function '%s': %v", funcName, err))
		L.Error()
		return 0
	}

	return 1 // Return the function's result
}

// findAndCallLuaFunction searches for a function by name in all loaded modules and calls it
func findAndCallLuaFunction(L *lua.State, funcName string, args ...interface{}) error {
	// Get package.loaded table which contains all loaded modules
	L.Global("package")
	L.PushString("loaded")
	L.RawGet(-2)
	if L.TypeOf(-1) != lua.TypeTable {
		L.Pop(2) // pop nil and package table
		return fmt.Errorf("package.loaded table not found")
	}
	L.Remove(-2) // remove package table

	// Iterate through all loaded modules
	L.PushNil() // first key
	for L.Next(-2) {
		if L.TypeOf(-1) == lua.TypeTable {
			// Check if this module has our function
			L.PushString(funcName)
			L.RawGet(-2)
			if L.IsFunction(-1) {
				// Found the function! Now set up the call

				// Push arguments
				for _, arg := range args {
					switch v := arg.(type) {
					case string:
						L.PushString(v)
					case int:
						L.PushInteger(v)
					case float64:
						L.PushNumber(v)
					case bool:
						L.PushBoolean(v)
					case nil:
						L.PushNil()
					default:
						return fmt.Errorf("unsupported argument type: %T", arg)
					}
				}

				// Call the function
				if err := L.ProtectedCall(len(args), 1, 0); err != nil {
					return fmt.Errorf("error calling function '%s': %v", funcName, err)
				}

				// Function called successfully
				return nil
			}
			L.Pop(1) // pop function (or nil)
		}
		L.Pop(1) // pop value, keep key for next iteration
	}
	L.Pop(1) // pop package.loaded table

	return fmt.Errorf("function '%s' not found in any loaded module", funcName)
}

// findLuaFunction is a Go function that can be called from Lua to find and execute a function by name
func findLuaFunction(L *lua.State) int {
	// Check arguments
	if L.Top() < 1 {
		L.PushString("findLuaFunction requires at least 1 argument: function name")
		L.Error()
		return 0
	}

	// Get function name
	funcName, ok := L.ToString(1)
	if !ok {
		L.PushString("first argument must be a string (function name)")
		L.Error()
		return 0
	}

	// Call FindAndCallLuaFunction with any additional arguments
	args := make([]interface{}, L.Top()-1)
	for i := 2; i <= L.Top(); i++ {
		switch {
		case L.IsString(i):
			args[i-2], _ = L.ToString(i)
		case L.IsNumber(i):
			args[i-2], _ = L.ToNumber(i)
		case L.IsBoolean(i):
			args[i-2] = L.ToBoolean(i)
		case L.IsNil(i):
			args[i-2] = nil
		default:
			L.PushString(fmt.Sprintf("unsupported argument type at position %d", i))
			L.Error()
			return 0
		}
	}

	if err := findAndCallLuaFunction(L, funcName, args...); err != nil {
		L.PushString(err.Error())
		L.Error()
		return 0
	}

	return 1 // return the result from the called function
}

func CreateLuaInterpreter() (*lua.State, []uifunc.InputField) {
	uifunc.InputFields = []uifunc.InputField{} // Initialize the InputFields slice
	// Initialize the Widgets slice
	//uifunc.Widgets = []uifunc.Widget{}
	statefunc.L = lua.NewState()
	lua.OpenLibraries(statefunc.L)

	// Set up the require handler with default script paths
	scriptPaths := []string{
		".",                   // current directory
		"scripts",             // scripts subdirectory
		os.Getenv("LUA_PATH"), // environment variable if set
	}
	SetupRequireHandler(statefunc.L, scriptPaths)

	// Register the Table type with a metatable
	registerTableType(statefunc.L)
	registerBrowseType(statefunc.L)
	registerFormType(statefunc.L)

	// Register utility Go functions that can be called from any Lua script
	statefunc.L.Register("getCurrentTime", getCurrentTime)
	statefunc.L.Register("addNumbers", addNumbers)
	statefunc.L.Register("findLuaFunction", findLuaFunction) // Add the new helper function

	// Register the UI functions with the Lua interpreter
	// statefunc.L.Register("AddInputField", uifunc.AddInputField)
	// statefunc.L.Register("AddTextView", uifunc.AddTextView)
	// statefunc.L.Register("SetTextViewText", uifunc.SetTextViewText)
	statefunc.L.Register("DBOpen", dbOpen)
	statefunc.L.Register("DBClose", dbClose)
	statefunc.L.Register("DBOpenTable", dbOpenTable)
	statefunc.L.Register("DBCreate", dbCreate)
	statefunc.L.Register("DBCreateTable", dbCreateTable)
	statefunc.L.Register("DBCreateTableTemp", dbCreateTableTemp)
	statefunc.L.Register("DBAlterTable", dbAlterTable)
	statefunc.L.Register("DBDropTable", dbDropTable)
	statefunc.L.Register("SetDateFormat", setDateFormat)
	statefunc.L.Register("SetTimeFormat", setTimeFormat)
	statefunc.L.Register("SetDateTimeFormat", setDateTimeFormat)
	statefunc.L.Register("Date", date)
	statefunc.L.Register("Time", getTime)
	statefunc.L.Register("DateTime", dateTime)
	statefunc.L.Register("DateDiff", dateDiff)
	statefunc.L.Register("TimeDiff", timeDiff)
	statefunc.L.Register("DateAdd", dateAdd)
	statefunc.L.Register("TimeAdd", timeAdd)
	statefunc.L.Register("AddBrowse", addBrowse)
	statefunc.L.Register("AddLookup", addLookup)
	statefunc.L.Register("AddForm", uifunc.AddForm)
	statefunc.L.Register("Confirm", confirm)
	statefunc.L.Register("Message", message)
	statefunc.L.Register("getLastError", getLastError)
	statefunc.L.Register("clearErrors", clearErrors)

	//registerUserMenuFunctions() // TODO: fix function call stuck problem
	registerHelpData()
	return statefunc.L, uifunc.InputFields
}

// registerTableType registers the Table type with Lua
func registerTableType(L *lua.State) {
	// Create the metatable
	L.NewTable()

	// Create a table for methods
	L.NewTable()

	// Add methods to the method table
	tableMethods := map[string]lua.Function{
		"Next": func(L *lua.State) int {
			// wrapper := checkTable(L)
			// if wrapper == nil {
			// 	return 0
			// }
			// L.PushBoolean(wrapper.Table.Next())
			return next(L)
		},
		"Prev": func(L *lua.State) int {
			return prev(L)
			// wrapper := checkTable(L)
			// if wrapper == nil {
			// 	return 0
			// }
			// L.PushBoolean(wrapper.Table.Prev())
			// return 1
		},
		"Find": func(L *lua.State) int {
			// wrapper := checkTable(L)
			// if wrapper == nil {
			// 	return 0
			// }
			// L.PushBoolean(wrapper.Table.Find())
			return find(L)
		},
		"FindByID": func(L *lua.State) int {
			return findByID(L)
			// wrapper := checkTable(L)
			// if wrapper == nil {
			// 	return 0
			// }
			// if L.Top() < 2 {
			// 	L.PushString("FindByID requires an ID parameter")
			// 	L.Error()
			// 	return 0
			// }
			// var id interface{}
			// switch {
			// case L.IsNumber(2):
			// 	val, ok := L.ToNumber(2)
			// 	if !ok {
			// 		L.PushBoolean(false)
			// 		return 1
			// 	}
			// 	id = int64(val)
			// case L.IsString(2):
			// 	val, ok := L.ToString(2)
			// 	if !ok {
			// 		L.PushBoolean(false)
			// 		return 1
			// 	}
			// 	id = val
			// default:
			// 	L.PushBoolean(false)
			// 	return 1
			// }
			// L.PushBoolean(wrapper.Table.FindByID(id))
			// return 1
		},
		"Insert": func(L *lua.State) int {
			return insert(L)
			// 	wrapper := checkTable(L)
			// 	if wrapper == nil {
			// 		return 0
			// 	}
			// 	if !L.IsTable(2) {
			// 		L.PushString("Insert requires a table parameter")
			// 		L.Error()
			// 		return 0
			// 	}

			// 	// Convert Lua table to map
			// 	fields := make(map[string]interface{})
			// 	L.PushNil() // First key
			// 	for L.Next(-2) {
			// 		// Key at -2, value at -1
			// 		key, ok := L.ToString(-2)
			// 		if !ok {
			// 			L.Pop(1) // Pop value, keep key for next iteration
			// 			continue
			// 		}

			// 		switch {
			// 		case L.IsString(-1):
			// 			val, ok := L.ToString(-1)
			// 			if ok {
			// 				fields[key] = val
			// 			}
			// 		case L.IsNumber(-1):
			// 			val, ok := L.ToNumber(-1)
			// 			if ok {
			// 				fields[key] = val
			// 			}
			// 		case L.IsBoolean(-1):
			// 			fields[key] = L.ToBoolean(-1)
			// 		case L.IsNil(-1):
			// 			fields[key] = nil
			// 		}
			// 		L.Pop(1) // Pop value, keep key for next iteration
			// 	}

			// 	var id int64
			// 	success := wrapper.Table.Insert(fields, &id)
			// 	L.PushBoolean(success)
			// 	if success {
			// 		L.PushNumber(float64(id))
			// 		return 2
			// 	}
			// 	return 1
		},
		"Update": func(L *lua.State) int {
			return update(L)
		},
		// "SetField": func(L *lua.State) int {
		// 	return SetField(L)
		// },
		"SetFilter": func(L *lua.State) int {
			return setFilter(L)
		},
		"SetRangeFilter": func(L *lua.State) int {
			return setRangeFilter(L)
			// wrapper := checkTable(L)
			// if wrapper == nil {
			// 	return 0
			// }
			// if L.Top() < 4 {
			// 	L.PushString("SetRangeFilter requires field, min, and max parameters")
			// 	L.Error()
			// 	return 0
			// }
			// field, ok := L.ToString(2)
			// if !ok {
			// 	L.PushBoolean(false)
			// 	return 1
			// }

			// var min, max interface{}
			// switch {
			// case L.IsNumber(3):
			// 	val, ok := L.ToNumber(3)
			// 	if ok {
			// 		min = val
			// 	}
			// case L.IsString(3):
			// 	val, ok := L.ToString(3)
			// 	if ok {
			// 		min = val
			// 	}
			// }

			// switch {
			// case L.IsNumber(4):
			// 	val, ok := L.ToNumber(4)
			// 	if ok {
			// 		max = val
			// 	}
			// case L.IsString(4):
			// 	val, ok := L.ToString(4)
			// 	if ok {
			// 		max = val
			// 	}
			// }

			// wrapper.Table.SetRangeFilter(field, min, max)
			// L.PushBoolean(true)
			// return 1
		},
		"OrderBy": func(L *lua.State) int {
			return setOrderBy(L)
			// wrapper := checkTable(L)
			// if wrapper == nil {
			// 	return 0
			// }
			// if L.Top() < 2 {
			// 	L.PushString("OrderBy requires an order parameter")
			// 	L.Error()
			// 	return 0
			// }
			// order, ok := L.ToString(2)
			// if !ok {
			// 	L.PushBoolean(false)
			// 	return 1
			// }
			// wrapper.Table.OrderBy(order)
			// L.PushBoolean(true)
			// return 1
		},
		"SetOnAfterDelete": func(L *lua.State) int {
			wrapper := checkTable(L)
			if wrapper == nil {
				return 0
			}
			if L.Top() < 2 {
				L.PushString("SetOnAfterDelete requires a function name parameter")
				L.Error()
				return 0
			}
			funcName, ok := L.ToString(2)
			if !ok {
				L.PushBoolean(false)
				return 1
			}
			wrapper.Table.SetOnAfterDelete(funcName)
			L.PushBoolean(true)
			return 1
		},
		"SetOnAfterUpdate": func(L *lua.State) int {
			wrapper := checkTable(L)
			if wrapper == nil {
				return 0
			}
			if L.Top() < 2 {
				L.PushString("SetOnAfterUpdate requires a function name parameter")
				L.Error()
				return 0
			}
			funcName, ok := L.ToString(2)
			if !ok {
				L.PushBoolean(false)
				return 1
			}
			wrapper.Table.SetOnAfterUpdate(funcName)
			L.PushBoolean(true)
			return 1
		},
		"SetOnAfterInsert": func(L *lua.State) int {
			wrapper := checkTable(L)
			if wrapper == nil {
				return 0
			}
			if L.Top() < 2 {
				L.PushString("SetOnAfterInsert requires a function name parameter")
				L.Error()
				return 0
			}
			funcName, ok := L.ToString(2)
			if !ok {
				L.PushBoolean(false)
				return 1
			}
			wrapper.Table.SetOnAfterInsert(funcName)
			L.PushBoolean(true)
			return 1
		},
	}

	// Register methods in the method table
	for name, fn := range tableMethods {
		L.PushString(name)
		L.PushGoFunction(fn)
		L.RawSet(-3)
	}

	// Store methods table in registry for __index to use
	L.PushString("TableMethods")
	L.PushValue(-2) // Copy the methods table
	L.RawSet(lua.RegistryIndex)
	L.Pop(1) // Remove methods table

	// Set __index metamethod
	L.PushString("__index")
	L.PushGoFunction(func(L *lua.State) int {
		if !L.IsString(2) {
			L.PushNil()
			return 1
		}

		// Get the wrapper
		wrapper := checkTable(L)
		if wrapper == nil {
			L.PushNil()
			return 1
		}

		key, ok := L.ToString(2)
		if !ok {
			L.PushNil()
			return 1
		}

		// First check method table
		L.PushString("TableMethods")
		L.RawGet(lua.RegistryIndex)
		L.PushString(key)
		L.RawGet(-2)
		if !L.IsNil(-1) {
			return 1
		}
		L.Pop(2) // pop nil and method table

		// If not a method, get field value
		val := wrapper.Table.GetField(key, "")
		if val == nil {
			errorhandlefunc.ThrowError(i18nfunc.T("error.db_field_not_found", map[string]interface{}{
				"Field": key,
				"Table": wrapper.Table.Name,
			}), errorhandlefunc.ErrorTypeScript, true)
			return 0
		}
		switch v := val.(type) {
		case string:
			L.PushString(v)
		case int:
			L.PushInteger(v)
		case int64:
			L.PushInteger(int(v))
		case float64:
			L.PushNumber(v)
		case bool:
			L.PushBoolean(v)
		case nil:
			L.PushNil()
		default:
			t := fmt.Sprintf("%v", reflect.TypeOf(val))
			if t == "*interface {}" {
				var vp *interface{} = val.(*interface{})
				vv := *vp
				L.PushString(vv.(string))
			} else {
				L.PushString(fmt.Sprintf("%v", v))
			}
		}
		return 1
	})
	L.RawSet(-3)

	// Set __newindex metamethod
	L.PushString("__newindex")
	L.PushGoFunction(func(L *lua.State) int {
		if !L.IsString(2) {
			return 0
		}

		wrapper := checkTable(L)
		if wrapper == nil {
			return 0
		}

		key, ok := L.ToString(2)
		if !ok {
			return 0
		}
		val := wrapper.Table.GetField(key, "")
		if val == nil {
			errorhandlefunc.ThrowError(i18nfunc.T("error.db_field_not_found", map[string]interface{}{
				"Field": key,
				"Table": wrapper.Table.Name,
			}), errorhandlefunc.ErrorTypeScript, true)
			return 0
		}

		// Convert the value based on its Lua type
		var value interface{}
		switch {
		case L.IsNumber(3):
			val, ok := L.ToNumber(3)
			if !ok {
				return 0
			}
			value = val
		case L.IsString(3):
			val, ok := L.ToString(3)
			if !ok {
				return 0
			}
			value = val
		case L.IsBoolean(3):
			value = L.ToBoolean(3)
		case L.IsNil(3):
			value = nil
		default:
			val, ok := L.ToString(3)
			if !ok {
				return 0
			}
			value = val
		}

		// Set the field value
		wrapper.Table.SetField(key, value)
		return 0
	})
	L.RawSet(-3)

	// Store the metatable in the registry
	L.PushString("TableMT")
	L.PushValue(-2) // Copy the metatable
	L.RawSet(lua.RegistryIndex)

	// Register help documentation for the table methods
}

// checkTable checks if the first argument is a Table and returns it
func checkTable(L *lua.State) *gormfunc.TableWrapper {
	if !L.IsUserData(1) {
		L.PushString("expected table object")
		L.Error()
		return nil
	}

	// Get the userdata and try to convert it
	ud := L.ToUserData(1)
	if ud == nil {
		L.PushString("nil userdata")
		L.Error()
		return nil
	}

	// Try to convert to TableWrapper
	if wrapper, ok := ud.(*gormfunc.TableWrapper); ok {
		return wrapper
	}

	// If we get here, it's the wrong type
	L.PushString("invalid table object")
	L.Error()
	return nil
}

func registerBrowseType(L *lua.State) {
	// Create a new metatable for Browse
	L.NewTable() // stack: [metatable]
	// Set __index to a table with methods
	L.NewTable() // stack: [metatable, __index]
	L.PushGoFunction(uifunc.AddTableField)
	L.SetField(-2, "AddTableField") // __index.BrowseTableAddField = BrowseTableAddField
	L.PushGoFunction(uifunc.AddFuncField)
	L.SetField(-2, "AddFuncField") // __index.BrowseTableAddField = BrowseTableAddField
	L.PushGoFunction(uifunc.AddField)
	L.SetField(-2, "AddField") // __index.BrowseTableAddField = BrowseTableAddField
	L.PushGoFunction(uifunc.SetFieldLookup)
	L.SetField(-2, "SetFieldLookup") // __index.BrowseTableAddField = BrowseTableAddField
	L.PushGoFunction(uifunc.AddButton)
	L.SetField(-2, "AddButton") // __index.BrowseTableAddField = BrowseTableAddField
	L.PushGoFunction(browseTable)
	L.SetField(-2, "Show") // __index.BrowseTable = BrowseTable
	// Set the metatable for the Browse type
	L.SetField(-2, "__index") // metatable.__index = __index
	// Register the metatable globally (optional, for reuse)
	L.SetGlobal("BrowseMT")

}

func registerFormType(L *lua.State) {
	// Create a new metatable for Form
	L.NewTable() // stack: [metatable]
	// Set __index to a table with methods
	L.NewTable() // stack: [metatable, __index]
	L.PushGoFunction(uifunc.AddForm)
	L.SetField(-2, "AddForm") // __index.FormAddField = FormAddField
	L.PushGoFunction(uifunc.FormShow)
	L.SetField(-2, "Show") // __index.FormShow = FormShow
	L.PushGoFunction(uifunc.AddInputField)
	L.SetField(-2, "AddInput") // __index.FormAddInput = FormAddInput
	// L.PushGoFunction(uifunc.AddDropDown)
	// L.SetField(-2, "AddDropDown") // __index.FormAddDropDown = FormAddDropDown
	// L.PushGoFunction(uifunc.AddCheckBox)
	// L.SetField(-2, "AddCheckBox") // __index.FormAddCheckBox = FormAddCheckBox
	L.PushGoFunction(uifunc.FormAddButton)
	L.SetField(-2, "AddButton") // __index.FormAddButton = FormAddButton
	// Set the metatable for the Form type
	L.SetField(-2, "__index") // metatable.__index = __index
	// Register the metatable globally (optional, for reuse)
	L.SetGlobal("FormMT")

}

func registerUserMenuFunctions() {
	statefunc.L.Register("AddMenu", uifunc.NewUserMenu)
	statefunc.L.Register("AddMenuItems", addMenuItems)
	statefunc.L.Register("AddMenuItem", addMenuItem)
	statefunc.L.Register("RemoveMenuItem", removeMenuItem)
	statefunc.L.Register("DisableMenuItem", disableMenuItem)
	statefunc.L.Register("EnableMenuItem", enableMenuItem)
}

func registerHelpData() {
	//formMethods := []string{ // TODO: until form is completely implemented
	//	"AddForm",
	//	"AddInput",
	//	"AddButton",
	//	"Show",
	//}
	// browseMethods := []string{
	// 	"AddField",
	// 	"SetFieldLookup",
	// 	"AddButton",
	// 	"Show",
	// }
	// tableMethods := []string{
	// 	"Find",
	// 	"FindByID",
	// 	"Next",
	// 	"Prev",
	// 	//"GetField",
	// 	//"SetField",
	// 	"Insert",
	// 	"Update",
	// 	"SetFilter",
	// 	//"SetRangeFilter",
	// 	"SetOnAfterDelete",
	// 	"SetOnAfterUpdate",
	// 	"SetOnAfterInsert",
	// 	"OrderBy",
	// }
	helpsysfunc.RegisterCommonFunctions()
	helpsysfunc.RegisterTableFunctions()  //RegisterMethodsForHelp(tableMethods, "Table", i18nfunc.T("help.table.description", nil))
	helpsysfunc.RegisterBrowseFunctions() //RegisterMethodsForHelp(browseMethods, "Browse", i18nfunc.T("help.browse.description", nil))
	//helpsysfunc.RegisterMethodsForHelp(formMethods, "Form", i18nfunc.T("help.form.description", nil)) // TODO: until form is completely implemented

}

func addMenuItems(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "AddMenuItems",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	items, ok := L.ToString(1)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "items",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	return uifunc.AddMenuItems(items)
}

func addMenuItem(L *lua.State) int {
	if L.Top() < 2 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "AddMenuItem",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	caption, ok := L.ToString(1)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "caption",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	funcName, ok := L.ToString(2)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "function name",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	return uifunc.AddMenuItem(caption, funcName)
}

func removeMenuItem(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "RemoveMenuItem",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	caption, ok := L.ToString(1)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "caption",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	return uifunc.RemoveMenuItem(caption)
}

func disableMenuItem(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "DisableMenuItem",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	caption, ok := L.ToString(1)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "caption",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	return uifunc.DisableMenuItem(caption)
}

func enableMenuItem(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "EnableMenuItem",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	caption, ok := L.ToString(1)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "caption",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	return uifunc.EnableMenuItem(caption)
}

// Register Date, Time, DateTime formats in Lua >>>>>>>>>>>>>>>>>>>>>>
func setDateFormat(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "SetDateFormat",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	dateFormat, ok := L.ToString(1)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "date format",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	timefunc.SetDateFormat(dateFormat)
	return 1
}

func setTimeFormat(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "SetTimeFormat",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	timeFormat, ok := L.ToString(1)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "time format",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	timefunc.SetTimeFormat(timeFormat)
	return 1
}

func setDateTimeFormat(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "SetDateTimeFormat",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	dateTimeFormat, ok := L.ToString(1)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "datetime format",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	timefunc.SetDateTimeFormat(dateTimeFormat)
	return 1
}

// date returns the current date in the format specified by SetDateFormat
func date(L *lua.State) int {
	L.PushString(timefunc.Date())
	return 1
}

// getTime returns the current time in the format specified by SetTimeFormat
func getTime(L *lua.State) int {
	L.PushString(timefunc.Time())
	return 1
}

// dateTime returns the current date and time in the format specified by SetDateTimeFormat
func dateTime(L *lua.State) int {
	L.PushString(timefunc.DateTime())
	return 1
}

func dateDiff(L *lua.State) int {
	if L.Top() < 2 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "DateDiff",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	start, ok := L.ToString(1)
	if !ok {
		x := L.ToValue(1)
		if x != nil {
			errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
				"Name": "start date",
			}), errorhandlefunc.ErrorTypeScript, true)
			return 0
		}
	}
	end, ok := L.ToString(2)
	if !ok {
		x := L.ToValue(2)
		if x != nil {
			errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
				"Name": "end date",
			}), errorhandlefunc.ErrorTypeScript, true)
			return 0
		}
	}
	mode, ok := L.ToString(3)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "mode",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	if mode != "d" && mode != "D" && mode != "m" && mode != "M" && mode != "y" && mode != "Y" {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_valid", map[string]interface{}{
			"Argument": mode,
			"Valid":    "d, D, m, M, y, Y",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	r := timefunc.DateDiff(start, end, mode)
	L.PushInteger(int(r))
	return 1
}

func timeDiff(L *lua.State) int {
	if L.Top() < 2 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "TimeDiff",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	start, ok := L.ToString(1)
	if !ok {
		x := L.ToValue(1)
		if x != nil {
			errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
				"Name": "start time",
			}), errorhandlefunc.ErrorTypeScript, true)
			return 0
		}
	}
	end, ok := L.ToString(2)
	if !ok {
		x := L.ToValue(2)
		if x != nil {
			errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
				"Name": "end time",
			}), errorhandlefunc.ErrorTypeScript, true)
			return 0
		}
	}
	mode, ok := L.ToString(3)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "mode",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	if mode != "h" && mode != "H" && mode != "m" && mode != "M" && mode != "s" && mode != "S" {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_valid", map[string]interface{}{
			"Argument": mode,
			"Valid":    "h, H, m, M, s, S",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	r := timefunc.TimeDiff(start, end, mode)
	L.PushInteger(int(r))
	return 1
}

// Register Date, Time, DateTime formats in Lua <<<<<<<<<<<<<<<<

// Register the UI functions with the Lua interpreter >>>>>>>>>>>>>>>>>>>>>>
func addBrowse(L *lua.State) int {
	return uifunc.BrowseTableNew(L, false)
}

func addLookup(L *lua.State) int {
	return uifunc.BrowseTableNew(L, true)
}

func dateAdd(L *lua.State) int {
	if L.Top() < 2 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "DateAdd",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	date, ok := L.ToString(1)
	if !ok {
		x := L.ToValue(1)
		if x != nil {
			errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
				"Name": "date",
			}), errorhandlefunc.ErrorTypeScript, true)
			return 0
		}
	}
	year, ok := L.ToInteger(2)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_int", map[string]interface{}{
			"Name": "year",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	month, ok := L.ToInteger(3)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_int", map[string]interface{}{
			"Name": "month",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	day, ok := L.ToInteger(4)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_int", map[string]interface{}{
			"Name": "day",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	mode, ok := L.ToString(3)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "mode",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	if mode != "d" && mode != "D" && mode != "m" && mode != "M" && mode != "y" && mode != "Y" {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_valid", map[string]interface{}{
			"Argument": mode,
			"Valid":    "d, D, m, M, y, Y",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	r := timefunc.DateAdd(date, year, month, day)
	L.PushString(r)
	return 1
}

func timeAdd(L *lua.State) int {
	if L.Top() < 2 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "TimeAdd",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	time, ok := L.ToString(1)
	if !ok {
		x := L.ToValue(1)
		if x != nil {
			errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
				"Name": "time",
			}), errorhandlefunc.ErrorTypeScript, true)
			return 0
		}
	}
	hour, ok := L.ToInteger(2)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_int", map[string]interface{}{
			"Name": "hour",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	minute, ok := L.ToInteger(3)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_int", map[string]interface{}{
			"Name": "minute",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	second, ok := L.ToInteger(4)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_int", map[string]interface{}{
			"Name": "second",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	mode, ok := L.ToString(3)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "mode",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	if mode != "d" && mode != "D" && mode != "m" && mode != "M" && mode != "y" && mode != "Y" {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_valid", map[string]interface{}{
			"Argument": mode,
			"Valid":    "d, D, m, M, y, Y",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	r := timefunc.TimeAdd(time, hour, minute, second)
	L.PushString(r)
	return 1
}

func browseTable(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.extra_args", map[string]interface{}{
			"Name": "BrowseTable",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	browse, ok := L.ToUserData(1).(*uifunc.TBrowse) // Get the browse from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_browse", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	browse.Show(L) // Call the BrowseTable method on the browse
	return 1       // Return the number of results
}

func confirm(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "Confirm",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	text, ok := L.ToString(1)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "text",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	uifunc.Confirm(text, func(ok bool) {
		L.PushBoolean(ok)
	})
	return 1
}

func message(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "Message",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	text, ok := L.ToString(1)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "text",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	uifunc.Message(text)
	return 1
}

// Get the last error message
func getLastError(L *lua.State) int {
	L.PushString(statefunc.GetLastErrorText())
	return 1
}

// Clear the error message
func clearErrors(L *lua.State) int {
	statefunc.ClearErrors()
	return 1
}

// Register the UI functions with the Lua interpreter <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

// Register the database functions >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
func dbOpen(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "DBOpen",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	path, ok := L.ToString(1) // Get the database path from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "database path",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	db := gormfunc.OpenDB(path) // Open the database
	if db == nil {
		errorhandlefunc.ThrowError(i18nfunc.T("error.db_open", map[string]interface{}{
			"Name": path,
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	L.PushUserData(db) // Push the database as userdata
	return 1           // Return the number of results
}

func dbClose(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "DBClose",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	db, ok := L.ToUserData(1).(*gorm.DB) // Get the database from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_db", map[string]interface{}{
			"Name": "database",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	err := gormfunc.CloseDB(db) // Close the database
	if err != nil {
		errorhandlefunc.ThrowError(i18nfunc.T("error.db_close", map[string]interface{}{
			"Error": err.Error(),
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	return 0 // Return success
}

func dbOpenTable(L *lua.State) int {
	if L.Top() < 2 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "DBOpenTable",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}

	// Get the database from Lua
	ud := L.ToUserData(1)
	if ud == nil {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_db", map[string]interface{}{
			"Name": "database",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	db, ok := ud.(*gorm.DB)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_db", map[string]interface{}{
			"Name": "database",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}

	// Get the table name from Lua
	tableName, ok := L.ToString(2)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "table name",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}

	// Open the table
	table := gormfunc.OpenTable(db, tableName)
	if table == nil {
		errorhandlefunc.ThrowError(i18nfunc.T("error.table_open_failed", map[string]interface{}{
			"Name": tableName,
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}

	// Create a new wrapper
	wrapper := &gormfunc.TableWrapper{Table: table}

	// Push the wrapper as userdata
	L.PushUserData(wrapper)

	// Get and set the metatable from registry
	L.PushString("TableMT")
	L.RawGet(lua.RegistryIndex)
	if L.IsNil(-1) {
		L.Pop(1)
		L.PushString("TableMT metatable not found")
		L.Error()
		return 0
	}
	L.SetMetaTable(-2)
	return 1
}

func dbCreate(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "DBCreate",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	dbName, ok := L.ToString(1) // Get the table name from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "table name",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	db, err := gormfunc.CreateDB(dbName) // Create the database
	if err != nil {
		errorhandlefunc.ThrowError(i18nfunc.T("error.db_create_failed", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	L.PushUserData(db) // Push the database as userdata
	return 1           // Return the number of results
}

func dbCreateTable(L *lua.State) int {
	if L.Top() < 4 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "DBCreateTable",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	db, ok := L.ToUserData(1).(*gorm.DB) // Get the database from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_db", map[string]interface{}{
			"Name": "database",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	tableName, ok := L.ToString(2) // Get the table name from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "table name",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	description, ok := L.ToString(3) // Get the table description from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "table description",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	openIfExists := L.ToBoolean(4) // Get the skip check from Lua

	table := gormfunc.CreateTable(db, tableName, description, openIfExists, false) // Create the table
	L.PushUserData(table)                                                          // Push the table as userdata
	L.Global("TableMT")                                                            // Push the metatable
	L.SetMetaTable(-2)                                                             // Set metatable for userdata
	return 1                                                                       // Return the number of results
}

func dbCreateTableTemp(L *lua.State) int {
	if L.Top() < 4 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "DBCreateTableTemp",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	db, ok := L.ToUserData(1).(*gorm.DB) // Get the database from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_db", map[string]interface{}{
			"Name": "database",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	tableName, ok := L.ToString(2) // Get the table name from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "table name",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	description, ok := L.ToString(3) // Get the table description from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "table description",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	openIfExists := L.ToBoolean(4) // Get the skip check from Lua

	table := gormfunc.CreateTable(db, tableName, description, openIfExists, true) // Create the table
	L.PushUserData(table)                                                         // Push the table as userdata
	L.Global("TableMT")                                                           // Push the metatable
	L.SetMetaTable(-2)                                                            // Set metatable for userdata
	return 1                                                                      // Return the number of results
}

func dbAlterTable(L *lua.State) int {
	if L.Top() < 3 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "DBAlterTable",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	db, ok := L.ToUserData(1).(*gorm.DB) // Get the database from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_db", map[string]interface{}{
			"Name": "database",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	tableName, ok := L.ToString(2) // Get the table name from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "table name",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	structure, ok := L.ToString(3) // Get the table description from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "table description",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}

	table := gormfunc.AlterTable(db, tableName, structure) // Alter the table
	L.PushUserData(table)                                  // Push the table as userdata
	L.Global("TableMT")                                    // Push the metatable
	L.SetMetaTable(-2)                                     // Set metatable for userdata
	return 1                                               // Return the number of results
}

func dbDropTable(L *lua.State) int {
	if L.Top() < 2 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "DBDropTable",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	db, ok := L.ToUserData(1).(*gorm.DB) // Get the database from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_db", map[string]interface{}{
			"Name": "database",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	tableName, ok := L.ToString(2) // Get the table name from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "table name",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	err := gormfunc.DropTable(db, tableName) // Drop the table
	if err != nil {
		errorhandlefunc.ThrowError(i18nfunc.T("error.db_drop_table_failed", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	return 1 // Return success
}

func setFilter(L *lua.State) int {
	if L.Top() < 2 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "SetFilter",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	wrapper := checkTable(L)
	if wrapper == nil {
		return 0
	}
	field, ok := L.ToString(2) // Get the field name from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "field name",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	filter, ok := L.ToString(3) // Get the filter from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "filter",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	wrapper.Table.SetFilter(field, filter) // Set the filter for the table
	return 1                               // Return success
}

func setRangeFilter(L *lua.State) int {
	if L.Top() < 2 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "SetRangeFilter",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	wrapper := checkTable(L)
	if wrapper == nil {
		return 0
	}
	field, ok := L.ToString(2) // Get the field name from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "field name",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	min, ok := L.ToInteger(3) // Get the minimum value from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_integer", map[string]interface{}{
			"Name": "minimum value",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	max, ok := L.ToInteger(4) // Get the maximum value from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_integer", map[string]interface{}{
			"Name": "maximum value",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	wrapper.Table.SetRangeFilter(field, min, max) // Set the range filter for the table
	return 1                                      // Return success
}

func setOrderBy(L *lua.State) int {
	if L.Top() < 2 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "SetOrderBy",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	wrapper := checkTable(L)
	if wrapper == nil {
		return 0
	}
	orderBy, ok := L.ToString(2) // Get the order by from Lua
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_string", map[string]interface{}{
			"Name": "order by",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	wrapper.Table.OrderBy(orderBy) // Set the order by for the table
	return 1                       // Return success
}

func insert(L *lua.State) int {
	wrapper := checkTable(L)
	if wrapper == nil {
		return 0
	}

	fields := wrapper.Table.GetCurrentRecord()
	var id int64
	result := wrapper.Table.Insert(fields, &id) // Insert the fields into the table
	L.PushBoolean(result)
	if result {
		L.PushInteger(int(id))
		return 2
	}
	return 1 // Return success
}

// find retrieves all rows from the table and returns them as a Rowset
func find(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.extra_args", map[string]interface{}{
			"Name": "Find",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	wrapper := checkTable(L)
	if wrapper == nil {
		return 0
	}
	L.PushBoolean(wrapper.Table.Find()) // Call the Find method on the table
	return 1                            // Return the number of results
}

func findByID(L *lua.State) int {

	if L.Top() < 2 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.extra_args", map[string]interface{}{
			"Name": "FindByID",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	wrapper := checkTable(L)
	if wrapper == nil {
		return 0
	}
	id, ok := L.ToInteger(2) // Get the ID from Lua
	if !ok {
		if L.IsNil(2) {
			L.PushNil()
			return 1
		}
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_integer", map[string]interface{}{
			"Name": "ID",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	value := wrapper.Table.FindByID(id) // Call the FindByID method on the table
	if !value {
		L.PushNil() // Push nil if no rows found
	} else {
		L.PushBoolean(value) // Push the value as boolean
	}
	return 1 // Return the number of results
}

// next retrieves the next row from the table and returns it as a Rowset
func next(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.extra_args", map[string]interface{}{
			"Name": "Next",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	wrapper := checkTable(L)
	if wrapper == nil {
		return 0
	}
	L.PushBoolean(wrapper.Table.Next()) // Call the Next method on the table
	return 1                            // Return the number of results
}

// prev retrieves the previous row from the table and returns it as a Rowset
func prev(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.extra_args", map[string]interface{}{
			"Name": "Prev",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	wrapper := checkTable(L)
	if wrapper == nil {
		return 0
	}
	L.PushBoolean(wrapper.Table.Prev()) // Call the Prev method on the table
	return 1                            // Return the number of results
}

func update(L *lua.State) int {
	if L.Top() < 1 {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_enough_args_lua", map[string]interface{}{
			"Name": "Update",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	ud := L.ToUserData(1)
	if ud == nil {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_table", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	wrapper, ok := ud.(*gormfunc.TableWrapper)
	if !ok {
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_table", nil), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	table := wrapper.Table // Get the table from the wrapper
	x := table.GetCurrentRecord()[gormfunc.PrimaryKeyField]
	var id int64
	switch v := x.(type) {
	case int:
		id = int64(v)
	case int64:
		id = v
	default:
		errorhandlefunc.ThrowError(i18nfunc.T("error.arg_not_integer", map[string]interface{}{
			"Name": "ID",
		}), errorhandlefunc.ErrorTypeScript, true)
		return 0
	}
	result := table.Update(id, table.GetCurrentRecord()) // Update the table with the current record
	L.SetTop(0)
	L.PushBoolean(result)
	return 1
}

// Register the database functions <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

// LoadLuaModule loads a Lua script as a module that can be required by other scripts
func LoadLuaModule(L *lua.State, moduleName string, scriptPath string) error {
	// Load the script
	if err := lua.DoFile(L, scriptPath); err != nil {
		return err
	}

	// Get the loaded module (should be on top of stack)
	L.SetGlobal(moduleName)
	return nil
}

// RegisterGoFunction registers a Go function to be callable from Lua
func RegisterGoFunction(L *lua.State, name string, fn lua.Function) {
	L.PushGoFunction(fn)
	L.SetGlobal(name)
}

// CallLuaFunction calls a Lua function from Go
func CallLuaFunction(L *lua.State, functionName string, args ...interface{}) error {
	// Get the function from global space
	L.Global(functionName)
	if L.TypeOf(-1) != lua.TypeFunction {
		L.Pop(1)
		return fmt.Errorf("function %s not found", functionName)
	}

	// Push arguments
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			L.PushString(v)
		case int:
			L.PushInteger(v)
		case float64:
			L.PushNumber(v)
		case bool:
			L.PushBoolean(v)
		case nil:
			L.PushNil()
		default:
			return fmt.Errorf("unsupported argument type: %T", arg)
		}
	}

	// Call the function
	return L.ProtectedCall(len(args), 1, 0)
}

// PushRecWithDotNotation pushes a Go map to Lua stack with dot notation support
func PushRecWithDotNotation(L *lua.State, rec gormfunc.Record) {
	// Create the main table
	L.NewTable()

	// Create metatable
	L.NewTable()

	// Set __index metamethod
	L.PushString("__index")
	L.PushGoFunction(func(L *lua.State) int {
		key, ok := L.ToString(2)
		if !ok {
			L.PushNil()
			return 1
		}

		// Get the original table
		L.PushValue(1)
		L.PushString(key)
		L.RawGet(-2)
		return 1
	})
	L.RawSet(-3)

	// Set __newindex metamethod
	L.PushString("__newindex")
	L.PushGoFunction(func(L *lua.State) int {
		key, ok := L.ToString(2)
		if !ok {
			return 0
		}

		// Set value in the original table
		L.PushValue(1)    // push table
		L.PushString(key) // push key
		L.PushValue(3)    // push value
		L.RawSet(-3)      // table[key] = value
		return 0
	})
	L.RawSet(-3)

	// Set the metatable
	L.SetMetaTable(-2)

	// Fill the table with map values
	for k, v := range rec {
		L.PushString(k)
		switch val := v.(type) {
		case string:
			L.PushString(val)
		case int:
			L.PushInteger(val)
		case float64:
			L.PushNumber(val)
		case bool:
			L.PushBoolean(val)
		case nil:
			L.PushNil()
		case gormfunc.Record:
			PushRecWithDotNotation(L, val) // Recursively handle nested maps ????
		default:
			L.PushString(fmt.Sprintf("%v", val))
		}
		L.RawSet(-3)
	}
}
