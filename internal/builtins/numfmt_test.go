// Behaviour adapted from uutils coreutils numfmt (MIT).
package builtins

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestNumfmt(t *testing.T) {
	cases := []struct {
		name                      string
		args                      []string
		input, output, diagnostic string
		code                      int
	}{
		{"exact", []string{"--from=iec", "9153396227555392131"}, "", "9153396227555392131\n", "", 0},
		{"missing newline", []string{"--from=auto"}, "1K\n1Ki", "1000\n1024", "", 0},
		{"round default", []string{"--to=si", "--", "9001", "-9099"}, "", "9.1k\n-9.1k\n", "", 0},
		{"precision", []string{"--format=%.1f", "1.01"}, "", "1.1\n", "", 0},
		{"zero sign", []string{"--format=%018.2f", "--", "-9869647"}, "", "-00000009869647.00\n", "", 0},
		{"fields", []string{"--from=auto", "--field=-2,4"}, "1K 2K 3K 4K", "1000 2000 3K 4000", "", 0},
		{"warn", []string{"--invalid=warn"}, "1\nFoo\n2", "1\nFoo\n2", "numfmt: invalid number: 'Foo'\n", 0},
		{"partial abort", []string{"--field=3", "Hello 40M World 90G"}, "", "Hello 40M ", "numfmt: invalid number: 'World'\n", 2},
		{"zero delimiter", []string{"-z", "--to=si"}, "1000\x002000", "1.0k\x002.0k", "", 0},
		{"fraction units", []string{"--from-unit=959", "--", "-615484.454"}, "", "-590249591.386\n", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, err bytes.Buffer
			code := runNumfmt(&Context{Args: tc.args, Stdin: strings.NewReader(tc.input), Stdout: &out, Stderr: &err, Env: []string{}})
			if code != tc.code || out.String() != tc.output || err.String() != tc.diagnostic {
				t.Fatalf("code=%d stdout=%q stderr=%q; want %d %q %q", code, out.String(), err.String(), tc.code, tc.output, tc.diagnostic)
			}
		})
	}
}

func TestNumfmtRoundingMethods(t *testing.T) {
	for _, tc := range []struct{ method, expected string }{
		{"from-zero", "9.1k\n-9.1k\n9.1k\n-9.1k\n"},
		{"from-zer", "9.1k\n-9.1k\n9.1k\n-9.1k\n"},
		{"f", "9.1k\n-9.1k\n9.1k\n-9.1k\n"},
		{"towards-zero", "9.0k\n-9.0k\n9.0k\n-9.0k\n"},
		{"up", "9.1k\n-9.0k\n9.1k\n-9.0k\n"},
		{"down", "9.0k\n-9.1k\n9.0k\n-9.1k\n"},
		{"nearest", "9.0k\n-9.0k\n9.1k\n-9.1k\n"},
		{"near", "9.0k\n-9.0k\n9.1k\n-9.1k\n"},
	} {
		t.Run(tc.method, func(t *testing.T) {
			r := runBuiltin(t, "numfmt", "--to=si", "--round="+tc.method, "--", "9001", "-9001", "9099", "-9099")
			assertExitCode(t, r, 0)
			assertStdout(t, r, tc.expected)
		})
	}
}

