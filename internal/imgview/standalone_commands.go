package imgview

import (
	"context"
	"reflect"

	tea "charm.land/bubbletea/v2"
)

type standalonePanic struct{ value any }

// standaloneCommands executes commands off the event loop; sends are canceled
// on shutdown, including those from long-running clock and decoder commands.
func standaloneCommands(ctx context.Context, ch chan<- tea.Msg, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		defer func() {
			if p := recover(); p != nil {
				select {
				case ch <- standalonePanic{p}:
				case <-ctx.Done():
				}
			}
		}()
		var execute func(tea.Cmd)
		var deliver func(tea.Msg)
		deliver = func(msg tea.Msg) {
			if msg == nil {
				return
			}
			if batch, ok := msg.(tea.BatchMsg); ok {
				for _, c := range batch {
					standaloneCommands(ctx, ch, c)
				}
				return
			}
			// Bubble Tea's sequenceMsg is unexported. Identify its exact package/type,
			// then iterate the exported Cmd values without unsafe reflection.
			t := reflect.TypeOf(msg)
			if t.PkgPath() == "charm.land/bubbletea/v2" && t.Name() == "sequenceMsg" {
				v := reflect.ValueOf(msg)
				for i := 0; i < v.Len(); i++ {
					execute(v.Index(i).Interface().(tea.Cmd))
				}
				return
			}
			select {
			case ch <- msg:
			case <-ctx.Done():
			}
		}
		execute = func(c tea.Cmd) {
			if c != nil && ctx.Err() == nil {
				deliver(c())
			}
		}
		execute(cmd)
	}()
}
