package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"log"
)

type WindowDialog struct {
	ID       string          `json:"id"`
	Kind     string          `json:"kind"`
	Title    string          `json:"title"`
	Width    int             `json:"width"`
	Height   int             `json:"height"`
	Payload  json.RawMessage `json:"payload"`
	Revision uint64          `json:"revision"`
}
type DialogAction struct {
	ID      string          `json:"id"`
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload"`
}
type nativeDialog struct {
	state  WindowDialog
	window *application.WebviewWindow
	order  uint64
}

func dialogWindowOptions(state WindowDialog) application.WebviewWindowOptions {
	options := application.WebviewWindowOptions{Name: "dialog-" + state.ID, Title: state.Title, URL: "/?desktop=1&dialog=" + state.ID, Width: max(320, min(1400, state.Width)), Height: max(180, min(1000, state.Height)), MinWidth: 320, MinHeight: 180, DisableResize: state.Kind != "database", BackgroundColour: application.NewRGB(255, 255, 255), Windows: application.WindowsWindow{Theme: application.Light}}
	var payload struct {
		Mode string `json:"mode"`
	}
	if state.Kind == "database" && json.Unmarshal(state.Payload, &payload) == nil && payload.Mode == "drop" {
		options.Width = 420
		options.Height = 320
		options.DisableResize = true
		options.MinimiseButtonState = application.ButtonHidden
		options.MaximiseButtonState = application.ButtonHidden
		options.FullscreenButtonState = application.ButtonHidden
	}
	return options
}

