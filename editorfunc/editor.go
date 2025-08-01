package editorfunc

import (
	"fmt"
	"gotulua/statefunc"
	"os"
	"regexp"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	IsNoHighlight = iota
	IsErrorHighlight
	IsWarningHighlight
)

const (
	editorTitle string = " (Ctrl+S to Save, Ctrl+Q to Quit, Ctrl+Z to Undo, Ctrl+Y to Redo, Insert to Copy, Ctrl+F to Find, F10 to Menu, F1 to Help, F5 to Run) "
)

// EditAction represents a single edit operation that can be undone/redone
type EditAction struct {
	beforeContent []string
	afterContent  []string
	beforeCursorX int
	beforeCursorY int
	afterCursorX  int
	afterCursorY  int
}

// Selection represents a text selection range
type Selection struct {
	startX, startY int
	endX, endY     int
	active         bool
}

// LuaEditor is a tview-based text editor for Lua scripts with syntax highlighting.
type LuaEditor struct {
	*tview.TextView
	height           int // number of visible lines in the editor area (calculated dynamically)
	content          []string
	cursorX          int // rune index within the current line
	cursorY          int
	onSave           func(content string)
	fileName         string
	statusBar        *StatusBar
	app              *tview.Application
	showSaveAsDialog func(*tview.Application, string, func(string) error, func())
	undoStack        []EditAction
	redoStack        []EditAction
	selection        Selection
	mouseDown        bool
	highlightedLine  int // line number of the currently highlighted line
	highlightType    int // type of highlight (error, warning, etc.)
	findText         string
	currentFindY     int
	currentFindX     int
}

