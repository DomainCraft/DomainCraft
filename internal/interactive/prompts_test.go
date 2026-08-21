package interactive

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/DomainCraft/DomainCraft/internal/bridge"
)

// setInput replaces the prompt input with the given script of keypresses and
// silences prompt output. Arrow keys are written as their escape sequences.
func setInput(s string) {
	promptInput = strings.NewReader(s)
	promptOutput = io.Discard
	promptReader = nil
}

func TestConfirm(t *testing.T) {
	cases := []struct {
		input string
		want  bool
		err   error
	}{
		{"\n", false, nil},             // Enter on default (No)
		{"\x1b[D\n", true, nil},        // ← moves to Yes
		{"\x1b[C\n", false, nil},       // → stays on No
		{"\x1b[D\x1b[C\n", false, nil}, // Yes then back to No
		{"\x1b", false, errAborted},    // Esc cancels
	}
	for _, c := range cases {
		setInput(c.input)
		got, err := confirm("?", "")
		if !errors.Is(err, c.err) {
			t.Fatalf("confirm(%q) error = %v, want %v", c.input, err, c.err)
		}
		if err != nil {
			continue
		}
		if got != c.want {
			t.Errorf("confirm(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestSelectBridge(t *testing.T) {
	reg := bridge.NewRegistry([]bridge.RegistryEntry{
		{ID: "a", Name: "Alpha", Language: "Go", Description: "first"},
		{ID: "b", Name: "Beta", Language: "C#", Description: "second"},
	})

	// explicit choice
	setInput("2\n")
	got, err := SelectBridge(reg)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "b" {
		t.Errorf("SelectBridge(2) = %q, want b", got.ID)
	}

	// default = first
	setInput("\n")
	got, err = SelectBridge(reg)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "a" {
		t.Errorf("SelectBridge(default) = %q, want a", got.ID)
	}

	// out-of-range digit is ignored, then 1 moves to the first
	setInput("91\n")
	got, err = SelectBridge(reg)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "a" {
		t.Errorf("SelectBridge(9 then 1) = %q, want a", got.ID)
	}

	// arrow down then Enter picks the second
	setInput("\x1b[B\n")
	got, err = SelectBridge(reg)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "b" {
		t.Errorf("SelectBridge(down) = %q, want b", got.ID)
	}

	// Esc aborts
	setInput("\x1b")
	if _, err = SelectBridge(reg); !errors.Is(err, errAborted) {
		t.Errorf("SelectBridge(esc) error = %v, want errAborted", err)
	}
}

func TestMultiSelect(t *testing.T) {
	files := []FileChoice{
		{Path: "p1", Label: "f1"},
		{Path: "p2", Label: "f2"},
		{Path: "p3", Label: "f3"},
	}

	// Enter confirms everything (all preselected)
	setInput("\n")
	got, err := multiSelect("t", "", files)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("default selection = %v, want all 3", got)
	}

	// toggle off item 2
	setInput("2\n")
	got, err = multiSelect("t", "", files)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "p1" || got[1] != "p3" {
		t.Errorf("after toggle 2 = %v, want [p1 p3]", got)
	}

	// toggle off 1 and 3, leaving only item 2
	setInput("13\n")
	got, err = multiSelect("t", "", files)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "p2" {
		t.Errorf("13 = %v, want [p2]", got)
	}

	// out-of-range '9' is ignored; 1,2 toggle the first two off
	setInput("912\n")
	got, err = multiSelect("t", "", files)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "p3" {
		t.Errorf("9 then 1,2 = %v, want [p3]", got)
	}

	// Space toggles the focused item, Enter confirms
	setInput(" \n") // toggle item 1 off
	got, err = multiSelect("t", "", files)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "p2" || got[1] != "p3" {
		t.Errorf("space = %v, want [p2 p3]", got)
	}

	// Esc aborts
	setInput("\x1b")
	if _, err = multiSelect("t", "", files); !errors.Is(err, errAborted) {
		t.Errorf("multiSelect(esc) error = %v, want errAborted", err)
	}
}

func TestPromptDeleteFilesEmpty(t *testing.T) {
	got, err := PromptDeleteFiles("Gone", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("PromptDeleteFiles(nil) = %v, want nil", got)
	}
}
