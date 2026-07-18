package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConceptSheetKeepsPunctuationDistinctConceptsSeparate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	conceptDir := filepath.Join(root, "Concepts", "golang")
	if err := os.MkdirAll(conceptDir, 0o750); err != nil {
		t.Fatalf("mkdir concepts: %v", err)
	}
	concepts := map[string]string{
		"A-B.md": "hyphenated concept body",
		"A B.md": "spaced concept body",
	}
	for name, body := range concepts {
		if err := os.WriteFile(filepath.Join(conceptDir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write concept %q: %v", name, err)
		}
	}

	lessonDir := filepath.Join(root, "Writing", "lessons", "golang")
	if err := os.MkdirAll(lessonDir, 0o750); err != nil {
		t.Fatalf("mkdir lesson: %v", err)
	}
	lessonBody := "---\ntitle: Collision lesson\ntype: lesson\nstatus: draft\n---\n\nSee [[A-B]] and [[A B]].\n"
	if err := os.WriteFile(filepath.Join(lessonDir, "Collision.md"), []byte(lessonBody), 0o600); err != nil {
		t.Fatalf("write lesson: %v", err)
	}

	srv := newServerWithContract(t, root, loadContract(t))
	code, page := get(t, srv.URL+"/notes/Writing/lessons/golang/Collision.md")
	if code != http.StatusOK {
		t.Fatalf("GET lesson status = %d, want %d", code, http.StatusOK)
	}
	tests := []struct {
		id   string
		body string
	}{
		{id: "path-Q29uY2VwdHMvZ29sYW5nL0EtQi5tZA", body: "hyphenated concept body"},
		{id: "path-Q29uY2VwdHMvZ29sYW5nL0EgQi5tZA", body: "spaced concept body"},
	}
	for _, tt := range tests {
		if !strings.Contains(page, `data-concept="`+tt.id+`"`) {
			t.Errorf("reading page is missing trigger ID %q", tt.id)
		}
		if !strings.Contains(page, `<template id="concept-`+tt.id+`"`) {
			t.Errorf("reading page is missing template ID %q", tt.id)
		}
		if !strings.Contains(page, tt.body) {
			t.Errorf("reading page is missing body for concept ID %q", tt.id)
		}
	}
	for _, href := range []string{
		`href="/notes/Concepts/golang/A-B.md"`,
		`href="/notes/Concepts/golang/A%20B.md"`,
	} {
		if !strings.Contains(page, href) {
			t.Errorf("ordinary concept link lost navigable %s", href)
		}
	}
}
