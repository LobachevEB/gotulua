package errorhandlefunc

import (
	"gotulua/pagesfunc"
	"gotulua/statefunc"
	"regexp"
	"strconv"
	"strings"

	"github.com/Shopify/go-lua"
)

func ShowScriptError(L *lua.State, msg string, doPanic bool) {
	//"runtime error: test1.lua:20: attempt to call local 'b' (a nil value)"
	var oldLine int = -1
	// var err error
	// if strings.Contains(msg, ":::") {
	// 	parts := strings.Split(msg, ":::")
	// 	if len(parts) >= 3 {
	// 		oldLine, err = strconv.Atoi(parts[1])
	// 		if err == nil {
	// 			msg = parts[0] + strings.Join(parts[2:], "")
	// 		}
	// 	}
	// }
	lua.Where(L, 1)
	where, _ := L.ToString(-1)
	L.Pop(1) // remove the where string from the stack
	if where == "" {
		where = msg
	} else {
		msg = where + msg
	}
	script, line := parseScriptLocation(where)
	statefunc.RunFlexLevel0.Clear()
	if oldLine >= 0 {
		line = oldLine
	}
	pagesfunc.SwitchToEditor(script, line, msg, true)
	// defer func() {
	// 	if r := recover(); r != nil {
	// 		f := statefunc.PopVisual()
	// 		if f != nil {
	// 			statefunc.RunFlexLevel0.Clear()
	// 			statefunc.App.SetRoot(f, true)
	// 			statefunc.App.ForceDraw()
	// 		}
	// 	}
	// }()
	f := statefunc.PopVisual()
	if f != nil {
		statefunc.RunFlexLevel0.Clear()
		statefunc.App.SetRoot(f, true)
		statefunc.App.ForceDraw()
	}
	if doPanic {
		// statefunc.InterruptScript(fmt.Sprintf(":::%d:::%s", line, msg))
		statefunc.InterruptScript(msg)
	}
}

func parseScriptLocation(where string) (string, int) {
	parts := strings.Split(where, ":")
	if len(parts) < 2 {
		return "", 0
	}
	lineNumPattern := regexp.MustCompile(`[0-9]+`)
	for i := len(parts) - 1; i > 0; i-- {
		match := lineNumPattern.FindString(parts[i])
		if match != "" {
			line, _ := strconv.Atoi(match)
			line-- // Lua uses 1-based indexing, so we need to decrement by 1
			script := strings.Join(parts[:i], ":")
			return script, line
		}
	}
	return "", 0
}
