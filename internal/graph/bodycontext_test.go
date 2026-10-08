package graph

import (
	"testing"

	"github.com/yuin/goldmark/parser"
)

func TestBodyObservationContextInvariant(t *testing.T) {
	context := parser.NewContext()
	if bodyObservationIn(context) != nil {
		t.Fatal("caught: absent observation context became a request")
	}
	want := &bodyObservation{}
	context.Set(bodyObservationKey, want)
	if bodyObservationIn(context) != want {
		t.Fatal("caught: valid observation context was replaced")
	}
	for _, tt := range []struct {
		name  string
		value any
		want  string
	}{
		{name: "invalid type", value: "invalid", want: "graph: unknown bodyObservation: string"},
		{name: "typed nil", value: (*bodyObservation)(nil), want: "graph: unknown bodyObservation: *graph.bodyObservation"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			context := parser.NewContext()
			context.Set(bodyObservationKey, tt.value)
			defer func() {
				if got := recover(); got != tt.want {
					t.Errorf("caught: invalid context rejection = %v", got)
				}
			}()
			bodyObservationIn(context)
		})
	}
}
