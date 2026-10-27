package android

import "os"

// JNIClass is the JNI class cmd/misagui registers natives on. Phase A4's
// app.misa.Native must use this name, and must call
// System.loadLibrary("misagui") from that class's static initializer so
// JNI_OnLoad can FindClass it. MISA_JNI_CLASS overrides the name (slash
// form, "app/misa/Native") when the library is loaded.
const JNIClass = "app/misa/Native"

// JNIClassName is JNIClass, or MISA_JNI_CLASS when that is set.
func JNIClassName() string {
	if s := os.Getenv("MISA_JNI_CLASS"); s != "" {
		return s
	}
	return JNIClass
}
