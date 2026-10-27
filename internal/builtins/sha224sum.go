// Behaviour adapted from toybox md5sum.c (0BSD) and uutils coreutils sha224sum (MIT).
package builtins

import "crypto/sha256"

func init() {
	Register(Command{Name: "sha224sum", Run: makeChecksumRunner("sha224sum", sha256.New224)})
}
