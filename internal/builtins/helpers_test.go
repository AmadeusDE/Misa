package builtins

import "runtime"

// exeSuffix is the extension a test executable needs to be found on PATH.
var exeSuffix = map[bool]string{true: ".exe", false: ""}[runtime.GOOS == "windows"]