// Lua syntax highlighting rules
var (
	luaKeywords = []string{
		"and", "break", "do", "else", "elseif", "end", "false", "for", "function",
		"goto", "if", "in", "local", "nil", "not", "or", "repeat", "return",
		"then", "true", "until", "while",
	}
	luaSqBracketPattern = regexp.MustCompile(`\[.*\]`)
	luaKeywordPattern   = regexp.MustCompile(`\b(` + strings.Join(luaKeywords, "|") + `)\b`)
	luaStringPattern    = regexp.MustCompile(`("([^"\\]|\\.)*"|'([^'\\]|\\.)*')`)
	luaCommentPattern   = regexp.MustCompile(`^--.*`)
	luaNumberPattern    = regexp.MustCompile(`\b\d+(\.\d+)?\b`)
	luaFunctionPattern  = regexp.MustCompile(`\bfunction\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
	tviewColorPattern   = regexp.MustCompile(`\x01\#{0,1}[A-F0-9\:\-]+\x02`)
)

// SyntaxHighlightLua applies Lua syntax highlighting to the given text.
func SyntaxHighlightLua(text string) string {
	// To avoid color tags being shown when the cursor is on a keyword (i.e., when tview regions are used),
	// we need to escape color tags inside regions. We'll do this by splitting the text into lines,
	// and only apply color tags to lines that are not currently selected (i.e., not under the cursor).
	// However, since this function doesn't know about the cursor, the best fix is to escape color tags
	// when inside a region, i.e., when tview will interpret the line as a region.
	// The most robust way is to escape color tags by doubling the opening bracket when inside a region.
	// But since we don't have region info here, a practical fix is to escape color tags globally
	// if the text is being used as a region, or to provide a function to do so.
	// For now, let's provide a helper to escape color tags, and assume the caller will use it
	// when rendering the cursor line.

	// The default implementation (for non-cursor lines):
	highlight := func(text string) string {
		text = luaSqBracketPattern.ReplaceAllStringFunc(text, func(s string) string {
			s = strings.ReplaceAll(s, "[", "\x01")
			s = strings.ReplaceAll(s, "]", "\x02")
			return s
		})
		// Highlight comments
		text = luaCommentPattern.ReplaceAllStringFunc(text, func(s string) string {
			if strings.HasSuffix(s, "\r") {
				return `[gray]` + s[:len(s)-1] + `[-]` + "\r"
			} else {
				return `[gray]` + s + `[-]`
			}
		})
		// Highlight strings
		text = luaStringPattern.ReplaceAllStringFunc(text, func(s string) string {
			if strings.HasSuffix(s, "\r") {
				return `[yellow]` + s[:len(s)-1] + `[-]` + "\r"
			} else {
				return `[yellow]` + s + `[-]`
			}
		})
		// Highlight numbers
		text = luaNumberPattern.ReplaceAllStringFunc(text, func(s string) string {
			if strings.HasSuffix(s, "\r") {
				return `[magenta]` + s[:len(s)-1] + `[-]` + "\r"
			} else {
				return `[magenta]` + s + `[-]`
			}
		})
		// Highlight keywords
		text = luaKeywordPattern.ReplaceAllStringFunc(text, func(s string) string {
			// if isCursorLine {
			// 	return escapeColorTags(`[blue::b]` + s + `[-::-]`)
			// }
			if strings.HasSuffix(s, "\r") {
				return `[#00BFFF::b]` + s[:len(s)-1] + `[-::-]` + "\r"
			} else {
				return `[#00BFFF::b]` + s + `[-::-]`
			}
		})
		// Highlight function names
		text = luaFunctionPattern.ReplaceAllStringFunc(text, func(s string) string {
			parts := luaFunctionPattern.FindStringSubmatch(s)
			if len(parts) > 1 {
				return "function [green::b]" + parts[1] + "[-::-]"
			}
			return s
		})
		return text
	}

	return highlight(text)
}

// highlightLineByStatus returns the text with the specified line (0-based) highlighted using the given color tag.
// colorTag should be a tview color tag, e.g. "[red]" or "[#00FF00]".
func highlightLineByStatus(highlightType int, text string) string {
	var colorTag string
	switch highlightType {
	case IsNoHighlight:
		return text
	case IsErrorHighlight:
		colorTag = "[:red:]"
	case IsWarningHighlight:
		colorTag = "[:yellow:]"
	}
	lines := strings.Split(text, "\r")
	line := colorTag + lines[0] + "[:-:]" //+ "\r"
	return line
}

// StatusBar is a simple status bar for the LuaEditor.
type StatusBar struct {
	*tview.TextView
}

// The status bar is a separate widget (StatusBar) and must be added to your tview layout (e.g., Flex, Grid, etc.)
// along with the LuaEditor. If you only add the LuaEditor to your layout and not the status bar, it won't be visible.
// Make sure you do something like:
//
//   flex := tview.NewFlex().SetDirection(tview.FlexRow).
//       AddItem(luaEditor, 0, 1, true).
//       AddItem(luaEditor.GetStatusBar(), 1, 0, false)
//
// This way, the status bar will be shown below the editor. Also, ensure that you call FillStatusBar()
// or SetStatus() to update its contents as needed.
//
// If you still don't see it, check that the status bar is not being hidden by other widgets or that its height is not set to 0.

// NewStatusBar creates a new StatusBar.
func NewStatusBar() *StatusBar {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(false).
		SetWrap(false)
	tv.SetBackgroundColor(tcell.ColorDefault)
	tv.SetTextColor(tcell.ColorWhite)
	return &StatusBar{TextView: tv}
}

// SetStatus sets the status message in the status bar.
func (sb *StatusBar) SetStatus(msg string) {
	sb.Clear()
	sb.Write([]byte(msg)) //SetText(msg)
}

func (sb *StatusBar) SetErrorStatus(msg string) {
	sb.Clear()
	msg = "[red::]" + msg + "[-::-]"
	sb.Write([]byte(msg)) //SetText(msg)
}

func (e *LuaEditor) FindText(text string, again bool) {
	if !again {
		e.currentFindY = 0
		e.currentFindX = 0
		e.findText = text
	}
	if e.findText == "" {
		return
	}
	for e.currentFindY < len(e.content) {
		cl := e.content[e.currentFindY]
		if e.currentFindX > 0 && e.currentFindX < len(cl) {
			cl = cl[e.currentFindX:]
		}
		if strings.Contains(cl, e.findText) {
			index := strings.Index(cl, e.findText)
			e.cursorY = e.currentFindY
			e.cursorX = index + e.currentFindX
			e.currentFindX += index + len(e.findText)
			_, _, _, height := e.GetInnerRect()
			row, _ := e.GetScrollOffset()
			if e.cursorY >= row+height {
				e.ScrollTo(e.cursorY-height+1, 0)
			}

			e.redraw()
			return
		}
		e.currentFindY++
		e.currentFindX = 0
		if e.currentFindY >= len(e.content) {
			e.currentFindY = 0
			e.currentFindX = 0
			e.SetStatus("Find: " + e.findText + " not found")
			e.redraw()
			return
		}
	}
	e.SetStatus("Find: " + e.findText + " not found")
	e.redraw()
}

// SetStatus sets the status message in the editor's status bar.
func (e *LuaEditor) SetStatus(msg string) {
	if e.statusBar != nil {
		e.statusBar.SetStatus(msg)
	}
}

// SetErrorStatus sets an error message in the editor's status bar, formatting it as an error.
func (e *LuaEditor) SetErrorStatus(msg string) {
	if e.statusBar != nil {
		e.statusBar.SetErrorStatus(msg)
	}
}

func (e *LuaEditor) SetHighlightedLine(line int, highlightType int) {
	if line < 0 || line >= len(e.content) {
		e.highlightedLine = -1
		e.highlightType = IsNoHighlight
	} else {
		e.highlightedLine = line
		e.highlightType = highlightType
	}
	e.redraw()
}

// GetStatusBar returns the status bar widget.
func (e *LuaEditor) GetStatusBar() *StatusBar {
	return e.statusBar
}

// FillStatusBar updates the status bar with current editor state (e.g., line/column).
func (e *LuaEditor) FillStatusBar() {
	line := e.cursorY + 1
	col := e.cursorX + 1
	totalLines := len(e.content)
	msg := "[::b]Ln:[-] %d [::b]Col:[-] %d   [::b]Lines:[-] %d"
	e.SetStatus(
		fmt.Sprintf(msg, line, col, totalLines),
	)
}

// OpenFile loads content from the specified file into the editor
func (e *LuaEditor) OpenFile(fileName string) error {
	data, err := os.ReadFile(fileName)
	if err != nil {
		e.SetErrorStatus(fmt.Sprintf("Error opening file: %v", err))
		return err
	}

	// Update editor state
	e.fileName = fileName
	e.content = strings.Split(string(data), "\n")
	e.cursorX = 0
	e.cursorY = 0

	// Update editor title
	e.SetBorder(true).SetTitle(fileName + editorTitle)

	// Reset scroll position
	e.ScrollTo(0, 0)

	e.SetStatus(fmt.Sprintf("Opened file: %s", fileName))
	e.redraw()
	return nil
}

// readInitialContentFromFile reads the content of the given fileName.
// If the file cannot be read, it returns the provided initialContent.
func readInitialContentFromFile(fileName, initialContent string) string {
	if fileName == "" {
		return initialContent
	}
	data, err := os.ReadFile(fileName)
	if err != nil {
		return initialContent
	}
	return string(data)
}

// NewLuaEditor creates a new LuaEditor.
func NewLuaEditor(app *tview.Application, initialContent string, fileName string, onSave func(content string)) *LuaEditor {
	// Initialize with empty content if both fileName and initialContent are empty
	if initialContent == "" && fileName == "" {
		initialContent = ""
	} else {
		// Try to read from file first
		initialContent = readInitialContentFromFile(fileName, initialContent)
	}

	lines := strings.Split(initialContent, "\n")
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetChangedFunc(func() {
			// Redraw on content change
		})
	editor := &LuaEditor{
		TextView:         tv,
		content:          lines,
		cursorX:          0,
		cursorY:          0,
		onSave:           onSave,
		fileName:         fileName,
		statusBar:        NewStatusBar(),
		app:              app,
		showSaveAsDialog: nil,
		undoStack:        make([]EditAction, 0),
		redoStack:        make([]EditAction, 0),
		highlightedLine:  -1,
		highlightType:    IsNoHighlight,
	}

	title := ""
	if fileName != "" {
		title += fileName + " "
	}
	title += editorTitle
	editor.SetBorder(true).SetTitle(title)
	editor.SetInputCapture(editor.handleInput)
	editor.SetStatus("Ready")
	editor.redraw()
	return editor
}

// SaveFile saves the current content to the file
func (e *LuaEditor) SaveFile() error {
	// Normalize line endings on Windows
	content := e.content
	var suffix string
	if runtime.GOOS == "windows" {
		suffix = "\r\n"
	} else {
		suffix = "\n"
	}
	normalizedContent := make([]string, len(content))
	for i, line := range content {
		// Remove any existing line endings
		line = strings.TrimRight(line, "\r\n")
		// Add Windows line ending
		normalizedContent[i] = line + suffix
	}
	content = normalizedContent

	// Join lines and write to file
	fileContent := strings.Join(content, "")
	err := os.WriteFile(e.fileName, []byte(fileContent), 0644)
	if err != nil {
		e.SetErrorStatus(fmt.Sprintf("Error saving file: %v", err))
		return err
	}
	e.SetStatus("File saved successfully")
	return nil
}

// SetSaveAsDialogHandler sets the function that will show the Save As dialog
func (e *LuaEditor) SetSaveAsDialogHandler(handler func(*tview.Application, string, func(string) error, func())) {
	e.showSaveAsDialog = handler
}

// ShowSaveAsDialog shows the Save As dialog for the editor
func (e *LuaEditor) ShowSaveAsDialog() {
	if e.showSaveAsDialog != nil {
		e.showSaveAsDialog(e.app, e.fileName, func(fileName string) error {
			return e.SaveFile()
		}, func() {
			e.SetStatus("Save As dialog cancelled")
		})
	} else {
		e.SetErrorStatus("Save As dialog not available")
	}
}

// calculateHeight updates the height field to the number of visible lines in the editor area.
func (e *LuaEditor) calculateHeight() {
	_, _, _, height := e.GetInnerRect()
	e.height = height
}

// redraw updates the TextView with syntax highlighted content and cursor.
func (e *LuaEditor) redraw() {
	var origLine string
	var line string
	var y int
	e.Clear()
	e.calculateHeight()
	for y, line = range e.content {
		var hl string
		line = strings.Trim(line, "\r")
		origLine = ""
		if strings.Contains(line, "[") || strings.Contains(line, "]") {
			origLine = line
			line = strings.ReplaceAll(line, "[", "\x01")
			line = strings.ReplaceAll(line, "]", "\x02")
		}
		if y == e.highlightedLine && e.highlightType != IsNoHighlight {
			hl = highlightLineByStatus(e.highlightType, line)
		} else {
			hl = SyntaxHighlightLua(line)
		}

		// Handle selection highlighting
		if e.selection.active {
			var newHl strings.Builder
			var currentPos int
			var inTag bool
			var currentTag strings.Builder
			runes := []rune(line)

			for pos := 0; pos < len(hl); {
				if pos < len(hl)-1 && hl[pos] == '[' && !inTag {
					// Start of a color tag
					inTag = true
					currentTag.Reset()
					currentTag.WriteByte('[')
					pos++
					continue
				}

				if inTag {
					currentTag.WriteByte(hl[pos])
					if hl[pos] == ']' {
						// End of color tag
						inTag = false
						newHl.WriteString(currentTag.String())
					}
					pos++
					continue
				}

				// Regular character
				runeIndex := 0
				for i := 0; i < len(runes); i++ {
					if runeIndex == currentPos {
						if e.selection.isSelected(i, y) {
							newHl.WriteString("[black:white]")
							//newHl.WriteByte(hl[pos])
							newHl.WriteRune(runes[i])
							newHl.WriteString("[-:-]")
						} else {
							//newHl.WriteByte(hl[pos])
							newHl.WriteRune(runes[i])
						}
						break
					}
					runeIndex++
				}
				currentPos++
				pos++
			}

			hl = newHl.String()
		}

		if y == e.cursorY {
			var cursorInHl int
			runes := []rune(line)
			lineLen := len(runes)
			if e.cursorX > lineLen {
				e.cursorX = lineLen
			}
			if e.cursorX < 0 {
				e.cursorX = 0
			}
			if lineLen == 0 || line == "\n" || line == "\r" || line == "\r\n" {
				// Highlight empty line at cursor
				//if e.cursorX < 1 {
				hl = "[white:blue] " + " " + "[-:-:-]" + line
				// } else {
				// 	hl = "[white:blue] " + line + "[-:-:-]"
				// }
			} else if e.cursorX < lineLen {
				// Map rune index to byte index in line
				byteIdx := 0
				for i := 0; i < e.cursorX && byteIdx < len(line); i++ {
					_, size := utf8.DecodeRuneInString(line[byteIdx:])
					byteIdx += size
				}
				// Now, map byteIdx in line to position in hl
				hlIdx, lineIdx := 0, 0
				for lineIdx < len(runes) && hlIdx < len(hl) {
					if hl[hlIdx] == '[' {
						end := strings.IndexByte(hl[hlIdx:], ']')
						if end == -1 {
							break
						}
						hlIdx += end + 1
					} else {
						if lineIdx == e.cursorX {
							cursorInHl = hlIdx
							break
						}
						_, size := utf8.DecodeRuneInString(hl[hlIdx:])
						hlIdx += size
						lineIdx++
					}
				}
				if lineIdx == e.cursorX {
					cursorInHl = hlIdx
				}

				// Check if we are inside a color tag
				openTagStart := strings.LastIndex(hl[:cursorInHl], "[")
				openTagEnd := strings.LastIndex(hl[:cursorInHl], "]")
				insideTag := openTagStart > openTagEnd

				if insideTag {
					// Insert cursor highlight after the open tag
					tagEnd := strings.IndexByte(hl[openTagStart:], ']')
					if tagEnd != -1 {
						tagEnd += openTagStart
						if string(runes[e.cursorX]) == "\r" {
							hl = hl[:cursorInHl] + "[white:blue]" + " " + "[-:-]" + hl[cursorInHl+utf8.RuneLen(runes[e.cursorX]):] + "\r"
						} else {
							hl = hl[:cursorInHl] + "[white:blue]" + string(runes[e.cursorX]) + "[-:-]" + hl[cursorInHl+utf8.RuneLen(runes[e.cursorX]):]
						}
					}
				} else {
					if string(runes[e.cursorX]) == "\r" {
						// if !strings.HasSuffix(hl, "\r") {
						// 	hl = hl + "\r"
						// }
						hl = hl[:cursorInHl] + "[white:blue]" + " " + "[-:-]" + hl[cursorInHl+utf8.RuneLen(runes[e.cursorX]):] + "\r"
					} else {
						// if !strings.HasSuffix(hl, "\r") {
						// 	hl = hl + "\r"
						// }
						hl = hl[:cursorInHl] + "[white:blue]" + string(runes[e.cursorX]) + "[-:-]" + hl[cursorInHl+utf8.RuneLen(runes[e.cursorX]):]
					}
				}
			} else if e.cursorX == lineLen {
				// Cursor at end of line
				if strings.HasSuffix(hl, "\r") {
					hl = hl[:len(hl)-1] + "[white:blue] [-:-]\r"
				} else {
					hl = hl + "[white:blue] [-:-]"
				}
			}
		}
		if y == e.cursorY {
			cpos := strings.Index(hl, "[white:blue]")
			if cpos != -1 {
				cend := strings.Index(hl[cpos:], "[-:-]") + cpos + len("[-:-]")
				prevTagBeginBegin := strings.LastIndex(hl[:cpos], "[")
				if prevTagBeginBegin != -1 {
					//beforePrevTag := hl[:prevTagBeginBegin]
					prevTagBeginEnd := strings.Index(hl[prevTagBeginBegin:], "]") + prevTagBeginBegin
					prevTagEndBegin := strings.Index(hl[cend:], "[") + cend
					if prevTagBeginBegin < cpos && prevTagEndBegin > cend {
						prevTagEndEnd := strings.Index(hl[prevTagEndBegin:], "]") + prevTagEndBegin
						endpart := make([]byte, len(hl[cend:]))
						copy(endpart, hl[cend:])
						endtag := make([]byte, prevTagEndEnd-prevTagEndBegin+1)
						copy(endtag, hl[prevTagEndBegin:prevTagEndEnd+1])
						beginpart := make([]byte, cpos) //prevTagBeginBegin
						copy(beginpart, hl[:cpos])
						begintag := make([]byte, prevTagBeginEnd-prevTagBeginBegin+1)
						copy(begintag, hl[prevTagBeginBegin:prevTagBeginEnd+1])
						cursorpart := make([]byte, cend-cpos)
						copy(cursorpart, hl[cpos:cend])
						hl = string(beginpart) + string(endtag) + string(cursorpart) + string(begintag) + string(endpart)
					}
				}
			}
		}
		if origLine != "" {
			if y == e.cursorY {
				hl = strings.ReplaceAll(hl, "\x02", "]")
			} else {
				if !luaKeywordPattern.MatchString(hl) && tviewColorPattern.MatchString(hl) {
					hl = strings.ReplaceAll(hl, "\x02", "[]")
				} else {
					hl = strings.ReplaceAll(hl, "\x02", "]")
				}
			}
			hl = strings.ReplaceAll(hl, "\x01", "[")
		}
		hl = hl + "\r"
		e.Write([]byte(hl))
	}
}

// Enable mouse support for navigation
func (e *LuaEditor) handleMouse(action tview.MouseAction, event *tcell.EventMouse) (consumed bool) {
	e.highlightType = IsNoHighlight // Reset highlight type on mouse action
	x, y := event.Position()
	left, top, _, _ := e.GetInnerRect()
	innerX, innerY := x-left, y-top

	// Get current scroll offset and adjust innerY
	row, _ := e.GetScrollOffset()
	adjustedY := row + innerY

	if adjustedY < 0 || adjustedY >= len(e.content) {
		return false
	}

	line := e.content[adjustedY]
	runes := []rune(line)
	cursorX := 0

	visualX := 0
	inColorTag := false
	for i := 0; i < len(line); i++ {
		if line[i] == '[' && !inColorTag {
			inColorTag = true
			continue
		}
		if inColorTag {
			if line[i] == ']' {
				inColorTag = false
			}
			continue
		}
		if visualX >= innerX {
			break
		}
		cursorX++
		visualX++
	}
	if cursorX > len(runes) {
		cursorX = len(runes)
	}

	switch action {
	case tview.MouseLeftDown:
		e.mouseDown = true
		e.selection.startX = cursorX
		e.selection.startY = adjustedY
		e.selection.endX = cursorX
		e.selection.endY = adjustedY
		e.selection.active = true
		e.cursorX = cursorX
		e.cursorY = adjustedY
		e.currentFindY = adjustedY
		e.currentFindX = 0
	case tview.MouseMove:
		if e.mouseDown {
			e.selection.endX = cursorX
			e.selection.endY = adjustedY
			e.cursorX = cursorX
			e.cursorY = adjustedY
			e.currentFindY = adjustedY
			e.currentFindX = 0
		}
	case tview.MouseLeftUp:
		e.mouseDown = false
		if e.selection.startX == cursorX && e.selection.startY == adjustedY {
			e.selection.active = false
		} else {
			e.selection.endX = cursorX
			e.selection.endY = adjustedY
		}
		e.cursorX = cursorX
		e.cursorY = adjustedY
		e.currentFindY = adjustedY
		e.currentFindX = 0
	}
	e.FillStatusBar()
	e.redraw()
	return true
}

// Attach mouse handler to the LuaEditor
func (e *LuaEditor) SetMouseSupport() {
	e.TextView.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		//consumed :=
		e.handleMouse(action, event)
		return action, event
	})
}

