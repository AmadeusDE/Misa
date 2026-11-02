package imageout

import (
	"errors"
	"fmt"
	"sort"

	"github.com/AmadeusDE/Misa/internal/imageframe"
)

func sortedIDs(m map[uint64]bool) []uint64 {
	ids := make([]uint64, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
func (s *Session) ownerIDs() []uint64 {
	m := make(map[uint64]bool, len(s.owners))
	for id := range s.owners {
		m[id] = true
	}
	return sortedIDs(m)
}

// Commit serializes deletes, the complete host text frame, then transfers and
// placements. Acknowledgements establish local complete submission only, not
// display. On any failure End is attempted and the session is poisoned: partial
// output cannot be safely retried or acknowledged without a stronger boundary.
func (s *Session) Commit(text string) ([]imageframe.Acknowledgement, error) {
	if s.failed || s.sink == nil {
		return nil, errors.New("session unavailable")
	}
	ids := s.ownerIDs()
	err := s.sink.Begin()
	if err == nil {
		err = s.commitPhases(ids, text)
		if err != nil {
			s.cleanupFailed(ids)
		}
	}
	endErr := s.sink.End()
	if err == nil && endErr != nil {
		// End may have partially restored state. Re-enter a boundary for a
		// best-effort owner-scoped cleanup and always attempt restoration.
		if s.sink.Begin() == nil {
			s.cleanupFailed(ids)
		}
		_ = s.sink.End()
	}
	if err != nil || endErr != nil {
		s.failed = true
		return nil, errors.Join(err, endErr)
	}
	var acks []imageframe.Acknowledgement
	for _, id := range ids {
		o := s.owners[id]
		clear(o.cleanup)
		if o.pending != nil {
			f := o.pending
			for _, e := range f.Effects {
				if e.Kind == imageframe.Delete && e.Placement == 0 {
					delete(o.resources, e.Resource)
				}
			}
			for _, e := range f.Effects {
				if e.Kind == imageframe.Transfer {
					o.resources[e.Resource] = true
				}
			}
			acks = append(acks, imageframe.Acknowledgement{Owner: id, Generation: f.Generation, Presentation: f.Presentation})
			o.pending = nil
		}
		if o.released {
			delete(s.owners, id)
		}
	}
	return acks, nil
}
func (s *Session) commitPhases(ids []uint64, text string) error {
	for _, id := range ids {
		o := s.owners[id]
		for _, resource := range sortedIDs(o.cleanup) {
			if err := s.sink.Delete(imageframe.Effect{Kind: imageframe.Delete, Owner: id, Resource: resource}); err != nil {
				return fmt.Errorf("delete: %w", err)
			}
		}
		if o.pending != nil {
			for _, e := range o.pending.Effects {
				if e.Kind == imageframe.Delete {
					if err := s.sink.Delete(e); err != nil {
						return err
					}
				}
			}
		}
	}
	var layers []CellUpdate
	for _, id := range ids {
		if f := s.owners[id].pending; f != nil && f.Cells != nil {
			layers = append(layers, CellUpdate{Owner: id, Layer: f.Cells})
		}
	}
	if err := s.sink.Text(text, layers); err != nil {
		return err
	}
	for _, kind := range []imageframe.EffectKind{imageframe.Transfer, imageframe.Place} {
		for _, id := range ids {
			if f := s.owners[id].pending; f != nil {
				for _, e := range f.Effects {
					if e.Kind == kind {
						var err error
						if kind == imageframe.Transfer {
							err = s.sink.Transfer(e)
						} else {
							err = s.sink.Place(e)
						}
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

// cleanupFailed only targets resources that this session may have emitted.
// It is best effort: a failed sink cannot promise that anything was removed.
func (s *Session) cleanupFailed(ids []uint64) {
	for _, id := range ids {
		o := s.owners[id]
		resources := make(map[uint64]bool)
		for r := range o.resources {
			resources[r] = true
		}
		for r := range o.cleanup {
			resources[r] = true
		}
		if o.pending != nil {
			for _, e := range o.pending.Effects {
				if e.Kind == imageframe.Transfer {
					resources[e.Resource] = true
				}
			}
		}
		for _, r := range sortedIDs(resources) {
			_ = s.sink.Delete(imageframe.Effect{Kind: imageframe.Delete, Owner: id, Resource: r})
		}
	}
}
