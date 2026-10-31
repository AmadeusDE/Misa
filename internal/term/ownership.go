package term

// ownership tracks startup resources until they are transferred to their owner.
// Startup is single-threaded; close is idempotent and unwinds in reverse order.
type ownership struct{ cleanup []func() }

func (o *ownership) add(f func()) int {
	o.cleanup = append(o.cleanup, f)
	return len(o.cleanup) - 1
}
func (o *ownership) release(i int) { o.cleanup[i] = nil }
func (o *ownership) close() {
	for i := len(o.cleanup) - 1; i >= 0; i-- {
		if f := o.cleanup[i]; f != nil {
			o.cleanup[i] = nil
			f()
		}
	}
}