func getRunes(line string) []rune {
	return []rune(line)
}

// handleInput processes key events for editing.
func (e *LuaEditor) handleInput(event *tcell.EventKey) *tcell.EventKey {
	// Helper to get rune slice of current line
	e.highlightType = IsNoHighlight
	setLine := func(y int, runes []rune) {
		e.content[y] = string(runes)
	}
	e.FillStatusBar()

	// Check for Ctrl+Shift+S (Save As)
	if event.Key() == tcell.KeyCtrlS && event.Modifiers()&tcell.ModShift != 0 {
		e.ShowSaveAsDialog()
		return nil
	}

	// Handle undo/redo
	if event.Key() == tcell.KeyCtrlZ {
		e.undo()
		return nil
	}
	if event.Key() == tcell.KeyCtrlY {
		e.redo()
		return nil
	}

	// Check for Shift+Insert (Paste)
	if event.Key() == tcell.KeyInsert && event.Modifiers()&tcell.ModShift != 0 {
		e.pasteFromClipboard()
		return nil
	}

	// Handle copy (using Insert key)
	if event.Key() == tcell.KeyInsert && event.Modifiers() == tcell.ModNone {
		e.copySelection()
		return nil
	}

	// Handle selection with shift + arrow keys
	if event.Modifiers()&tcell.ModShift != 0 {
		switch event.Key() {
		case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown, tcell.KeyEnd, tcell.KeyHome:
			if !e.selection.active {
				e.selection.active = true
				e.selection.startX = e.cursorX
				e.selection.startY = e.cursorY
			}
		}
	} else {
		// Clear selection when moving cursor without shift
		switch event.Key() {
		case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
			e.selection.active = false
		}
	}

	// Save current state before modification
	beforeContent := make([]string, len(e.content))
	copy(beforeContent, e.content)
	beforeX, beforeY := e.cursorX, e.cursorY

	switch event.Key() {
	case tcell.KeyF3, tcell.KeyF4:
		e.FindText("", true)
		return nil
	case tcell.KeyCtrlS:
		if e.fileName != "" {
			err := e.SaveFile()
			if err != nil {
				// Error message already set in SaveFile
				return nil
			}
		} else {
			// No filename set, show Save As dialog
			e.ShowSaveAsDialog()
		}
		if e.onSave != nil {
			e.onSave(strings.Join(e.content, "\n"))
		}
		return nil
	case tcell.KeyCtrlQ:
		// Exit editor (handled by parent)
		return event
	case tcell.KeyF1, tcell.KeyF2:
		if statefunc.ShowHelpFunc != nil {
			statefunc.PushVisual(statefunc.MainFlex)
			statefunc.ShowHelpFunc(true, func(functionName string) {
				lineRunes := getRunes(e.content[e.cursorY])
				if e.cursorX > len(lineRunes) {
					e.cursorX = len(lineRunes)
				}
				emptyLine := len(lineRunes) == 0
				last13 := false
				if !emptyLine {
					if lineRunes[len(lineRunes)-1] == '\r' {
						// If the last character is a carriage return, remove it
						lineRunes = lineRunes[:len(lineRunes)-1]
						last13 = true
					}
				}
				if e.cursorX > len(lineRunes) {
					e.cursorX = len(lineRunes)
				}
				lineRunes = append(lineRunes[:e.cursorX], append([]rune(functionName), lineRunes[e.cursorX:]...)...)
				if emptyLine || last13 {
					lineRunes = append(lineRunes, '\r')
				}
				setLine(e.cursorY, lineRunes)
				e.cursorX += len(functionName)
				e.redraw()
			})
		}
	case tcell.KeyF5:
		statefunc.PushVisual(statefunc.MainFlex)
		statefunc.App.SetRoot(statefunc.RunFlexLevel0, true)
		statefunc.StartScript(statefunc.L, e.GetFileName(), statefunc.RunLuaScriptFunc)

	case tcell.KeyUp:
		if e.cursorY > 0 {
			e.cursorY--
			e.currentFindY = e.cursorY
			e.currentFindX = 0
			lineRunes := getRunes(e.content[e.cursorY])
			if e.cursorX > len(lineRunes) {
				e.cursorX = len(lineRunes)
			}
			// Scroll up if cursor is above visible window
			row, _ := e.GetScrollOffset()
			if e.cursorY < row {
				e.ScrollTo(row-1, 0)
			} else if e.cursorY < 0 {
				e.ScrollTo(0, 0)
			}
		}
	case tcell.KeyDown:
		if e.cursorY < len(e.content)-1 {
			e.cursorY++
			e.currentFindY = e.cursorY
			e.currentFindX = 0
			lineRunes := getRunes(e.content[e.cursorY])
			if e.cursorX > len(lineRunes) {
				e.cursorX = len(lineRunes)
			}
			// Scroll down if cursor is below visible window
			_, _, _, height := e.GetInnerRect()
			row, _ := e.GetScrollOffset()
			if e.cursorY >= row+height {
				e.ScrollTo(e.cursorY-height+1, 0)
			}
		}
	case tcell.KeyLeft:
		if e.cursorX > 0 {
			e.cursorX--
		} else if e.cursorY > 0 {
			e.cursorY--
			e.cursorX = len(getRunes(e.content[e.cursorY]))
			// Scroll up if cursor is above visible window
			row, _ := e.GetScrollOffset()
			if e.cursorY < row {
				e.ScrollTo(0, e.cursorY)
			} else if e.cursorY < 0 {
				e.ScrollTo(0, 0)
			}
		}
		e.currentFindY = e.cursorY
		e.currentFindX = 0
	case tcell.KeyRight:
		if e.cursorY <= len(e.content)-1 {
			lineRunes := getRunes(e.content[e.cursorY])
			if e.cursorX < len(lineRunes) {
				if string(lineRunes[e.cursorX]) == "\r" {
					e.cursorX = 0
					e.cursorY++
					// Scroll down if cursor is below visible window
					_, _, _, height := e.GetInnerRect()
					row, _ := e.GetScrollOffset()
					if e.cursorY >= row+height {
						e.ScrollTo(e.cursorY-height+1, 0)
					}
				} else {
					e.cursorX++
				}
			}
		} else if e.cursorY < len(e.content)-1 {
			e.cursorY++
			e.cursorX = 0
			// Scroll down if cursor is below visible window
			_, _, _, height := e.GetInnerRect()
			row, _ := e.GetScrollOffset()
			if e.cursorY >= row+height {
				e.ScrollTo(0, e.cursorY-height+1)
			}
		}
		e.currentFindY = e.cursorY
		e.currentFindX = 0
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		lineRunes := getRunes(e.content[e.cursorY])
		if e.cursorX > 0 {
			// Remove rune before cursor
			lineRunes = append(lineRunes[:e.cursorX-1], lineRunes[e.cursorX:]...)
			setLine(e.cursorY, lineRunes)
			e.cursorX--
		} else if e.cursorY > 0 {
			// Join with previous line
			if len(lineRunes) == 1 && lineRunes[0] == '\r' {
				e.content = append(e.content[:e.cursorY], e.content[e.cursorY+1:]...)
				e.cursorY--
				e.cursorX = len(getRunes(e.content[e.cursorY]))
			} else {
				prevRunes := getRunes(e.content[e.cursorY-1])
				if strings.HasSuffix(string(prevRunes), "\r") {
					prevRunes = prevRunes[:len(prevRunes)-1]
				}
				newRunes := append(prevRunes, lineRunes...)
				setLine(e.cursorY-1, newRunes)
				e.content = append(e.content[:e.cursorY], e.content[e.cursorY+1:]...)
				e.cursorY--
				e.cursorX = len(prevRunes)
			}
		}
		e.currentFindY = e.cursorY
		e.currentFindX = 0
	case tcell.KeyDelete:
		if e.selection.active {
			// If selection is active, delete the selected text
			e.deleteSelection()
		} else {
			lineRunes := getRunes(e.content[e.cursorY])
			if e.cursorX < len(lineRunes) {
				// Remove rune at cursor
				if string(lineRunes[e.cursorX]) == "\r" {
					// If we're deleting a carriage return, join with next line
					if e.cursorY < len(e.content)-1 {
						nextLineRunes := getRunes(e.content[e.cursorY+1])
						lineRunes = append(lineRunes[:e.cursorX], nextLineRunes...)
						setLine(e.cursorY, lineRunes)
						e.content = append(e.content[:e.cursorY+1], e.content[e.cursorY+2:]...)
					}
				} else {
					// Normal character deletion
					lineRunes = append(lineRunes[:e.cursorX], lineRunes[e.cursorX+1:]...)
					setLine(e.cursorY, lineRunes)
				}
			} else if e.cursorY < len(e.content)-1 {
				// At end of line, join with next line
				nextLineRunes := getRunes(e.content[e.cursorY+1])
				if strings.HasSuffix(string(lineRunes), "\r") {
					lineRunes = lineRunes[:len(lineRunes)-1]
				}
				lineRunes = append(lineRunes, nextLineRunes...)
				setLine(e.cursorY, lineRunes)
				e.content = append(e.content[:e.cursorY+1], e.content[e.cursorY+2:]...)
			}
		}
		e.currentFindY = e.cursorY
		e.currentFindX = 0
	case tcell.KeyEnter:
		lineRunes := getRunes(e.content[e.cursorY])
		// Split at cursor
		before := lineRunes[:e.cursorX]
		after := lineRunes[e.cursorX:]
		e.content[e.cursorY] = string(before)
		newLine := string(after)
		// Rune-aware split and insert for Enter key
		if e.cursorY == len(e.content)-1 {
			// At last line, append new line
			if !strings.HasSuffix(e.content[e.cursorY], "\r") {
				e.content[e.cursorY] += "\r"
			}
			e.content = append(e.content, newLine)
		} else {
			// Insert new line after current line
			tmp := make([]string, len(e.content)+1)
			copy(tmp, e.content[:e.cursorY+1])
			if !strings.HasSuffix(tmp[e.cursorY], "\r") {
				tmp[e.cursorY] += "\r"
			}
			if !strings.HasSuffix(newLine, "\r") {
				newLine += "\r"
			}
			tmp[e.cursorY+1] = newLine
			copy(tmp[e.cursorY+2:], e.content[e.cursorY+1:])
			e.content = tmp
		}
		e.cursorY++
		e.cursorX = 0
		e.currentFindY = e.cursorY
		e.currentFindX = 0
	case tcell.KeyHome:
		// Move cursor to the beginning of the line
		e.cursorX = 0
		e.currentFindY = e.cursorY
		e.currentFindX = 0
	case tcell.KeyEnd:
		// Move cursor to the end of the line (rune-aware)
		lineRunes := getRunes(e.content[e.cursorY])
		e.cursorX = len(lineRunes)
		e.currentFindY = e.cursorY
		e.currentFindX = 0
	case tcell.KeyPgUp:
		// Move cursor up by visible height or to top, and scroll screen if needed
		pageSize := e.height - 1
		if e.cursorY > pageSize {
			e.cursorY -= pageSize
		} else {
			e.cursorY = 0
		}
		// Clamp cursorX to line length
		lineRunes := getRunes(e.content[e.cursorY])
		if e.cursorX > len(lineRunes) {
			e.cursorX = len(lineRunes)
		}
		// Scroll the view so that the cursor is visible at the top of the screen
		e.ScrollTo(e.cursorY, 0)
		e.currentFindY = e.cursorY
		e.currentFindX = 0
	case tcell.KeyPgDn:
		// Move cursor down by visible height or to bottom, and scroll screen if needed
		pageSize := e.height - 1
		if e.cursorY+pageSize < len(e.content)-1 {
			e.cursorY += pageSize
		} else {
			e.cursorY = len(e.content) - 1
		}
		// Clamp cursorX to line length
		lineRunes := getRunes(e.content[e.cursorY])
		if e.cursorX > len(lineRunes) {
			e.cursorX = len(lineRunes)
		}
		// Scroll the view so that the cursor is visible at the bottom of the screen
		topLine := e.cursorY - (e.height - 1)
		if topLine < 0 {
			topLine = 0
		}
		e.ScrollTo(topLine, 0)
		e.currentFindY = e.cursorY
		e.currentFindX = 0
	default:
		// Insert printable runes
		if e.selection.active {
			// If selection is active, delete the selected text
			e.deleteSelection()
		}
		e.currentFindY = e.cursorY
		e.currentFindX = 0
		r := event.Rune()
		if r != 0 {
			lineRunes := getRunes(e.content[e.cursorY])
			emptyLine := len(lineRunes) == 0
			var last13 bool
			if !emptyLine {
				if lineRunes[len(lineRunes)-1] == '\r' {
					// If the last character is a carriage return, remove it
					lineRunes = lineRunes[:len(lineRunes)-1]
					last13 = true
				}
			}
			if e.cursorX > len(lineRunes) {
				e.cursorX = len(lineRunes)
			}
			var ins []rune
			if r == '\t' {
				ins = []rune{' ', ' ', ' ', ' '}
			} else {
				ins = []rune{r}
			}
			lineRunes = append(lineRunes[:e.cursorX], append(ins, lineRunes[e.cursorX:]...)...)
			if emptyLine || last13 {
				lineRunes = append(lineRunes, '\r')
			}
			setLine(e.cursorY, lineRunes)
			if r == '\t' {
				e.cursorX += 4
			} else {
				e.cursorX++
			}
		}
	}

	// Update selection end point if selection is active
	if e.selection.active {
		e.selection.endX = e.cursorX
		e.selection.endY = e.cursorY
	}

	// Record edit if content changed
	if !equalStringSlices(beforeContent, e.content) {
		e.recordEdit(beforeContent, e.content, beforeX, beforeY, e.cursorX, e.cursorY)
		e.selection.active = false // Clear selection after edit
	}

	e.FillStatusBar()
	e.redraw()
	return nil
}

