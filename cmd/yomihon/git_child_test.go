package main

import (
	"os"
	"testing"

	"github.com/koopa0/yomihon/internal/status"
)

func TestMain(m *testing.M) {
	if handled, exit := status.RunGitChild(os.Args[1:], os.Stderr); handled {
		os.Exit(exit)
	}
	os.Exit(m.Run())
}
