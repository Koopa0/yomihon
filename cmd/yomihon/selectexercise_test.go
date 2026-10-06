package main

import (
	"context"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/wording"
)

// A buffered value alone never demonstrates selection among ready cases. The
// lesson must tell the reader to expire the timer too, then explain both results.
func TestExampleSelectExerciseMakesBothCasesReady(t *testing.T) {
	site := exampleSite(t)
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			page := readingPageIn(t, site, "/notes/Lessons/go/G06%20select%20%E8%88%87%E9%80%BE%E6%99%82.md", lang)
			prose := words(elementInner(t, page, `<div class="y-prose"`))
			for _, phrase := range []string{
				"把 values 改成容量為一",
				"呼叫 receive 前先送入 7",
				"把 timeout 改成 0",
				"重複執行多次",
				"會看到 <nil> 與 等待逾時 兩種結果",
				"<nil> 表示這次接到了 7",
				"select 以均勻的偽隨機方式選一個",
				"再加一個 default",
				"default 只在沒有其他分支可進行時執行，而且立即執行",
			} {
				if !strings.Contains(prose, phrase) {
					t.Errorf("caught: select exercise omits %q", phrase)
				}
			}
		})
	}

	lesson, err := os.ReadFile("../../examples/vault/Lessons/go/G06 select 與逾時.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(lesson), "```go\n") != 1 {
		t.Fatal("select lesson must carry exactly one Go example")
	}
	_, tail, _ := strings.Cut(string(lesson), "```go\n")
	code, _, closed := strings.Cut(tail, "```\n")
	if !closed {
		t.Fatal("select lesson Go example is unclosed")
	}
	formatted, err := format.Source([]byte(code))
	if err != nil {
		t.Fatalf("format lesson Go example: %v", err)
	}
	if string(formatted) != code {
		t.Error("select lesson Go example is not gofmt-clean")
	}
	// Apply the exercise to the whole example, preserving what main prints.
	// Renaming main lets repeated calls each use their own channel and timer.
	for _, edit := range []struct{ before, after string }{
		{"values := make(chan int)", "values := make(chan int, 1)\n\tvalues <- 7"},
		{"receive(ctx, values, 20*time.Millisecond)", "receive(ctx, values, 0)"},
		{"func main() {", "func exercise() {"},
	} {
		if strings.Count(code, edit.before) != 1 {
			t.Fatalf("select exercise edit %q must match exactly once", edit.before)
		}
		code = strings.Replace(code, edit.before, edit.after, 1)
	}
	source := code + "\nfunc main() {\n\tfor range 1024 {\n\t\texercise()\n\t}\n}\n"
	path := filepath.Join(t.TempDir(), "exercise.go")
	if writeErr := os.WriteFile(path, []byte(source), 0o600); writeErr != nil { // #nosec G703 -- the test creates this file inside its own temporary directory
		t.Fatalf("write select exercise: %v", writeErr)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "go", "run", "-trimpath", "-p", "2", path).CombinedOutput() // #nosec G204 -- fixed Go command and test-owned temporary source
	if err != nil {
		t.Fatalf("run select exercise: %v\n%s", err, output)
	}
	var values, timeouts int
	for line := range strings.SplitSeq(strings.TrimSpace(string(output)), "\n") {
		switch line {
		case "<nil>":
			values++
		case "等待逾時":
			timeouts++
		default:
			t.Fatalf("caught: select exercise printed %q, want <nil> or 等待逾時", line)
		}
	}
	if values <= 0 || timeouts <= 0 || values+timeouts != 1024 {
		t.Errorf("caught: repeated select exercise produced values=%d timeouts=%d, want both results over 1024 runs", values, timeouts)
	}
}