// Helper function to compare string slices
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// GoToAndHighlightLine moves the cursor to the specified row (0-based), opens the editor view, and highlights that line.
func (e *LuaEditor) GoToAndHighlightLine(row int) {
	if row < 0 {
		row = 0
	}
	if row >= len(e.content) {
		row = len(e.content) - 1
	}
	e.cursorY = row
	// Clamp cursorX to line length
	lineRunes := getRunes(e.content[e.cursorY])
	if e.cursorX > len(lineRunes) {
		e.cursorX = len(lineRunes)
	}
	// Scroll so that the line is visible (put it at top if possible)
	e.ScrollTo(e.cursorY, 0)
	e.redraw()
}

// GetFileName returns the current file name
func (e *LuaEditor) GetFileName() string {
	return e.fileName
}

// SetFileName sets the current file name
func (e *LuaEditor) SetFileName(fileName string) {
	e.fileName = fileName
}

// recordEdit records an edit action for undo/redo
func (e *LuaEditor) recordEdit(beforeContent []string, afterContent []string, beforeX, beforeY, afterX, afterY int) {
	action := EditAction{
		beforeContent: make([]string, len(beforeContent)),
		afterContent:  make([]string, len(afterContent)),
		beforeCursorX: beforeX,
		beforeCursorY: beforeY,
		afterCursorX:  afterX,
		afterCursorY:  afterY,
	}
	copy(action.beforeContent, beforeContent)
	copy(action.afterContent, afterContent)
	e.undoStack = append(e.undoStack, action)
	// Clear redo stack when a new edit is made
	e.redoStack = nil
}

