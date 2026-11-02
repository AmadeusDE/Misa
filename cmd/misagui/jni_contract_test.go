package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// These tests are source contracts: cmd/misagui builds only for
// android && cgo, so the JNI side (Java declarations, C trampolines, Go
// exports) is checked by reading the files that must agree.
const (
	nativeJava   = "../../android/app/src/main/java/app/misa/Native.java"
	activityJava = "../../android/app/src/main/java/app/misa/MisaActivity.java"
	jniGo        = "jni.go"
	jniC         = "jni_onload.c"
)

func readSource(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestNativeDeclarationsMatchRegisterNatives keeps app.misa.Native and the
// RegisterNatives table in jni_onload.c in step: a missing or mistyped
// descriptor makes the JVM fail the whole registration.
func TestNativeDeclarationsMatchRegisterNatives(t *testing.T) {
	java := javaNatives(t, readSource(t, nativeJava))
	c := registeredNatives(t, readSource(t, jniC))
	if len(java) == 0 {
		t.Fatal("Native.java declares no native methods")
	}
	for name, want := range java {
		got, ok := c[name]
		if !ok {
			t.Errorf("%s is not registered in %s", name, jniC)
			continue
		}
		if got != want {
			t.Errorf("%s registered as %q, Java declares %q", name, got, want)
		}
	}
	for name := range c {
		if _, ok := java[name]; !ok {
			t.Errorf("%s is registered but not declared in Native.java", name)
		}
	}
}

// TestTrampolinesAndGoExportsMatch follows each registration through the C
// trampoline to the exported Go function and the Manager method it calls.
func TestTrampolinesAndGoExportsMatch(t *testing.T) {
	c := readSource(t, jniC)
	goSrc := readSource(t, jniGo)
	registered := registeredNatives(t, c)
	if len(registered) == 0 {
		t.Fatal("no RegisterNatives entries")
	}
	for name := range registered {
		export, method := goNames(name)
		if !strings.Contains(c, "jni_"+name+"(JNIEnv *env, jclass cls") {
			t.Errorf("no trampoline jni_%s in %s", name, jniC)
		}
		if !strings.Contains(c, export+"(") {
			t.Errorf("trampoline for %s does not call %s", name, export)
		}
		if !strings.Contains(goSrc, "//export "+export+"\n") {
			t.Errorf("%s has no //export %s", jniGo, export)
		}
		if !strings.Contains(goSrc, "android.Default."+method+"(") {
			t.Errorf("%s does not call android.Default.%s", jniGo, method)
		}
	}
}

// TestSessionIdsTakesOneSnapshot pins the approved marshaling: the Go side
// copies one snapshot into a malloc'd buffer, so ids cannot shift between a
// count call and a fill call.
func TestSessionIdsTakesOneSnapshot(t *testing.T) {
	body := cFunction(t, readSource(t, jniC), "static jintArray jni_sessionIds(")
	if n := strings.Count(body, "misaSessionIDs("); n != 1 {
		t.Fatalf("sessionIds calls misaSessionIDs %d times, want one snapshot: %s", n, body)
	}
	if !strings.Contains(body, "free(ids)") {
		t.Fatalf("sessionIds does not free the Go buffer: %s", body)
	}
	if !strings.Contains(body, "SetIntArrayRegion") {
		t.Fatalf("sessionIds does not copy the snapshot into the Java array: %s", body)
	}
	goSrc := readSource(t, jniGo)
	export := cFunction(t, goSrc, "func misaSessionIDs(")
	if !strings.Contains(export, "android.Default.SessionIDs()") || !strings.Contains(export, "C.malloc(") {
		t.Fatalf("misaSessionIDs does not copy one snapshot into C memory: %s", export)
	}
}

// cFunction returns a C function or Go func from its signature to its closing
// brace in column zero.
func cFunction(t *testing.T, src, signature string) string {
	t.Helper()
	i := strings.Index(src, signature)
	if i < 0 {
		t.Fatalf("no %s", signature)
	}
	rest := src[i:]
	if end := strings.Index(rest, "\n}\n"); end >= 0 {
		return rest[:end]
	}
	t.Fatalf("%s is not closed", signature)
	return ""
}

// goNames maps a Java native method to its Go export and Manager method:
// misa<Name> and <Name>, except where the Go side spells it differently.
func goNames(java string) (export, method string) {
	switch java {
	case "onSurfaceCreated":
		return "misaOnSurface", "OnSurfaceCreated"
	case "sessionIds":
		return "misaSessionIDs", "SessionIDs"
	}
	camel := strings.ToUpper(java[:1]) + java[1:]
	return "misa" + camel, camel
}

// TestActivityPropagatesVisibilityAndFocus pins the lifecycle contract:
// onStart/onStop carry visibility, window focus is forwarded as it happens
// (never faked on start), and the calls reach the Manager through JNI.
func TestActivityPropagatesVisibilityAndFocus(t *testing.T) {
	src := readSource(t, activityJava)
	start := javaMethod(t, src, "protected void onStart()")
	stop := javaMethod(t, src, "protected void onStop()")
	focus := javaMethod(t, src, "public void onWindowFocusChanged(boolean hasFocus)")

	if !strings.Contains(start, "Native.onVisibility(true)") {
		t.Fatalf("onStart does not report visibility: %s", start)
	}
	if strings.Contains(start, "Native.onFocus") {
		t.Fatalf("onStart fakes a focus event: %s", start)
	}
	if !strings.Contains(stop, "Native.onVisibility(false)") {
		t.Fatalf("onStop does not report visibility: %s", stop)
	}
	if !strings.Contains(focus, "Native.onFocus(hasFocus)") {
		t.Fatalf("onWindowFocusChanged does not forward the real focus: %s", focus)
	}
	if strings.Contains(focus, "Native.onFocus(true)") || strings.Contains(focus, "Native.onFocus(false)") {
		t.Fatalf("onWindowFocusChanged invents a focus state: %s", focus)
	}
	if !strings.Contains(readSource(t, nativeJava), "native void onVisibility(boolean visible);") ||
		!strings.Contains(readSource(t, nativeJava), "native void onFocus(boolean hasFocus);") {
		t.Fatal("Native.java is missing the lifecycle declarations")
	}
}

// TestSessionReconciliationUsesReadOnlyIDs keeps the tab list off
// switchSession: probing a session makes it current, which would focus and
// resume a hidden tab.
func TestSessionReconciliationUsesReadOnlyIDs(t *testing.T) {
	src := readSource(t, activityJava)
	body := javaMethod(t, src, "private void reconcileSessions()")
	if !strings.Contains(body, "Native.sessionIds()") {
		t.Fatalf("reconcileSessions does not read the session list: %s", body)
	}
	if strings.Contains(body, "Native.switchSession") {
		t.Fatalf("reconcileSessions probes sessions with switchSession: %s", body)
	}
	if !strings.Contains(readSource(t, nativeJava), "native int[] sessionIds();") {
		t.Fatal("Native.java is missing sessionIds")
	}
}

// javaNatives maps each declared native method to its JNI descriptor.
func javaNatives(t *testing.T, src string) map[string]string {
	t.Helper()
	decl := regexp.MustCompile(`(?s)native\s+([\w.\[\]]+)\s+(\w+)\s*\(([^)]*)\)\s*;`)
	out := map[string]string{}
	for _, m := range decl.FindAllStringSubmatch(src, -1) {
		out[m[2]] = descriptor(t, m[1], m[3])
	}
	return out
}

func descriptor(t *testing.T, ret, params string) string {
	t.Helper()
	var b strings.Builder
	b.WriteByte('(')
	for _, param := range strings.Split(params, ",") {
		param = strings.TrimSpace(param)
		if param == "" {
			continue
		}
		fields := strings.Fields(param)
		if len(fields) < 2 {
			t.Fatalf("cannot parse parameter %q", param)
		}
		b.WriteString(jniType(t, strings.Join(fields[:len(fields)-1], " ")))
	}
	b.WriteByte(')')
	b.WriteString(jniType(t, ret))
	return b.String()
}

func jniType(t *testing.T, java string) string {
	t.Helper()
	types := map[string]string{
		"void":     "V",
		"boolean":  "Z",
		"int":      "I",
		"float":    "F",
		"String":   "Ljava/lang/String;",
		"String[]": "[Ljava/lang/String;",
		"int[]":    "[I",
		"Surface":  "Landroid/view/Surface;",
	}
	desc, ok := types[java]
	if !ok {
		t.Fatalf("no JNI descriptor for Java type %q", java)
	}
	return desc
}

// registeredNatives maps each RegisterNatives entry to its descriptor.
func registeredNatives(t *testing.T, src string) map[string]string {
	t.Helper()
	entry := regexp.MustCompile(`\{"(\w+)", "([^"]+)", \(void \*\)jni_(\w+)\}`)
	out := map[string]string{}
	for _, m := range entry.FindAllStringSubmatch(src, -1) {
		if m[1] != m[3] {
			t.Fatalf("registration %s uses trampoline jni_%s", m[1], m[3])
		}
		out[m[1]] = m[2]
	}
	return out
}

// javaMethod returns the body of one method; Java methods here are indented
// four spaces and close with a four-space brace.
func javaMethod(t *testing.T, src, header string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(header) + `\s*\{(.*?)\n    \}`).FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("no %s in MisaActivity.java", header)
	}
	return m[1]
}

func TestActivityPollRunsOnlyWhileVisible(t *testing.T) {
	src := readSource(t, activityJava)
	create := javaMethod(t, src, "protected void onCreate(Bundle savedInstanceState)")
	start := javaMethod(t, src, "protected void onStart()")
	stop := javaMethod(t, src, "protected void onStop()")
	if strings.Contains(create, "handler.post(tick)") {
		t.Fatal("onCreate arms a poll before the Activity is visible")
	}
	if !strings.Contains(start, "tick.run()") {
		t.Fatal("onStart must immediately refresh labels and arm the next visible poll")
	}
	if !strings.Contains(stop, "handler.removeCallbacks(tick)") {
		t.Fatal("onStop leaves the UI label poll running in the background")
	}
}
