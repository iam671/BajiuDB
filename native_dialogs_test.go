package main

import (
	"encoding/json"
	"github.com/wailsapp/wails/v3/pkg/application"
	"testing"
)

func TestDropDialogWindowOptions(t *testing.T) {
	options := dialogWindowOptions(WindowDialog{Kind: "database", Width: 1000, Height: 650, Payload: json.RawMessage(`{"mode":"drop"}`)})
	if options.Width != 420 || options.Height != 320 || !options.DisableResize || options.MinimiseButtonState != application.ButtonHidden || options.MaximiseButtonState != application.ButtonHidden || options.FullscreenButtonState != application.ButtonHidden {
		t.Fatal("drop database dialog must stay compact without minimise, maximise or fullscreen buttons")
	}
	query := dialogWindowOptions(WindowDialog{Kind: "database", Width: 1000, Height: 650, Payload: json.RawMessage(`{"mode":"query"}`)})
	if query.DisableResize || query.MinimiseButtonState != application.ButtonEnabled || query.MaximiseButtonState != application.ButtonEnabled {
		t.Fatal("query workspace should remain resizable")
	}
}

func TestWindowDialogValidation(t *testing.T) {
	good := WindowDialog{ID: "qa-native-window", Kind: "connection", Payload: json.RawMessage(`{"model":{"name":"qa-only"}}`)}
	if err := validWindowDialog(good); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*WindowDialog){func(d *WindowDialog) { d.ID = "../../other" }, func(d *WindowDialog) { d.Kind = "arbitrary" }, func(d *WindowDialog) { d.Payload = json.RawMessage(`not JSON`) }} {
		copy := good
		change(&copy)
		if validWindowDialog(copy) == nil {
			t.Fatal("accepted invalid dialog")
		}
	}
}

func TestIndexFieldsWindow(t *testing.T) {
	state := WindowDialog{ID: "qa-index-fields", Kind: "index-fields", Width: 480, Height: 440, Payload: json.RawMessage(`{"fields":[],"value":"","kind":"NORMAL","method":"","sqlMode":""}`)}
	if err := validWindowDialog(state); err != nil {
		t.Fatal(err)
	}
	options := dialogWindowOptions(state)
	if options.DisableResize || options.Width != 480 || options.Height != 440 {
		t.Fatal("index field selection must use a resizable native window")
	}
}

func TestCellValueWindow(t *testing.T) {
	state := WindowDialog{ID: "qa-cell-value", Kind: "cell-value", Width: 380, Height: 460, Payload: json.RawMessage(`{"cell":{"mode":"value","value":"2026-11-18"}}`)}
	if err := validWindowDialog(state); err != nil {
		t.Fatal(err)
	}
	options := dialogWindowOptions(state)
	if options.DisableResize || options.Width != 380 || options.Height != 460 {
		t.Fatal("cell value editor must use an independent resizable native window")
	}
}
func TestConnectionTypePickerNativeWindow(t *testing.T) {
	state := WindowDialog{ID: "qa-connection-picker", Kind: "connection-type", Title: "选择连接类型", Width: 800, Height: 570, Payload: json.RawMessage(`{"recent":["mysql","redis"]}`)}
	if err := validWindowDialog(state); err != nil {
		t.Fatal(err)
	}
	options := dialogWindowOptions(state)
	if options.DisableResize || options.Width != 800 || options.Height != 570 || options.MinWidth != 520 || options.MinHeight != 360 {
		t.Fatal("type picker must be an independent resizable native window")
	}
}
