package term

import (
	"reflect"
	"testing"
)

func TestOwnershipCleanupAndTransfer(t *testing.T) {
	var seen []int
	var o ownership
	o.add(func() { seen = append(seen, 1) })
	transferred := o.add(func() { seen = append(seen, 2) })
	o.add(func() { seen = append(seen, 3) })
	o.release(transferred)
	o.close()
	o.close()
	if !reflect.DeepEqual(seen, []int{3, 1}) {
		t.Fatalf("cleanup order: %v", seen)
	}
}
