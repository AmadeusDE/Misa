// Package android is the Android window backend for the Misa GUI.
//
// Geometry, input translation, damage blit and the session manager are pure
// Go, so their tests run on Linux and in the android-device emulator job.
// Window presentation (ANativeWindow RGBA_8888) is built only with
// android && cgo. cmd/misagui loads this package into libmisagui.so and
// feeds it from JNI.
//
// misa --gui on Android still reports that there is no Wayland window. The
// APK is the GUI: several terminal sessions, each a child libmisa.so, survive
// surface loss and rotation.
package android
