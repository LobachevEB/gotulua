package luafunc

import (
	"gotulua/errorhandlefunc"
	"gotulua/statefunc"
	"gotulua/uifunc"

	"github.com/Shopify/go-lua"
)

func RunLuaScript(script string) error {
	defer func() {
		if r := recover(); r != nil {
			f := statefunc.PopVisual()
			if f != nil {
				statefunc.RunFlexLevel0.Clear()
				statefunc.App.SetRoot(f, true)
				statefunc.App.ForceDraw()
			}
		}
	}()
	if script == "" {
		errorhandlefunc.ThrowError("Lua script is empty", errorhandlefunc.ErrorTypeScript, false)
		return nil
	}
	uifunc.ClearWidgets() // Clear the widgets before running the script
	statefunc.ClearErrorRun()
	err := lua.DoFile(statefunc.L, script)
	if err != nil {
		if statefunc.IsErrorRun() {
			statefunc.ClearErrorRun()
			return err
		}
		errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeScript, false)
		return err
	}
	return nil
}
