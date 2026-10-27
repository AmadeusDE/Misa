// Behaviour adapted from toybox md5sum.c (0BSD) and uutils coreutils sha384sum (MIT).
package builtins

import "crypto/sha512"

func init() {
	Register(Command{Name: "sha384sum", Run: makeChecksumRunner("sha384sum", sha512.New384)})
}
