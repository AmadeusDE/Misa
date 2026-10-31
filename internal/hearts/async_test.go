package hearts

import (
	"reflect"
	"testing"
)

func TestDecisionSnapshotSurvivesConcurrentRestart(t *testing.T) {
	tab := &table{state: NewState(42, 100), committed: true, selected: map[int]bool{0: true, 1: true, 2: true}}
	work := tab.Work()
	want := work()
	done := make(chan bool, 1)
	go func() {
		for i := 0; i < 100; i++ {
			if !reflect.DeepEqual(work(), want) {
				done <- false
				return
			}
		}
		done <- true
	}()
	for i := 0; i < 100; i++ {
		tab.state = NewState(uint64(i), 100)
		tab.selected = map[int]bool{}
		tab.committed = false
	}
	if !<-done {
		t.Fatal("pass decision retained mutable game state")
	}
	tab.state.Passing = false
	tab.state.Turn = 1
	work = tab.Work()
	want = work()
	go func(work func() any, want any) {
		for i := 0; i < 100; i++ {
			if !reflect.DeepEqual(work(), want) {
				done <- false
				return
			}
		}
		done <- true
	}(work, want)
	for i := 0; i < 100; i++ {
		tab.state = NewState(uint64(i), 100)
	}
	if !<-done {
		t.Fatal("play decision retained mutable game state")
	}
}
