package imageout

import (
	"errors"

	"github.com/AmadeusDE/Misa/internal/imageframe"
)

// TransactionSink is an injected, exclusively owned output boundary. The host
// must call Commit only at an explicit boundary for the complete text frame;
// neither a View string nor an arbitrary writer supplies such a boundary.
// Begin saves cursor/screen state; End restores it, including after any error
// (even a Begin error). Methods must not retain or mutate their arguments.
// Effect resource IDs are namespaced by Owner, including at the sink.
type TransactionSink interface {
	Begin() error
	Delete(imageframe.Effect) error
	Text(string, []CellUpdate) error
	Transfer(imageframe.Effect) error
	Place(imageframe.Effect) error
	End() error
}

// CellUpdate belongs to an owner in the complete host text transaction. The
// sink composes these layers into that transaction, not into a later write.
type CellUpdate struct {
	Owner uint64
	Layer *imageframe.CellLayer
}

const maxSessionEffects = 1024
const maxSessionBytes = 64 << 20

type sessionOwner struct {
	generation, latest uint64
	accepted           bool
	visible, released  bool
	resources          map[uint64]bool
	cleanup            map[uint64]bool
	pending            *imageframe.Frame
}

// Session is synchronous and not concurrency-safe. The exclusive output owner
// controls all calls. Pending frames and their referenced raster pixels must be
// immutable until replaced, invalidated, or committed; pixel data is not copied.
type Session struct {
	sink            TransactionSink
	owners          map[uint64]*sessionOwner
	limit           int
	generation      uint64
	closing, failed bool
}

// NewSession has no default output; sink must be supplied by the output owner.
// maxOwners bounds registered owners, including those awaiting cleanup.
func NewSession(sink TransactionSink, maxOwners int) *Session {
	return &Session{sink: sink, limit: maxOwners, owners: make(map[uint64]*sessionOwner)}
}
func (s *Session) next() (uint64, error) {
	if s.generation == ^uint64(0) {
		return 0, errors.New("generation exhausted")
	}
	s.generation++
	return s.generation, nil
}
func (s *Session) Register(owner uint64) (uint64, error) {
	if s.failed || s.closing || s.sink == nil || owner == 0 || len(s.owners) >= s.limit || s.owners[owner] != nil {
		return 0, errors.New("cannot register owner")
	}
	g, err := s.next()
	if err != nil {
		return 0, err
	}
	s.owners[owner] = &sessionOwner{generation: g, visible: true, resources: make(map[uint64]bool), cleanup: make(map[uint64]bool)}
	return g, nil
}
func (s *Session) invalidate(o *sessionOwner) (uint64, error) {
	g, err := s.next()
	if err != nil {
		return 0, err
	}
	o.generation = g
	o.accepted = false
	o.pending = nil
	for id := range o.resources {
		o.cleanup[id] = true
		delete(o.resources, id)
	}
	return g, nil
}

// SetVisible invalidates pending work and owned resources on each transition.
func (s *Session) SetVisible(owner uint64, visible bool) (uint64, error) {
	o := s.owners[owner]
	if s.failed || s.closing || o == nil || o.released {
		return 0, errors.New("unknown or unavailable owner")
	}
	if o.visible == visible {
		return o.generation, nil
	}
	g, err := s.invalidate(o)
	if err == nil {
		o.visible = visible
	}
	return g, err
}

// Resize invalidates work and queues owned resources for deletion.
func (s *Session) Resize(owner uint64) (uint64, error) {
	o := s.owners[owner]
	if s.failed || s.closing || o == nil || o.released {
		return 0, errors.New("unknown or unavailable owner")
	}
	return s.invalidate(o)
}

// Release queues only this owner's resource deletes; repeat calls are harmless.
func (s *Session) Release(owner uint64) error {
	o := s.owners[owner]
	if o == nil || o.released {
		return nil
	}
	if s.failed {
		return errors.New("session failed")
	}
	if _, err := s.invalidate(o); err != nil {
		return err
	}
	o.released = true
	return nil
}

// Close queues all owned resource deletes for the next explicit Commit.
func (s *Session) Close() error {
	if s.closing {
		return nil
	}
	if s.failed {
		return errors.New("session failed")
	}
	for id := range s.owners {
		if err := s.Release(id); err != nil {
			return err
		}
	}
	s.closing = true
	return nil
}

// Submit replaces the owner's pending frame. Frame.Generation must match the
// current owner generation; Presentation.Generation must strictly increase
// within that generation, including across successful commits. Effects and cell
// slices are copied, but immutable referenced payloads are shared. Ownership is validated
// against committed resources and transfers in this frame.
func (s *Session) Submit(f imageframe.Frame) error {
	o := s.owners[f.Owner]
	if s.failed || s.closing || o == nil || o.released || !o.visible || f.Generation != o.generation || (o.accepted && f.Presentation.Generation <= o.latest) {
		return errors.New("stale or unavailable owner")
	}
	if len(f.Effects) > maxSessionEffects {
		return errors.New("too many effects")
	}
	known := make(map[uint64]bool, len(o.resources))
	for id := range o.resources {
		known[id] = true
	}
	for _, e := range f.Effects {
		if e.Owner != f.Owner || e.Resource == 0 || e.Kind < imageframe.Transfer || e.Kind > imageframe.Delete {
			return errors.New("invalid effect")
		}
		if e.Raster != nil && e.Raster.ID != e.Resource {
			return errors.New("raster resource mismatch")
		}
		if e.PlacementData != nil && (e.PlacementData.ID != e.Placement || (e.PlacementData.Raster != nil && e.PlacementData.Raster.ID != e.Resource)) {
			return errors.New("placement resource mismatch")
		}
		if e.Kind == imageframe.Delete && !known[e.Resource] {
			return errors.New("delete of unowned resource")
		}
		if e.Kind == imageframe.Delete && e.Placement == 0 {
			delete(known, e.Resource)
		}
	}
	bytes := 0
	for _, e := range f.Effects {
		if e.Kind == imageframe.Transfer {
			known[e.Resource] = true
			if e.Raster != nil && e.Raster.Pixels != nil {
				n := len(e.Raster.Pixels.Pix)
				if n > maxSessionBytes-bytes {
					return errors.New("frame too large")
				}
				bytes += n
			}
		}
	}
	if len(known) > maxSessionEffects {
		return errors.New("too many resources")
	}
	for _, e := range f.Effects {
		if e.Kind == imageframe.Place && !known[e.Resource] {
			return errors.New("placement of unowned resource")
		}
	}
	f.Effects = append([]imageframe.Effect(nil), f.Effects...)
	if f.Cells != nil {
		if len(f.Cells.Cells) > maxSessionEffects*1024 {
			return errors.New("cell layer too large")
		}
		for _, cell := range f.Cells.Cells {
			if len(cell.Text) > maxSessionBytes-bytes {
				return errors.New("frame too large")
			}
			bytes += len(cell.Text)
		}
		c := *f.Cells
		c.Cells = append([]imageframe.Cell(nil), c.Cells...)
		f.Cells = &c
	}
	o.pending = &f
	o.latest = f.Presentation.Generation
	o.accepted = true
	return nil
}