// undo reverts the last edit action
func (e *LuaEditor) undo() {
	if len(e.undoStack) == 0 {
		e.SetStatus("Nothing to undo")
		return
	}

	// Pop the last action from undo stack
	lastIdx := len(e.undoStack) - 1
	action := e.undoStack[lastIdx]
	e.undoStack = e.undoStack[:lastIdx]

	// Save current state for redo
	currentContent := make([]string, len(e.content))
	copy(currentContent, e.content)
	redoAction := EditAction{
		beforeContent: currentContent,
		afterContent:  make([]string, len(action.afterContent)),
		beforeCursorX: e.cursorX,
		beforeCursorY: e.cursorY,
		afterCursorX:  action.afterCursorX,
		afterCursorY:  action.afterCursorY,
	}
	copy(redoAction.afterContent, action.afterContent)
	e.redoStack = append(e.redoStack, redoAction)

	// Restore the previous state
	e.content = make([]string, len(action.beforeContent))
	copy(e.content, action.beforeContent)
	e.cursorX = action.beforeCursorX
	e.cursorY = action.beforeCursorY
	e.redraw()
	e.SetStatus("Undo successful")
}

// redo reapplies the last undone action
func (e *LuaEditor) redo() {
	if len(e.redoStack) == 0 {
		e.SetStatus("Nothing to redo")
		return
	}

	// Pop the last action from redo stack
	lastIdx := len(e.redoStack) - 1
	action := e.redoStack[lastIdx]
	e.redoStack = e.redoStack[:lastIdx]

	// Save current state for undo
	currentContent := make([]string, len(e.content))
	copy(currentContent, e.content)
	undoAction := EditAction{
		beforeContent: currentContent,
		afterContent:  make([]string, len(action.afterContent)),
		beforeCursorX: e.cursorX,
		beforeCursorY: e.cursorY,
		afterCursorX:  action.afterCursorX,
		afterCursorY:  action.afterCursorY,
	}
	copy(undoAction.afterContent, action.afterContent)
	e.undoStack = append(e.undoStack, undoAction)

	// Restore the redone state
	e.content = make([]string, len(action.afterContent))
	copy(e.content, action.afterContent)
	e.cursorX = action.afterCursorX
	e.cursorY = action.afterCursorY
	e.redraw()
	e.SetStatus("Redo successful")
}

