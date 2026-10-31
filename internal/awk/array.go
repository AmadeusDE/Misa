package awk

// array is an associative array that remembers insertion order, so
// `for (k in a)` visits elements in the order they were first created.
type array struct {
	pos  map[string]int
	ents []aent
	dead int
}

type aent struct {
	key  string
	v    Value
	live bool
}

func newArray() *array { return &array{pos: map[string]int{}} }

func (a *array) len() int { return len(a.pos) }

func (a *array) has(k string) bool {
	_, ok := a.pos[k]
	return ok
}

// peek reads an element without creating it.
func (a *array) peek(k string) (Value, bool) {
	i, ok := a.pos[k]
	if !ok {
		return Value{}, false
	}
	return a.ents[i].v, true
}

// get reads an element; referencing a missing one creates it.
func (a *array) get(k string) Value {
	i, ok := a.pos[k]
	if !ok {
		a.set(k, Value{})
		return Value{}
	}
	return a.ents[i].v
}

func (a *array) set(k string, v Value) {
	if i, ok := a.pos[k]; ok {
		a.ents[i].v = v
		return
	}
	a.pos[k] = len(a.ents)
	a.ents = append(a.ents, aent{key: k, v: v, live: true})
}

func (a *array) del(k string) {
	i, ok := a.pos[k]
	if !ok {
		return
	}
	delete(a.pos, k)
	a.ents[i] = aent{}
	a.dead++
	if a.dead > 32 && a.dead > len(a.ents)/2 {
		a.compact()
	}
}

func (a *array) compact() {
	live := a.ents[:0]
	for _, e := range a.ents {
		if e.live {
			a.pos[e.key] = len(live)
			live = append(live, e)
		}
	}
	clear(a.ents[len(live):])
	a.ents, a.dead = live, 0
}

func (a *array) clear() {
	clear(a.pos)
	a.ents, a.dead = nil, 0
}

// keys is a snapshot of the live keys in insertion order.
func (a *array) keys() []string {
	out := make([]string, 0, len(a.pos))
	for _, e := range a.ents {
		if e.live {
			out = append(out, e.key)
		}
	}
	return out
}
