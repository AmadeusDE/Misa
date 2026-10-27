//go:build android && cgo

// JNI_OnLoad registers the Go entry points on app/misa/Native (or
// MISA_JNI_CLASS). The class path and the Java declarations live in doc.go.
// Trampolines convert JNI types and call the exported Go functions so the
// JVM never has to match a cgo calling convention itself.

#include "_cgo_export.h"
#include <android/log.h>
#include <android/native_window_jni.h>
#include <jni.h>
#include <stdlib.h>
#include <string.h>

static const char *const kTag = "misagui";

static void jni_onCreate(JNIEnv *env, jclass cls, jobjectArray vars, jstring exe) {
    (void)cls;
    const char *exePath = "";
    const char *owned = NULL;
    if (exe) {
        owned = (*env)->GetStringUTFChars(env, exe, NULL);
        if (owned) {
            exePath = owned;
        }
    }
    jsize n = vars ? (*env)->GetArrayLength(env, vars) : 0;
    char **arr = NULL;
    if (n > 0) {
        arr = calloc((size_t)n, sizeof(char *));
        if (arr) {
            for (jsize i = 0; i < n; i++) {
                jstring js = (*env)->GetObjectArrayElement(env, vars, i);
                arr[i] = strdup("");
                if (!js) {
                    continue;
                }
                const char *s = (*env)->GetStringUTFChars(env, js, NULL);
                if (s) {
                    free(arr[i]);
                    arr[i] = strdup(s);
                    (*env)->ReleaseStringUTFChars(env, js, s);
                }
                (*env)->DeleteLocalRef(env, js);
            }
        } else {
            n = 0;
        }
    }
    char *err = misaOnCreate((char *)exePath, arr, (int)n);
    if (err) {
        __android_log_print(ANDROID_LOG_ERROR, kTag, "onCreate: %s", err);
        free(err);
    }
    for (jsize i = 0; i < n; i++) {
        free(arr[i]);
    }
    free(arr);
    if (owned) {
        (*env)->ReleaseStringUTFChars(env, exe, owned);
    }
}

static void jni_onSurfaceCreated(JNIEnv *env, jclass cls, jobject surface) {
    (void)cls;
    ANativeWindow *win = NULL;
    if (surface) {
        win = ANativeWindow_fromSurface(env, surface);
    }
    misaOnSurface(win);
}

static void jni_onSurfaceChanged(JNIEnv *env, jclass cls, jint w, jint h, jfloat density,
    jint left, jint top, jint right, jint bottom) {
    (void)env;
    (void)cls;
    misaOnSurfaceChanged((int)w, (int)h, (float)density, (int)left, (int)top, (int)right, (int)bottom);
}

static void jni_onSurfaceDestroyed(JNIEnv *env, jclass cls) {
    (void)env;
    (void)cls;
    misaOnSurfaceDestroyed();
}

static void jni_onText(JNIEnv *env, jclass cls, jstring text) {
    (void)cls;
    const char *s = "";
    const char *owned = NULL;
    if (text) {
        owned = (*env)->GetStringUTFChars(env, text, NULL);
        if (owned) {
            s = owned;
        }
    }
    misaOnText((char *)s);
    if (owned) {
        (*env)->ReleaseStringUTFChars(env, text, owned);
    }
}

static void jni_onKey(JNIEnv *env, jclass cls, jint code, jint meta, jboolean down) {
    (void)env;
    (void)cls;
    misaOnKey((int)code, (int)meta, down ? 1 : 0);
}

static void jni_onMotion(JNIEnv *env, jclass cls, jint action, jfloat x, jfloat y, jint buttons) {
    (void)env;
    (void)cls;
    misaOnMotion((int)action, (float)x, (float)y, (int)buttons);
}

static void jni_onPinch(JNIEnv *env, jclass cls, jfloat scale) {
    (void)env;
    (void)cls;
    misaOnPinch((float)scale);
}

static void jni_onLowMemory(JNIEnv *env, jclass cls) {
    (void)env;
    (void)cls;
    misaOnLowMemory();
}

static jint jni_newSession(JNIEnv *env, jclass cls) {
    (void)env;
    (void)cls;
    return (jint)misaNewSession();
}

static void jni_closeSession(JNIEnv *env, jclass cls, jint id) {
    (void)env;
    (void)cls;
    misaCloseSession((int)id);
}

static void jni_switchSession(JNIEnv *env, jclass cls, jint id) {
    (void)env;
    (void)cls;
    misaSwitchSession((int)id);
}

static jint jni_activeSession(JNIEnv *env, jclass cls) {
    (void)env;
    (void)cls;
    return (jint)misaActiveSession();
}

static jint jni_sessionCount(JNIEnv *env, jclass cls) {
    (void)env;
    (void)cls;
    return (jint)misaSessionCount();
}

jint JNI_OnLoad(JavaVM *vm, void *reserved) {
    (void)reserved;
    JNIEnv *env = NULL;
    if ((*vm)->GetEnv(vm, (void **)&env, JNI_VERSION_1_6) != JNI_OK || !env) {
        return JNI_ERR;
    }
    char *name = misaJNIClass();
    if (!name) {
        return JNI_ERR;
    }
    jclass cls = (*env)->FindClass(env, name);
    if (!cls) {
        (*env)->ExceptionClear(env);
        __android_log_print(ANDROID_LOG_ERROR, kTag, "FindClass %s failed", name);
        free(name);
        return JNI_ERR;
    }
    JNINativeMethod methods[] = {
        {"onCreate", "([Ljava/lang/String;Ljava/lang/String;)V", (void *)jni_onCreate},
        {"onSurfaceCreated", "(Landroid/view/Surface;)V", (void *)jni_onSurfaceCreated},
        {"onSurfaceChanged", "(IIFIIII)V", (void *)jni_onSurfaceChanged},
        {"onSurfaceDestroyed", "()V", (void *)jni_onSurfaceDestroyed},
        {"onText", "(Ljava/lang/String;)V", (void *)jni_onText},
        {"onKey", "(IIZ)V", (void *)jni_onKey},
        {"onMotion", "(IFFI)V", (void *)jni_onMotion},
        {"onPinch", "(F)V", (void *)jni_onPinch},
        {"onLowMemory", "()V", (void *)jni_onLowMemory},
        {"newSession", "()I", (void *)jni_newSession},
        {"closeSession", "(I)V", (void *)jni_closeSession},
        {"switchSession", "(I)V", (void *)jni_switchSession},
        {"activeSession", "()I", (void *)jni_activeSession},
        {"sessionCount", "()I", (void *)jni_sessionCount},
    };
    if ((*env)->RegisterNatives(env, cls, methods, sizeof methods / sizeof methods[0]) != 0) {
        (*env)->ExceptionClear(env);
        __android_log_print(ANDROID_LOG_ERROR, kTag, "RegisterNatives %s failed", name);
        free(name);
        return JNI_ERR;
    }
    free(name);
    return JNI_VERSION_1_6;
}
