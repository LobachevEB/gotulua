package errorhandlefunc

import "github.com/Shopify/go-lua"

const (
	ErrorTypeScript = iota
	ErrorTypeData
)

var L *lua.State

func SetLuaState(l *lua.State) {
	L = l
}

func ThrowError(msg string, errorType int, doPanic bool) {
	switch errorType {
	case ErrorTypeScript:
		ShowScriptError(L, msg, doPanic)
	case ErrorTypeData:
		ShowDataError(msg, doPanic)
	}
}