func TestNumfmtUpstreamLoops(t *testing.T) {
	cases := []struct {
		name                      string
		args                      []string
		input, output, diagnostic string
		code                      int
	}{
		{"zero width", []string{"--format=%0 6f", "1234"}, "", "001234\n", "", 0},
		{"zero unit", []string{"--format=%05.1f", "--to=si", "--suffix=B", "1234567"}, "", "001.3MB\n", "", 0},
		{"zero width outer padding", []string{"--format=%08.1f", "--to=si", "--padding=12", "1234567"}, "", "   000001.3M\n", "", 0},
		{"explicit precision", []string{"--format=%.1f", "0.99", "1", "1.01"}, "", "1.0\n1.0\n1.1\n", "", 0},
		{"precision down", []string{"--format=%.1f", "--round=down", "0.99", "1", "1.01"}, "", "0.9\n1.0\n1.0\n", "", 0},
		{"round unitless", []string{"--to=si", "--", "0.4", "0.5", "0.6", "1.4", "3.14", "-0.4", "-0.5", "-0.6", "-1.4"}, "", "0\n0\n1\n1\n3\n-0\n-0\n-1\n-1\n", "", 0},
		{"suffix only units", []string{"--from-unit=Ki", "--to-unit=K", "1000"}, "", "1024\n", "", 0},
		{"to unit", []string{"--to-unit=1024", "--", "6000", "-6000", "6000.0", "-6000.0", "6000.00", "-6000.00"}, "", "6\n-6\n5.9\n-5.9\n5.86\n-5.86\n", "", 0},
		{"precision carry", []string{"--to=si", "--format=%.0f", "999999"}, "", "1M\n", "", 0},
		{"precision carry down", []string{"--to=si", "--format=%.0f", "--round=down", "999999"}, "", "999k\n", "", 0},
		{"iec precision cap", []string{"--to=iec", "--format=%.5f", "1500", "999999", "310174"}, "", "1.46500K\n976.56200K\n302.90500K\n", "", 0},
		{"unit separator", []string{"--to=iec-i", "--unit-separator=__", "2048"}, "", "2.0__Ki\n", "", 0},
		{"empty delimiter", []string{"-d", "", "--from=si", "4.0 K", "4  ", "2K "}, "", "4000\n4\n2000\n", "", 0},
		{"empty delimiter spaces", []string{"-d", "", "--from=si", "--unit-separator=  "}, "1  K\n2  M\n3  G\n", "1000\n2000000\n3000000000\n", "", 0},
		{"all ranges", []string{"--field=1,-,3", "--from=auto", "1K 2K 3K"}, "", "1000 2000 3000\n", "", 0},
		{"format error newline", []string{"--format=ab\ncd"}, "", "", "numfmt: format 'ab\\ncd' has no % directive\n", 1},
		{"format width overflow", []string{"--format=%18446744073709551616f"}, "", "", "numfmt: invalid format '%18446744073709551616f' (width overflow)\n", 1},
		{"format precision error", []string{"--format=%.-1f"}, "", "", "numfmt: invalid precision in format '%.-1f'\n", 1},
		{"format precision value overflow", []string{"--format=%.2f", "100000000000000000"}, "", "", "numfmt: value/precision too large to be printed: '1e+17/2' (consider using --to)\n", 2},
		{"format zero precision overflow", []string{"--format=%.40f", "0"}, "", "", "numfmt: value/precision too large to be printed: '0e+0/40' (consider using --to)\n", 2},
		{"invalid unit negative", []string{"--from-unit", "-1"}, "", "", "numfmt: invalid unit size: '-1'\n", 1},
		{"invalid unit overflow", []string{"--to-unit=18014398509481984Ki"}, "", "", "numfmt: invalid unit size: '18014398509481984Ki'\n", 1},
		{"negative positional", []string{"-8765432"}, "", "", "numfmt: unexpected argument: unknown option -8\n", 1},
		{"leading plus", []string{"--from=auto", "+1e-3"}, "", "", "numfmt: invalid number: '+1e-3'\n", 2},
		{"invalid utf8", []string{}, "10\n\xff", "10\n", "numfmt: invalid number: '\\377'\n", 2},
		{"scientific negative", []string{"--from=auto", "--", "-1e-5"}, "", "", "numfmt: invalid suffix in input: '-1e-5'\n", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, err bytes.Buffer
			code := runNumfmt(&Context{Args: tc.args, Stdin: strings.NewReader(tc.input), Stdout: &out, Stderr: &err, Env: []string{}})
			if code != tc.code || out.String() != tc.output || err.String() != tc.diagnostic {
				t.Fatalf("code=%d stdout=%q stderr=%q; want %d %q %q", code, out.String(), err.String(), tc.code, tc.output, tc.diagnostic)
			}
		})
	}
}

type numfmtBrokenWriter struct{}

func (numfmtBrokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestNumfmtWriteErrors(t *testing.T) {
	for _, mode := range []string{"abort", "fail", "warn", "ignore"} {
		t.Run(mode, func(t *testing.T) {
			var diagnostic bytes.Buffer
			code := runNumfmt(&Context{Args: []string{"--invalid=" + mode, "42"}, Stdin: strings.NewReader(""), Stdout: numfmtBrokenWriter{}, Stderr: &diagnostic})
			if code != 1 || !strings.Contains(diagnostic.String(), "write error") {
				t.Fatalf("code=%d stderr=%q", code, diagnostic.String())
			}
		})
	}
}
