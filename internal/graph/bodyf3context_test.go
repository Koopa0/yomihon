package graph

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/parser"
)

func TestBodyFactsF3ContextControl(t *testing.T) {
	t.Log("AGREEMENT-INVOKED F3/f3-context-invalid")
	type result struct {
		Observation *bodyObservation
		Panic       string
		Panicked    bool
	}
	read := func(context parser.Context) (got result) {
		defer func() {
			if value := recover(); value != nil {
				got.Panicked = true
				var ok bool
				got.Panic, ok = value.(string)
				if !ok {
					got.Panic = fmt.Sprintf("non-string panic %T: %v", value, value)
				}
			}
		}()
		got.Observation = bodyObservationIn(context)
		return got
	}
	observation := &bodyObservation{}
	for _, tt := range []struct {
		name  string
		set   bool
		value any
		want  result
	}{
		{name: "absent"},
		{name: "present", set: true, value: observation, want: result{Observation: observation}},
		{name: "wrong type", set: true, value: "invalid", want: result{Panic: "graph: invalid body observation context", Panicked: true}},
		{name: "typed nil", set: true, value: (*bodyObservation)(nil), want: result{Panic: "graph: invalid body observation context", Panicked: true}},
	} {
		context := parser.NewContext()
		if tt.set {
			context.Set(bodyObservationKey, tt.value)
		}
		got := read(context)
		if diff := cmp.Diff(tt.want, got, cmp.Comparer(func(a, b *bodyObservation) bool { return a == b })); diff != "" {
			t.Errorf("caught: F3 invalid-context %s (-want +got):\n%s", tt.name, diff)
		}
	}
}
