package statefunc

import (
	"context"
	"sync"

	"github.com/Shopify/go-lua"
	"github.com/rivo/tview"
)

const (
	RunAsScript = iota
	RunAsForm
)

var RunFlexLevel0 *tview.Flex
var RunFlexLevelUserMenu *tview.Flex
var RunFlexLevelDialog *tview.Flex
var RunFlexLevelHelp *tview.Flex
var MainMenuFlex *tview.Flex
var EditorFlex *tview.Flex
var MainFlex *tview.Flex
var Pages *tview.Pages
var App *tview.Application
var L *lua.State
var visualStack *[]*tview.Flex
var InitialTop int
var runMode int = RunAsScript // Default run mode is script
var ShowHelpFunc func(fromEditor bool, callback func(string))
var lastErrorText string
var isErrorRun bool
var RunLuaScriptFunc func(string) error

// ScriptManager handles script execution and interruption
type ScriptManager struct {
	mu            sync.Mutex
	currentState  *lua.State
	currentCancel context.CancelFunc
}

var (
	// Script is the global script manager
	Script = &ScriptManager{}
)

func SetState(runFlex *tview.Flex, mainFlex *tview.Flex, pages *tview.Pages, app *tview.Application) {
	RunFlexLevel0 = runFlex
	RunFlexLevel0.SetDirection(tview.FlexRow)
	MainFlex = mainFlex
	MainFlex.SetTitle("main")
	Pages = pages
	App = app
	RunFlexLevelUserMenu = tview.NewFlex()
	RunFlexLevelDialog = tview.NewFlex()
	RunFlexLevelHelp = tview.NewFlex()
	MainMenuFlex = tview.NewFlex().SetDirection(tview.FlexColumn)
	EditorFlex = tview.NewFlex().SetDirection(tview.FlexColumn)
	visualStack = &[]*tview.Flex{}
}

func SetLuaState(l *lua.State) {
	L = l
}

func PushVisual(flex *tview.Flex) {
	*visualStack = append(*visualStack, flex)
}

func PopVisual() *tview.Flex {
	if len(*visualStack) == 0 {
		return nil
	}
	flex := (*visualStack)[len(*visualStack)-1]
	*visualStack = (*visualStack)[:len(*visualStack)-1]
	App.SetFocus(flex)
	return flex
}

func clearVisualStack() {
	*visualStack = nil
	visualStack = &[]*tview.Flex{}
}

func ShowPreviousVisual() {
	flex := PopVisual()
	if flex != nil {
		App.SetRoot(flex, true)
	}
}

func ShowMainVisual() {
	clearVisualStack()
	if MainFlex != nil {
		App.SetRoot(MainFlex, true)
		App.ForceDraw()
	}
}

func ShowRunVisual() {
	if RunFlexLevel0 != nil {
		App.SetRoot(RunFlexLevel0, true)
		App.ForceDraw()
	}
}

// startScript starts script execution in a separate goroutine that can be cancelled
func (sm *ScriptManager) startScript(L *lua.State, scriptName string, scriptFunc func(string) error) {
	sm.mu.Lock()
	defer func() {
		if r := recover(); r != nil {
			f := PopVisual()
			if f != nil {
				RunFlexLevel0.Clear()
				App.SetRoot(f, true)
				App.ForceDraw()
			}
		}
	}()
	defer sm.mu.Unlock()

	// Cancel any existing script
	if sm.currentCancel != nil {
		sm.currentCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	sm.currentState = L
	sm.currentCancel = cancel

	go func() {
		defer func() {
			if r := recover(); r != nil {
				f := PopVisual()
				if f != nil {
					RunFlexLevel0.Clear()
					App.SetRoot(f, true)
					App.ForceDraw()
				}
			}
		}()
		done := make(chan error, 1)
		setInitialTop(L.Top()) // Set the initial top for the Lua state
		// Run the script in a goroutine
		go func() {
			defer func() {
				if r := recover(); r != nil {
					f := PopVisual()
					if f != nil {
						RunFlexLevel0.Clear()
						App.SetRoot(f, true)
						App.ForceDraw()
					}
				}
			}()
			done <- scriptFunc(scriptName)
		}()

		// Wait for either completion or cancellation
		select {
		case err := <-done:
			if err != nil {
				return
			}
		case <-ctx.Done():
			return
		}

		sm.mu.Lock()
		sm.currentState = nil
		sm.currentCancel = nil
		sm.mu.Unlock()
	}()
}

// interruptScript interrupts the currently running script
func (sm *ScriptManager) interruptScript(msg string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.currentCancel != nil {
		sm.currentCancel()
		sm.currentState = nil
		sm.currentCancel = nil
	}
	L.PushString(msg)
	L.Error()
}

// getCurrentLuaState returns the currently running Lua state, if any
func (sm *ScriptManager) getCurrentLuaState() *lua.State {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.currentState
}

// Convenience functions for the global script manager
func StartScript(L *lua.State, scriptName string, scriptFunc func(string) error) {
	Script.startScript(L, scriptName, scriptFunc)
}

func InterruptScript(msg string) {
	Script.interruptScript(msg)
}

func GetCurrentLuaState() *lua.State {
	return Script.getCurrentLuaState()
}

func setInitialTop(top int) {
	InitialTop = top
}

func IsNowOnInitialTop() bool {
	return InitialTop == L.Top()
}

func SetRunMode(mode int) {
	runMode = mode
}
func GetRunMode() int {
	return runMode
}
func IsRunAsScript() bool {
	return runMode == RunAsScript
}

func IsRunAsForm() bool {
	return runMode == RunAsForm
}

func CatchErrorShowEditor(msg string) {
	//RunFlexLevel0.Clear()
	//clearVisualStack()
	SetLastErrorText(msg)
	// App.SetRoot(MainFlex, true)
	// App.SetFocus(EditorFlex)
	// App.ForceDraw()
}

func SetLastErrorText(msg string) {
	lastErrorText = msg
}
func GetLastErrorText() string {
	return lastErrorText
}
func ClearErrors() {
	lastErrorText = ""
}
func SetErrorRun() {
	isErrorRun = true
}
func ClearErrorRun() {
	isErrorRun = false
}
func IsErrorRun() bool {
	return isErrorRun
}
