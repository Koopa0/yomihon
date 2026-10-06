package main

import (
	"fmt"
	"testing"
)

func TestLoadConfigRejectsInvalidPorts(t *testing.T) {
	for _, value := range []string{"abc", "http", "-1", "65536", "70000", "99999999999999999999999999", " 9924", "9924 ", "1.2", "0x26c4", "９９２４"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("YOMIHON_PORT", value)
			_, err := loadConfig(t.TempDir())
			want := fmt.Sprintf("YOMIHON_PORT %q must be an integer from 0 to 65535", value)
			if err == nil || err.Error() != want {
				t.Errorf("caught: loadConfig(port %q) error = %v, want %q", value, err, want)
			}
		})
	}
}

func TestLoadConfigKeepsValidPortSpellings(t *testing.T) {
	for _, test := range []struct {
		value string
		want  string
	}{
		{value: "", want: "9610"},
		{value: "0", want: "0"},
		{value: "1", want: "1"},
		{value: "65535", want: "65535"},
		{value: "9924", want: "9924"},
		{value: "009924", want: "009924"},
		{value: "+9924", want: "+9924"},
	} {
		t.Run(test.value, func(t *testing.T) {
			t.Setenv("YOMIHON_PORT", test.value)
			cfg, err := loadConfig(t.TempDir())
			if err != nil {
				t.Fatalf("caught: loadConfig(port %q) error = %v, want none", test.value, err)
			}
			if cfg.port != test.want {
				t.Errorf("caught: loadConfig(port %q) port = %q, want %q", test.value, cfg.port, test.want)
			}
		})
	}
}