// isSelected returns true if the given position is within the selection range
func (s *Selection) isSelected(x, y int) bool {
	if !s.active {
		return false
	}

	// Normalize selection coordinates
	startY, endY := s.startY, s.endY
	startX, endX := s.startX, s.endX
	if startY > endY || (startY == endY && startX > endX) {
		startY, endY = endY, startY
		startX, endX = endX, startX
	}

	if y < startY || y > endY {
		return false
	}
	if y == startY && x < startX {
		return false
	}
	if y == endY && x >= endX {
		return false
	}
	return true
}

// getSelectedText returns the currently selected text without color tags
func (e *LuaEditor) getSelectedText() string {
	if !e.selection.active {
		return ""
	}

	// Normalize selection coordinates
	startY, endY := e.selection.startY, e.selection.endY
	startX, endX := e.selection.startX, e.selection.endX
	if startY > endY || (startY == endY && startX > endX) {
		startY, endY = endY, startY
		startX, endX = endX, startX
	}

	var result strings.Builder

	for y := startY; y <= endY; y++ {
		if y >= len(e.content) {
			break
		}

		line := e.content[y]
		lineRunes := []rune(line)

		start := 0
		if y == startY {
			start = startX
		}

		end := len(lineRunes)
		if y == endY {
			if endX < end {
				end = endX
			}
		}

		if start < len(lineRunes) && end <= len(lineRunes) {
			result.WriteString(string(lineRunes[start:end]))
			if y < endY && !strings.HasSuffix(line, "\r") {
				result.WriteString("\n")
			}
		}
	}

	return result.String()
}

