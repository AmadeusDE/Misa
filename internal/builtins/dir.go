// dir and vdir are ls output-mode aliases: dir is "ls -C -b" and vdir is
// "ls -l -b", defined (like the format and quoting defaults) after
// uutils coreutils (MIT license) src/uu/dir/src/dir.rs and
// src/uu/vdir/src/vdir.rs, pinned at commit
// fee61ff5d4f87e0f912a52e639942b5f7ad58663. They are thin wrappers over the
// ls implementation, which keeps its own defaults.
//
// Options are ls's (a superset of the reference); unsupported options fail
// with "dir: unknown option ..." and exit status 2, exactly like ls. Two
// deliberate differences from the reference aliases: Misa's -C is a tab
// grid rather than a space-padded one, and -Q still works as ls's extra
// quoting pass on top of the escape quoting these aliases default to.
//
// See docs/CREDITS.md and THIRD_PARTY_NOTICES.md.

package builtins

func init() {
	Register(Command{Name: "dir", Run: runDir})
	Register(Command{Name: "vdir", Run: runVdir})
}

func runDir(ctx *Context) int { return runLsMode(ctx, lsModeDir) }

func runVdir(ctx *Context) int { return runLsMode(ctx, lsModeVdir) }