func validWindowDialog(state WindowDialog) error {
	if len(state.ID) < 8 || len(state.ID) > 80 || len(state.Payload) > 1<<20 || !json.Valid(state.Payload) {
		return errors.New("窗口参数无效")
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(state.Payload, &object) != nil || object == nil || len(state.Title) > 512 {
		return errors.New("窗口内容无效")
	}
	for _, c := range state.ID {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
			return errors.New("窗口标识无效")
		}
	}
	switch state.Kind {
	case "connection", "group", "confirm", "about", "database":
	default:
		return errors.New("未知窗口类型")
	}
	return nil
}
func dispatchWindow(window *application.WebviewWindow, name string, value any) {
	if window == nil {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	// ExecJS targets exactly one WebView. Form credentials are never broadcast to
	// other windows or written to a URL, file, or persistent browser storage.
	window.ExecJS(fmt.Sprintf("window.dispatchEvent(new CustomEvent(%q,{detail:%s}))", name, data))
}
func (a *App) OpenWindowDialog(state WindowDialog) error {
	if err := validWindowDialog(state); err != nil {
		return err
	}
	if a.desktop == nil {
		return errors.New("桌面窗口未初始化")
	}
	a.dialogMu.Lock()
	if _, exists := a.dialogs[state.ID]; exists {
		a.dialogMu.Unlock()
		return a.UpdateWindowDialog(state)
	}
	if len(a.dialogs) >= 8 {
		a.dialogMu.Unlock()
		return errors.New("打开的窗口过多")
	}
	a.dialogMu.Unlock()
	state.Revision = 1
	window := a.desktop.Window.NewWithOptions(dialogWindowOptions(state))
	a.dialogMu.Lock()
	a.dialogSequence++
	a.dialogs[state.ID] = &nativeDialog{state: state, window: window, order: a.dialogSequence}
	a.dialogMu.Unlock()
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		a.dialogMu.Lock()
		_, tracked := a.dialogs[state.ID]
		a.dialogMu.Unlock()
		if tracked {
			event.Cancel()
			go dispatchWindow(window, "bajiu:dialog-close", state.ID)
		}
	})
	window.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		a.dialogMu.Lock()
		entry := a.dialogs[state.ID]
		var snapshot WindowDialog
		if entry != nil {
			snapshot = entry.state
		}
		a.dialogMu.Unlock()
		if entry != nil {
			dispatchWindow(window, "bajiu:dialog-state", snapshot)
		}
		a.traceWindows()
	})
	if windowTrace == "1" {
		window.OnWindowEvent(events.Common.WindowDidMove, func(*application.WindowEvent) { a.traceWindows() })
	}
	a.updateModalWindows()
	window.Center()
	window.Focus()
	return nil
}
func (a *App) traceWindows() {
	if windowTrace != "1" {
		return
	}
	type geometry struct {
		Kind   string `json:"kind"`
		X      int    `json:"x"`
		Y      int    `json:"y"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	type target struct {
		kind   string
		window *application.WebviewWindow
	}
	targets := []target{{"main", a.mainWindow}}
	a.dialogMu.Lock()
	for _, entry := range a.dialogs {
		targets = append(targets, target{entry.state.Kind, entry.window})
	}
	a.dialogMu.Unlock()
	values := []geometry{}
	for _, entry := range targets {
		if entry.window == nil {
			continue
		}
		x, y := entry.window.Position()
		w, h := entry.window.Size()
		values = append(values, geometry{entry.kind, x, y, w, h})
	}
	data, _ := json.Marshal(values)
	log.Printf("native-window-qa %s", data)
}
func (a *App) GetWindowDialog(id string) (WindowDialog, error) {
	a.dialogMu.Lock()
	defer a.dialogMu.Unlock()
	entry := a.dialogs[id]
	if entry == nil {
		return WindowDialog{}, errors.New("窗口已关闭")
	}
	return entry.state, nil
}
func (a *App) UpdateWindowDialog(state WindowDialog) error {
	if err := validWindowDialog(state); err != nil {
		return err
	}
	a.dialogMu.Lock()
	entry := a.dialogs[state.ID]
	if entry == nil {
		a.dialogMu.Unlock()
		return errors.New("窗口已关闭")
	}
	if entry.state.Kind != state.Kind {
		a.dialogMu.Unlock()
		return errors.New("窗口类型已变化")
	}
	state.Revision = entry.state.Revision + 1
	entry.state = state
	window := entry.window
	a.dialogMu.Unlock()
	window.SetTitle(state.Title)
	dispatchWindow(window, "bajiu:dialog-state", state)
	return nil
}
func (a *App) CloseWindowDialog(id string) {
	a.dialogMu.Lock()
	entry := a.dialogs[id]
	delete(a.dialogs, id)
	a.dialogMu.Unlock()
	if entry != nil {
		entry.window.Close()
	}
	a.updateModalWindows()
}
func (a *App) SendWindowDialogAction(action DialogAction) error {
	if len(action.Payload) > 1<<20 || !json.Valid(action.Payload) {
		return errors.New("窗口操作参数无效")
	}
	a.dialogMu.Lock()
	_, ok := a.dialogs[action.ID]
	a.dialogMu.Unlock()
	if !ok {
		return errors.New("窗口已关闭")
	}
	dispatchWindow(a.mainWindow, "bajiu:dialog-action", action)
	return nil
}
func (a *App) updateModalWindows() {
	a.dialogMu.Lock()
	type windowEntry struct {
		kind   string
		window *application.WebviewWindow
		order  uint64
	}
	entries := make([]windowEntry, 0, len(a.dialogs))
	confirm := false
	for _, entry := range a.dialogs {
		entries = append(entries, windowEntry{entry.state.Kind, entry.window, entry.order})
		if entry.state.Kind == "confirm" {
			confirm = true
		}
	}
	a.dialogMu.Unlock()
	if a.mainWindow != nil {
		a.mainWindow.SetEnabled(len(entries) == 0)
	}
	for _, entry := range entries {
		entry.window.SetEnabled(!confirm || entry.kind == "confirm")
	}
	var focus *application.WebviewWindow
	var order uint64
	for _, entry := range entries {
		if (!confirm || entry.kind == "confirm") && entry.order >= order {
			focus = entry.window
			order = entry.order
		}
	}
	if focus != nil {
		focus.Restore()
		focus.Focus()
	}
	if len(entries) == 0 && a.mainWindow != nil {
		a.mainWindow.Focus()
	}
}