// deleteSelection deletes the selected text and returns it.
// It also updates the editor content and cursor position accordingly.
func (e *LuaEditor) deleteSelection() string {
	if !e.selection.active {
		return ""
	}

	// Normalize selection coordinates
	startY, endY := e.selection.startY, e.selection.endY
	startX, endX := e.selection.startX, e.selection.endX
	if startY > endY || (startY == endY && startX > endX) {
		startY, endY = endY, startY
		startX, endX = endX, startX
	}

	var deletedText strings.Builder

	// Save current state for undo
	beforeContent := make([]string, len(e.content))
	copy(beforeContent, e.content)
	beforeX, beforeY := e.cursorX, e.cursorY

	if startY == endY {
		// Single line selection
		line := []rune(e.content[startY])
		if startX < 0 {
			startX = 0
		}
		if endX > len(line) {
			endX = len(line)
		}
		if startX < endX {
			deletedText.WriteString(string(line[startX:endX]))
			newLine := string(line[:startX]) + string(line[endX:])
			e.content[startY] = newLine
			e.cursorX = startX
			e.cursorY = startY
		}
	} else {
		// Multi-line selection
		// First line: keep before startX, delete from startX to end
		firstLine := []rune(e.content[startY])
		if startX < 0 {
			startX = 0
		}
		if startX > len(firstLine) {
			startX = len(firstLine)
		}
		deletedText.WriteString(string(firstLine[startX:]))
		deletedText.WriteString("\n")

		// Middle lines: delete entirely
		for y := startY + 1; y < endY; y++ {
			if y >= len(e.content) {
				break
			}
			deletedText.WriteString(e.content[y])
			deletedText.WriteString("\n")
		}

		// Last line: keep after endX, delete from 0 to endX
		if endY < len(e.content) {
			lastLine := []rune(e.content[endY])
			if endX > len(lastLine) {
				endX = len(lastLine)
			}
			deletedText.WriteString(string(lastLine[:endX]))
		}

		// Merge first part of first line and last part of last line
		newFirst := string(firstLine[:startX])
		newLast := ""
		if endY < len(e.content) {
			lastLine := []rune(e.content[endY])
			if endX < len(lastLine) {
				newLast = string(lastLine[endX:])
			}
		}
		mergedLine := newFirst + newLast

		// Replace lines in content
		e.content = append(e.content[:startY], append([]string{mergedLine}, e.content[endY+1:]...)...)
		e.cursorX = startX
		e.cursorY = startY
	}

	// Record the edit for undo
	e.recordEdit(beforeContent, e.content, beforeX, beforeY, e.cursorX, e.cursorY)
	e.selection.active = false
	e.redraw()
	return deletedText.String()
}

