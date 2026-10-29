package app.misa;

import android.view.Surface;

// JNI methods registered by JNI_OnLoad in cmd/misagui/jni_onload.c.
// The library loads from this class's static initializer so FindClass
// ("app/misa/Native") sees the application class loader.
public final class Native {
    static {
        System.loadLibrary("misagui");
    }

    private Native() {}

    public static native void onCreate(String[] env, String exePath);

    public static native void onSurfaceCreated(Surface surface);

    public static native void onSurfaceChanged(
            int w, int h, float density, int insetLeft, int insetTop, int insetRight, int insetBottom);

    public static native void onSurfaceDestroyed();

    public static native void onText(String text, int meta);

    public static native void muxAction(String name);

    public static native void onKey(int code, int meta, boolean down);

    public static native void onMotion(int action, float x, float y, int buttonState);

    public static native void onPinch(float scale);

    public static native void onLowMemory();

    public static native int newSession();

    public static native void closeSession(int id);

    public static native void switchSession(int id);

    public static native int activeSession();

    public static native int sessionCount();
}
