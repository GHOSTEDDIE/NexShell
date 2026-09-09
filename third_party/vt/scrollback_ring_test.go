package vt

import (
	"fmt"
	uv "github.com/charmbracelet/ultraviolet"
	"math/rand"
	"reflect"
	"testing"
)

func TestScrollbackRingMatchesOrderedHistory(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	s := NewScrollback(5)
	var want []uv.Line
	limit := 5
	for step := 0; step < 2000; step++ {
		switch random.Intn(10) {
		case 0:
			s.Clear()
			want = nil
		case 1, 2:
			limit = 1 + random.Intn(25)
			s.SetMaxLines(limit)
			if len(want) > limit {
				want = want[len(want)-limit:]
			}
		default:
			text := fmt.Sprintf("中文é-%d", step)
			line := uv.Line{{Content: text, Width: 2}}
			s.Push(line)
			want = append(want, uv.Line{{Content: text, Width: 2}})
			line[0].Content = "must not alias source"
			if len(want) > limit {
				want = want[1:]
			}
		}
		if s.Len() != len(want) || s.MaxLines() != limit {
			t.Fatal("length mismatch", step)
		}
		lines := s.Lines()
		for i := range want {
			if !reflect.DeepEqual(s.Line(i), want[i]) || !reflect.DeepEqual(lines[i], want[i]) || *s.CellAt(0, i) != want[i][0] {
				t.Fatalf("order changed at step %d line %d", step, i)
			}
		}
		if s.Line(-1) != nil || s.Line(s.Len()) != nil || s.CellAt(999, 0) != nil {
			t.Fatal("bounds changed")
		}
	}
}
func TestScrollbackClearReleasesCells(t *testing.T) {
	s := NewScrollback(3)
	for i := 0; i < 8; i++ {
		s.Push(uv.Line{{Content: "retained", Width: 1}})
	}
	slots := s.lines
	s.Clear()
	for _, line := range slots {
		if line != nil {
			t.Fatal("clear retained a line")
		}
	}
}