// copySelection copies the selected text to clipboard
func (e *LuaEditor) copySelection() {
	if !e.selection.active {
		return
	}

	text := e.getSelectedText()
	if text != "" {
		err := clipboard.WriteAll(text)
		if err != nil {
			e.SetErrorStatus(fmt.Sprintf("Failed to copy to clipboard: %v", err))
		} else {
			e.SetStatus("Text copied to clipboard")
		}
	}
}

// pasteFromClipboard pastes text from clipboard at current cursor position
func (e *LuaEditor) pasteFromClipboard() error {
	text, err := clipboard.ReadAll()
	if err != nil {
		e.SetErrorStatus(fmt.Sprintf("Failed to read from clipboard: %v", err))
		return err
	}

	if text == "" {
		return nil
	}

	// Save current state for undo
	beforeContent := make([]string, len(e.content))
	copy(beforeContent, e.content)
	beforeX, beforeY := e.cursorX, e.cursorY

	// Split pasted text into lines
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		// Remove carriage returns
		line = strings.ReplaceAll(line, "\r", "")

		if i == 0 {
			// For first line, insert at cursor position
			currentLine := e.content[e.cursorY]
			runes := []rune(currentLine)
			if e.cursorX > len(runes) {
				e.cursorX = len(runes)
			}
			newLine := string(runes[:e.cursorX]) + line
			if i == len(lines)-1 {
				// If this is the only line, append rest of current line
				newLine += string(runes[e.cursorX:])
			} else {
				// Add carriage return for Windows
				if runtime.GOOS == "windows" {
					newLine += "\r"
				}
			}
			e.content[e.cursorY] = newLine
		} else if i == len(lines)-1 {
			// For last line, append rest of current line
			currentLine := e.content[e.cursorY]
			runes := []rune(currentLine)
			line += string(runes[e.cursorX:])
			// Insert as new line
			e.content = append(e.content[:e.cursorY+i], append([]string{line}, e.content[e.cursorY+i:]...)...)
		} else {
			// For middle lines, insert as new lines
			if runtime.GOOS == "windows" {
				line += "\r"
			}
			e.content = append(e.content[:e.cursorY+i], append([]string{line}, e.content[e.cursorY+i:]...)...)
		}
	}

	// Update cursor position
	if len(lines) > 1 {
		e.cursorY += len(lines) - 1
		e.cursorX = len([]rune(lines[len(lines)-1]))
	} else {
		e.cursorX += len([]rune(lines[0]))
	}

	// Record the edit for undo
	e.recordEdit(beforeContent, e.content, beforeX, beforeY, e.cursorX, e.cursorY)
	e.redraw()
	return nil
}
