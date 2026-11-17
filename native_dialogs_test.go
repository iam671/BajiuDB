package main

import (
	"encoding/json"
	"testing"
)

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
