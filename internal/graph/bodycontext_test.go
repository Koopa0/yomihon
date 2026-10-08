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
	for _, value := range []any{"invalid", (*bodyObservation)(nil)} {
		t.Run("invalid observation", func(t *testing.T) {
			context := parser.NewContext()
			context.Set(bodyObservationKey, value)
			defer func() {
				if got := recover(); got != "graph: invalid body observation context" {
					t.Errorf("caught: invalid context rejection = %v", got)
				}
			}()
			bodyObservationIn(context)
		})
	}
}
