package native

import "testing"

func TestUname_Basic(t *testing.T) {
	r := run(t, "uname")
	wantSuccess(t, r)
	wantOutContains(t, r, "Linux")
}

func TestUname_AllFlag(t *testing.T) {
	r := run(t, "uname", "-a")
	wantSuccess(t, r)
	wantOutContains(t, r, "Linux")
	wantOutContains(t, r, "GNU/Linux")
}

func TestUname_Processor(t *testing.T) {
	r := run(t, "uname", "-p")
	wantSuccess(t, r)
}

func TestUname_AllLabeled(t *testing.T) {
	r := run(t, "uname", "-A")
	wantSuccess(t, r)
	wantOutContains(t, r, "Kernel name: ")
	wantOutContains(t, r, "Operating system: ")
}
