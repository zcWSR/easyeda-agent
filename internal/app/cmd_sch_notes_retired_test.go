package app

import (
	"bytes"
	"strings"
	"testing"
)

// Removing Notes must remove its public mutation surface while leaving ordinary
// text inspection available for existing documents and module-title validation.
func TestSchematicNotesCommandRetired(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newSchCmd(&appConfig{}, &out, &errOut)
	foundTextList := false
	for _, child := range cmd.Commands() {
		if child.Name() == "note" {
			t.Fatal("standalone sch note must not be exposed in 1.4")
		}
		if child.Name() == "text-list" {
			foundTextList = true
		}
	}
	if !foundTextList {
		t.Fatal("text-list must remain available for existing annotations and titles")
	}
}

func TestSchematicTextCreateRequiresExactDocBeforeDispatch(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newSchCmd(&appConfig{}, &out, &errOut)
	for _, child := range cmd.Commands() {
		if child.Name() != "text-create" {
			continue
		}
		if err := child.Flags().Set("content", "ABC123"); err != nil {
			t.Fatal(err)
		}
		if err := child.Flags().Set("x", "120"); err != nil {
			t.Fatal(err)
		}
		if err := child.Flags().Set("y", "80"); err != nil {
			t.Fatal(err)
		}
		err := child.RunE(child, nil)
		if err == nil || !strings.Contains(err.Error(), "requires --doc") {
			t.Fatalf("expected exact-document refusal before dispatch, got %v", err)
		}
		return
	}
	t.Fatal("text-create command missing")
}
