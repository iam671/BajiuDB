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
