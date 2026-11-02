package vp8l

import "errors"

// ErrLimit means a decoder allocation would exceed the working-byte budget.
var ErrLimit = errors.New("webp: decoder working-memory limit exceeded")

// Budget conservatively charges cumulative backing-storage allocations. Storage
// is not credited back on return: unreachable temporary buffers can still await
// GC. Caller-owned encoded input and runtime/allocator overhead are not included.
type Budget struct{ remaining int64 }

func NewBudget(n int64) *Budget { return &Budget{remaining: n} }

func (b *Budget) Check(n int64) error {
	if n < 0 || n > b.remaining || uint64(n) > uint64(^uint(0)>>1) {
		return ErrLimit
	}
	return nil
}

func (b *Budget) Reserve(n int64) error {
	if err := b.Check(n); err != nil {
		return err
	}
	b.remaining -= n
	return nil
}
