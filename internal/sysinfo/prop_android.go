//go:build android && cgo

package sysinfo

/*
#include <stdlib.h>
#include <sys/system_properties.h>
*/
import "C"
import "unsafe"

// systemProperty reads a property through bionic. It is the only cgo in this
// module: import "C" stays behind android && cgo.
func systemProperty(name string) string {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	buf := make([]byte, C.PROP_VALUE_MAX)
	n := int(C.__system_property_get(cname, (*C.char)(unsafe.Pointer(&buf[0]))))
	if n <= 0 {
		return ""
	}
	if n > len(buf) {
		n = len(buf)
	}
	return string(buf[:n])
}
