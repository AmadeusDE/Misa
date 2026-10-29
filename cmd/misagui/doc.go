// Command misagui is libmisagui.so, the Android GUI as a c-shared library.
//
// Phase A4's Java class is app.misa.Native. JNI_OnLoad (jni_onload.c)
// RegisterNatives on that class, or on MISA_JNI_CLASS when that environment
// variable is set (slash form, for example "app/misa/Native"). The class
// must load the library from its own static initializer so FindClass sees
// the application class loader:
//
//	package app.misa;
//
//	import android.view.Surface;
//
//	public final class Native {
//	    static { System.loadLibrary("misagui"); }
//
//	    public static native void onCreate(String[] env, String exePath);
//	    public static native void onSurfaceCreated(Surface surface);
//	    public static native void onSurfaceChanged(int w, int h, float density,
//	        int insetLeft, int insetTop, int insetRight, int insetBottom);
//	    public static native void onSurfaceDestroyed();
//	    public static native void onText(String text, int meta);
//	    public static native void muxAction(String name);
//	    public static native void onKey(int code, int meta, boolean down);
//	    public static native void onMotion(int action, float x, float y, int buttonState);
//	    public static native void onPinch(float scale);
//	    public static native void onLowMemory();
//	    public static native int newSession();
//	    public static native void closeSession(int id);
//	    public static native void switchSession(int id);
//	    public static native int activeSession();
//	    public static native int sessionCount();
//	}
//
// onMotion: for ACTION_SCROLL (8), x and y are wheel steps (positive down
// and to the right). Every other action passes view pixels and the
// MotionEvent button state. onPinch's scale is a factor on the current
// point size. onText's meta is the latched Ctrl/Alt state (0 sends the
// commit as text). muxAction names a multiplexer button (internal/gui/android
// MuxChords). onCreate's env entries are KEY=VALUE; exePath is libmisa.so.
//
// Build (API 26, the APK minSdk; -tags nough because the agent runs in the
// child): ANDROID_NDK_HOME=... just android-lib
package main
