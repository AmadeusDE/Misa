// Behaviour adapted from toybox uuidgen.c (0BSD) and uutils util-linux uuidgen (MIT).
package builtins

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func init() { Register(Command{Name: "uuidgen", Run: runUUIDGen}) }

var uuidNamespaces = map[string]string{"@dns": "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "@url": "6ba7b811-9dad-11d1-80b4-00c04fd430c8", "@oid": "6ba7b812-9dad-11d1-80b4-00c04fd430c8", "@x500": "6ba7b814-9dad-11d1-80b4-00c04fd430c8"}

func runUUIDGen(ctx *Context) int {
	mode := "random"
	modeSet := false
	ns, name := "", ""
	hasNS, hasName := false, false
	for i := 0; i < len(ctx.Args); i++ {
		a := ctx.Args[i]
		switch a {
		case "-r", "--random", "-t", "--time", "-m", "--md5", "-s", "--sha1":
			if modeSet {
				return uuidErr(ctx, "conflicting options")
			}
			modeSet = true
			switch a {
			case "-t", "--time":
				mode = "time"
			case "-m", "--md5":
				mode = "md5"
			case "-s", "--sha1":
				mode = "sha1"
			default:
				mode = "random"
			}
		case "--namespace":
			i++
			if i == len(ctx.Args) {
				return uuidErr(ctx, "missing namespace")
			}
			ns = ctx.Args[i]
			hasNS = true
		case "--name":
			i++
			if i == len(ctx.Args) {
				return uuidErr(ctx, "missing name")
			}
			name = ctx.Args[i]
			hasName = true
		case "--help":
			fmt.Fprintln(ctx.Stdout, "Usage: uuidgen [options]\nGenerate a UUID.")
			return 0
		case "--version":
			fmt.Fprintln(ctx.Stdout, "uuidgen (Misa) 0.1.0")
			return 0
		default:
			return uuidErr(ctx, "invalid option "+a)
		}
	}
	if (mode == "md5" || mode == "sha1") != (hasNS && hasName) || hasNS != hasName {
		return uuidErr(ctx, "namespace and name required for name-based UUIDs")
	}
	var b []byte
	if mode == "md5" || mode == "sha1" {
		base, ok := uuidNamespaces[ns]
		if !ok {
			return uuidErr(ctx, "invalid namespace")
		}
		raw, _ := hex.DecodeString(strings.ReplaceAll(base, "-", ""))
		data := append(raw, []byte(name)...)
		if mode == "md5" {
			x := md5.Sum(data)
			b = x[:]
		} else {
			x := sha1.Sum(data)
			b = x[:16]
		}
	} else {
		b = make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return uuidErr(ctx, err.Error())
		}
	}
	if mode == "time" {
		t := time.Now().UnixNano()/100 + 0x01b21dd213814000
		for i := 0; i < 8; i++ {
			b[i] = byte(t >> (8 * i))
		}
		b[8] = 0x80 | b[8]&0x3f
	}
	if mode == "random" {
		b[6] = (b[6] & 0x0f) | 0x40
	}
	if mode == "time" {
		b[6] = (b[6] & 0x0f) | 0x10
	}
	if mode == "md5" {
		b[6] = (b[6] & 0x0f) | 0x30
	}
	if mode == "sha1" {
		b[6] = (b[6] & 0x0f) | 0x50
	}
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b)
	fmt.Fprintf(ctx.Stdout, "%s-%s-%s-%s-%s\n", s[:8], s[8:12], s[12:16], s[16:20], s[20:])
	return 0
}
func uuidErr(ctx *Context, s string) int { fmt.Fprintf(ctx.Stderr, "uuidgen: %s\n", s); return 1 }
