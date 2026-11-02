package imageout

import (
	"errors"
	"fmt"
	"image"
	"reflect"
	"testing"

	"github.com/AmadeusDE/Misa/internal/imageframe"
)

type transcriptSink struct {
	events        []string
	layers        []CellUpdate
	fail          string
	cursor, saved int
}

func (s *transcriptSink) event(v string) error {
	s.events = append(s.events, v)
	if v != "begin" && v != "end" {
		s.cursor++
	}
	if s.fail == v {
		return errors.New("injected")
	}
	return nil
}
func (s *transcriptSink) Begin() error { s.saved = s.cursor; return s.event("begin") }
func (s *transcriptSink) Delete(e imageframe.Effect) error {
	return s.event(fmt.Sprintf("delete:%d:%d", e.Owner, e.Resource))
}
func (s *transcriptSink) Text(text string, layers []CellUpdate) error {
	s.layers = layers
	return s.event("text:" + text)
}
func (s *transcriptSink) Transfer(e imageframe.Effect) error {
	return s.event(fmt.Sprintf("transfer:%d:%d", e.Owner, e.Resource))
}
func (s *transcriptSink) Place(e imageframe.Effect) error {
	return s.event(fmt.Sprintf("place:%d:%d", e.Owner, e.Resource))
}
func (s *transcriptSink) End() error { s.cursor = s.saved; return s.event("end") }
func frame(owner, gen, resource uint64) imageframe.Frame {
	raster := &imageframe.Raster{ID: resource, Pixels: image.NewNRGBA(image.Rect(0, 0, 1, 1))}
	placement := &imageframe.Placement{ID: 1, Raster: raster, Source: image.Rect(0, 0, 1, 1), Dest: image.Rect(0, 0, 1, 1), Clip: image.Rect(0, 0, 1, 1)}
	return imageframe.Frame{Owner: owner, Generation: gen, Presentation: imageframe.Presentation{Generation: 1}, Effects: []imageframe.Effect{
		{Kind: imageframe.Transfer, Owner: owner, Resource: resource, Raster: raster},
		{Kind: imageframe.Place, Owner: owner, Resource: resource, Placement: 1, PlacementData: placement},
	}}
}
func TestSessionOrderingOwnershipAndCleanup(t *testing.T) {
	sink := new(transcriptSink)
	s := NewSession(sink, 2)
	a, _ := s.Register(1)
	b, _ := s.Register(2)
	if err := s.Submit(frame(1, a, 7)); err != nil {
		t.Fatal(err)
	}
	if err := s.Submit(frame(2, b, 7)); err != nil {
		t.Fatal(err)
	}
	acks, err := s.Commit("first")
	if err != nil || len(acks) != 2 {
		t.Fatalf("%v %v", acks, err)
	}
	sink.events = nil
	if err := s.Release(1); err != nil {
		t.Fatal(err)
	}
	if err := s.Release(1); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Commit("second"); err != nil {
		t.Fatal(err)
	}
	want := []string{"begin", "delete:1:7", "text:second", "end"}
	if !reflect.DeepEqual(sink.events, want) {
		t.Fatalf("%v", sink.events)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	sink.events = nil
	if _, err = s.Commit("exit"); err != nil {
		t.Fatal(err)
	}
	want = []string{"begin", "delete:2:7", "text:exit", "end"}
	if !reflect.DeepEqual(sink.events, want) {
		t.Fatal(sink.events)
	}
}
func TestSessionBackpressureAndInvalidation(t *testing.T) {
	sink := new(transcriptSink)
	s := NewSession(sink, 1)
	g, _ := s.Register(1)
	if _, err := s.Register(2); err == nil {
		t.Fatal("owner cap")
	}
	f := frame(1, g, 1)
	if err := s.Submit(f); err != nil {
		t.Fatal(err)
	}
	f = frame(1, g, 2)
	f.Presentation.Generation = 2
	if err := s.Submit(f); err != nil {
		t.Fatal(err)
	}
	a, err := s.Commit("frame")
	if err != nil || len(a) != 1 {
		t.Fatalf("%v %v", a, err)
	}
	want := []string{"begin", "text:frame", "transfer:1:2", "place:1:2", "end"}
	if !reflect.DeepEqual(sink.events, want) {
		t.Fatal(sink.events)
	}
	if _, err = s.SetVisible(1, false); err != nil {
		t.Fatal(err)
	}
	if err = s.Submit(f); err == nil {
		t.Fatal("hidden frame accepted")
	}
	ng, err := s.SetVisible(1, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Submit(f); err == nil {
		t.Fatal("stale frame accepted")
	}
	ng, err = s.Resize(1)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Submit(frame(1, g, 3)); err == nil {
		t.Fatal("stale resize")
	}
	if err = s.Submit(frame(1, ng, 3)); err != nil {
		t.Fatal(err)
	}
	if err = s.Submit(frame(2, ng, 3)); err == nil {
		t.Fatal("unknown owner")
	}
}
func TestSessionFailureRestoresAndDoesNotAcknowledge(t *testing.T) {
	sink := &transcriptSink{fail: "text:x"}
	s := NewSession(sink, 1)
	g, _ := s.Register(1)
	_ = s.Submit(frame(1, g, 1))
	a, err := s.Commit("x")
	if err == nil || len(a) != 0 {
		t.Fatalf("%v %v", a, err)
	}
	if sink.events[len(sink.events)-1] != "end" {
		t.Fatal(sink.events)
	}
	if _, err = s.Commit("again"); err == nil {
		t.Fatal("failed session reused")
	}
}

func TestSessionDeleteBeforeTextAndValidation(t *testing.T) {
	sink := new(transcriptSink)
	s := NewSession(sink, 2)
	g, _ := s.Register(1)
	_, _ = s.Register(2)
	if err := s.Submit(frame(1, g, 5)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit("old"); err != nil {
		t.Fatal(err)
	}
	bad := frame(1, g, 6)
	bad.Effects = append(bad.Effects, imageframe.Effect{Kind: imageframe.Delete, Owner: 2, Resource: 5})
	if s.Submit(bad) == nil {
		t.Fatal("cross owner accepted")
	}
	bad.Effects[len(bad.Effects)-1].Owner = 1
	bad.Effects[len(bad.Effects)-1].Resource = 99
	if s.Submit(bad) == nil {
		t.Fatal("unowned deletion accepted")
	}
	f := frame(1, g, 6)
	f.Presentation.Generation = 2
	f.Effects = append(f.Effects, imageframe.Effect{Kind: imageframe.Delete, Owner: 1, Resource: 5})
	if err := s.Submit(f); err != nil {
		t.Fatal(err)
	}
	sink.events = nil
	if _, err := s.Commit("new"); err != nil {
		t.Fatal(err)
	}
	want := []string{"begin", "delete:1:5", "text:new", "transfer:1:6", "place:1:6", "end"}
	if !reflect.DeepEqual(sink.events, want) {
		t.Fatal(sink.events)
	}
}

func TestSessionRejectsOutOfOrderAndRepeatedPresentation(t *testing.T) {
	sink := new(transcriptSink)
	s := NewSession(sink, 1)
	g, _ := s.Register(1)
	newer := frame(1, g, 2)
	newer.Presentation.Generation = 2
	if err := s.Submit(newer); err != nil {
		t.Fatal(err)
	}
	if s.Submit(frame(1, g, 1)) == nil {
		t.Fatal("accepted stale presentation")
	}
	if s.Submit(newer) == nil {
		t.Fatal("accepted repeated pending presentation")
	}
	if _, err := s.Commit("current"); err != nil {
		t.Fatal(err)
	}
	if s.Submit(newer) == nil {
		t.Fatal("accepted repeated committed presentation")
	}
	sink.events = nil
	a, err := s.Commit("unchanged")
	if err != nil || len(a) != 0 {
		t.Fatalf("%v %v", a, err)
	}
	if !reflect.DeepEqual(sink.events, []string{"begin", "text:unchanged", "end"}) {
		t.Fatal(sink.events)
	}
}

func TestSessionBeginAndEndErrors(t *testing.T) {
	for _, phase := range []string{"begin", "end"} {
		t.Run(phase, func(t *testing.T) {
			sink := &transcriptSink{fail: phase}
			s := NewSession(sink, 1)
			g, _ := s.Register(1)
			_ = s.Submit(frame(1, g, 1))
			a, err := s.Commit("x")
			if err == nil || len(a) != 0 {
				t.Fatalf("%v %v", a, err)
			}
			if sink.events[len(sink.events)-1] != "end" {
				t.Fatal(sink.events)
			}
			if phase == "end" && !containsEvent(sink.events, "delete:1:1") {
				t.Fatal(sink.events)
			}
		})
	}
}
func containsEvent(events []string, want string) bool {
	for _, e := range events {
		if e == want {
			return true
		}
	}
	return false
}

func TestSessionTwoPaneResize(t *testing.T) {
	sink := new(transcriptSink)
	s := NewSession(sink, 2)
	a, _ := s.Register(1)
	b, _ := s.Register(2)
	_ = s.Submit(frame(1, a, 7))
	_ = s.Submit(frame(2, b, 7))
	if _, err := s.Commit("both"); err != nil {
		t.Fatal(err)
	}
	want := []string{"begin", "text:both", "transfer:1:7", "transfer:2:7", "place:1:7", "place:2:7", "end"}
	if !reflect.DeepEqual(sink.events, want) {
		t.Fatal(sink.events)
	}
	a, err := s.Resize(1)
	if err != nil {
		t.Fatal(err)
	}
	f := frame(1, a, 8)
	if err = s.Submit(f); err != nil {
		t.Fatal(err)
	}
	sink.events = nil
	acks, err := s.Commit("resized")
	if err != nil || len(acks) != 1 || acks[0].Generation != a {
		t.Fatalf("%v %v", acks, err)
	}
	want = []string{"begin", "delete:1:7", "text:resized", "transfer:1:8", "place:1:8", "end"}
	if !reflect.DeepEqual(sink.events, want) {
		t.Fatal(sink.events)
	}
}
func TestSessionLimitsEffectsAndResources(t *testing.T) {
	s := NewSession(new(transcriptSink), 1)
	g, _ := s.Register(1)
	f := frame(1, g, 1)
	f.Effects = make([]imageframe.Effect, maxSessionEffects+1)
	if s.Submit(f) == nil {
		t.Fatal("effect limit")
	}
	f = frame(1, g, 1)
	f.Effects[0].Raster = &imageframe.Raster{ID: 2}
	if s.Submit(f) == nil {
		t.Fatal("resource mismatch")
	}
}

func TestSessionCellLayerInTextTransaction(t *testing.T) {
	sink := new(transcriptSink)
	s := NewSession(sink, 1)
	g, _ := s.Register(1)
	f := frame(1, g, 1)
	f.Cells = &imageframe.CellLayer{Cells: []imageframe.Cell{{Text: "a"}}}
	if err := s.Submit(f); err != nil {
		t.Fatal(err)
	}
	f.Cells.Cells[0].Text = "changed"
	if _, err := s.Commit("complete"); err != nil {
		t.Fatal(err)
	}
	if len(sink.layers) != 1 || sink.layers[0].Owner != 1 || sink.layers[0].Layer.Cells[0].Text != "a" {
		t.Fatal(sink.layers)
	}
}

func TestSessionBoundaryAndCursorPreservation(t *testing.T) {
	for _, failure := range []string{"", "text:complete", "end", "begin"} {
		t.Run(failure, func(t *testing.T) {
			sink := &transcriptSink{cursor: 42}
			s := NewSession(sink, 1)
			g, err := s.Register(1)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Submit(frame(1, g, 1)); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Resize(1); err != nil {
				t.Fatal(err)
			}
			g, err = s.SetVisible(1, false)
			if err != nil {
				t.Fatal(err)
			}
			g, err = s.SetVisible(1, true)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Submit(frame(1, g, 2)); err != nil {
				t.Fatal(err)
			}
			if len(sink.events) != 0 || sink.cursor != 42 {
				t.Fatalf("output before commit: %+v", sink)
			}
			sink.fail = failure
			_, err = s.Commit("complete")
			if (err != nil) != (failure != "") {
				t.Fatalf("error: %v", err)
			}
			if sink.cursor != 42 {
				t.Fatalf("cursor not restored: %d", sink.cursor)
			}
		})
	}
	sink := new(transcriptSink)
	s := NewSession(sink, 1)
	g, _ := s.Register(1)
	_ = s.Submit(frame(1, g, 1))
	_, _ = s.Commit("first")
	sink.events = nil
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) != 0 {
		t.Fatal(sink.events)
	}
}
