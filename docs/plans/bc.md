# bc

`bc` is a command-line calculator with the POSIX `bc` language. Misa's implementation is native Go in
`internal/builtins/bc*.go`; numbers are exact decimals built on `math/big`, so `0.1 + 0.2` is `.3`, never a
binary floating-point artefact.

Reference: [Toybox](https://github.com/landley/toybox) `toys/pending/bc.c` at commit
`6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb` (0BSD) and the POSIX `bc` specification. No code was copied:
Toybox stores base-100 digits and compiles to bytecode, this implementation keeps a `big.Int` with a decimal
scale and evaluates an AST. `tests/bc.test` exists upstream but its fixtures (`tests/bc/*.txt`) are not part
of the Toybox tree, so no upstream cases are vendored here.

## Usage

```
bc [-ilqsw] [file ...]
```

| Option | Behaviour |
|---|---|
| `-i`, `--interactive` | Print the version banner and keep going after errors inside standard input. |
| `-q`, `--quiet` | Do not print the banner (`-i -q`). |
| `-l`, `--mathlib` | **Not implemented**; exits 1 with `bc: -l (math library): not implemented`. |
| `-s`, `--standard` | **Not implemented**; exits 1 rather than silently accepting non-POSIX input. |
| `-w`, `--warn` | **Not implemented**; same reason. |
| `-h`, `--help`, `--version` | Usage and version, exit 0. |

Operands are read in order, then standard input. A `-` operand means standard input at that position.
A file is parsed as one unit; standard input is fed a logical line at a time (a unit is held back until it
parses, so multi-line blocks, strings in comments and backslash continuations work). `BC_LINE_LENGTH`
overrides the output wrap column (default 69; a value below 2 or above 65535 is ignored).

## Numbers

A value is an arbitrary-precision integer `unscaled` with a decimal `scale`: `value = unscaled / 10^scale`.
Every stored digit is exact; there is no rounding mode other than truncation toward zero where POSIX
requires it, and no floating point.

Literals follow POSIX: `123`, `.5`, `1.50`, upper-case digits `A`–`Z` in a base above 10, and one `.` at most.
A **one-character** literal is always the value of that digit regardless of `ibase` (`ibase=2; A` is 10).
Only **letters** that are too large for `ibase` clamp to `ibase-1`, as in Toybox (`ibase=2; 11A` is 7;
`ibase=10; 1A` is 19); `0`–`9` always keep their face value, so `ibase=2; 12` is 4. Lower-case letters never
continue a literal, so `ibase=16; 1a` is a syntax error (it reads as the number 1 then the name `a`). A
literal in a non-decimal base with a fraction is truncated to as many decimal places as it has fraction
digits (`ibase=16; .A` is `.6`); if that value is zero the result has scale 0, as in Toybox, so
`ibase=16; scale(.0001)` is 0 while `scale(0.0001)` in base 10 is 4.

Limits (`limits` prints them):

| Limit | Value |
|---|---|
| `BC_BASE_MAX` (obase) | 2147483647 |
| `BC_DIM_MAX` (array index) | 1000000 |
| `BC_SCALE_MAX` | 100000 |
| `BC_STRING_MAX` | 1000000 |
| `BC_NUM_MAX` (decimal digits in one value) | 1000000 |
| `BC_NAME_MAX` | 128 |
| `MAX Exponent` (`^`) | 1000000 |
| `Number of vars` | 65535 |

`limits` is a runtime statement here (Toybox prints the table from its parser, so it also prints inside a
branch that never runs); the names and order follow Toybox's eight-line table.

Anything that would exceed those limits fails with `bc: number too large`, `bc: overflow` or `bc: bad
scale; must be [0, 100000]` instead of allocating unbounded memory or looping forever. Function recursion is
capped at 1000 frames (`bc: call stack depth exceeded`).

## Scale rules

The `scale` variable always means "decimal places after the point". Operations follow POSIX:

| Operation | Result scale |
|---|---|
| `a + b`, `a - b` | `max(scale(a), scale(b))` |
| `a * b` | `min(scale(a)+scale(b), max(scale, scale(a), scale(b)))` |
| `a / b` | `scale` |
| `a % b` | `max(scale + scale(b), scale(a))` |
| `a ^ b` (`b` integer, `b > 0`) | `min(scale(a)*b, max(scale, scale(a)))` |
| `a ^ b` (`b < 0`) | `scale` |
| `sqrt(a)` | `max(scale, scale(a))`, but `scale` alone when `a` is zero (Toybox checks zero first) |

`a % b` is `a - (a/b)*b` using a quotient truncated at `scale`, so `7%3` is 1 and `7.5%2` with `scale=2` is
`0` while `scale=0` gives `1.5`. `a ^ b` requires an integer exponent (`bc: non integer number`
otherwise); `length(x)` is the count of significant digits (`length(0)` is 0, like Toybox) and `scale(x)` is
the stored scale.

## Language

Supported statements: expression, string, `print list`, `if`/`else`, `while`, `for`, `break`, `continue`,
`quit`, `halt`, `limits`, `return [(expr)]`, `define [void] name(params) { auto …; … }`. Statements end at a
newline or `;`; `{}` groups them. `if (c) s`, `while (c) s` and `for (…; …; …) s` accept either one statement
or a block, and newlines before the body are skipped, so the body may start on the next line. A `for` header
may omit any of its three parts (`for (;;)` loops until `break`) and may be broken across lines; a `print`
list may continue after a comma. A value of `1 +\n2` is still an error, as in Toybox.

Expressions, tightest first: `++`/`--` (postfix), `^` (right associative, a signed exponent is allowed),
unary `-` and `!`, `* / %`, `+ -`, `== != < <= > >=`, `&&`, `||`, assignment (`= += -= *= /= %= ^=`, right
associative). `-2^2` is `-4`. Relational and logical results are 1 or 0; `&&`/`||` evaluate both sides (no
short circuit), as Toybox does. `@`-style GNU extensions do not exist.

Primary expressions: number, variable, array element (`a[2]`), function call, `( expr )`, `last` or its `.`
shortcut, `ibase`, `obase`, `scale`, and the builtins `length`, `scale`, `sqrt`, `abs`. A bare assignment
statement is not printed and does not update `last`; an assignment whose statement starts with `(` is an
ordinary expression and is printed, so `(x) = 2` prints 2. Assigning `ibase`, `obase` or `scale` stores the
truncated integer but yields the right-hand side as written, so `(scale = 2.7)` prints 2.7 and leaves
`scale == 2`. `++`/`--` need an unparenthesized variable (`(x)++` is an error, as in Toybox).

Function parameters are values (pass by value) and `auto` names are locals; any other name is global.
Scoping is dynamic: a called function sees the autos and parameters of its callers, so a `g()` called from
`f()` reads and writes `f`'s `auto a` (the same rule Toybox and GNU bc use). Only one `auto` statement is
allowed per function. A `void` function cannot be used in an expression (`bc: cannot use a void value in an
expression` is reported through the value path) and may not `return` a value at all; `void` is a keyword here,
while Toybox has no `void` functions. Arrays are global;
array parameters, `auto a[]` and whole-array arguments are rejected at parse time
(`bc: array parameters are not supported`, `bc: array autos are not supported`).

Statements and `define` bodies may span lines. `quit` is a compile-time command, as in Toybox: parsing of the
current unit stops and nothing from that unit runs. `halt` stops execution immediately with status 0.

## Output

Values print in the current `obase`. In base 10 the value is printed with every stored fraction digit, a
leading `.` and no `0` before the point (`.50`, not `0.50`); zero always prints as `0`, whatever its scale.
Bases 2–16 print one digit per place with `0-9A-Z`; above 16 every digit prints as a space-separated group
of decimal digits (`.50`-style separator for the fraction), e.g. `obase=100; 12345` prints ` 01 23 45`.

Output wraps exactly like Toybox: a `\` and a newline are inserted once the current column reaches
`BC_LINE_LENGTH-1` (default 68), including in the middle of a long number. `print` writes without a trailing
newline; every non-assignment expression statement adds one. A **string statement** prints the string
verbatim — no escape processing and no newline — which is what Toybox's `BC_INST_PRINT_STR` does; use
`print "…"` when escapes are wanted. `print` strings substitute `\a \b \\ \e \f \n \" \r \t`; an unknown
escape prints the backslash and the character. A string literal ends at the next `"` — backslash does not
escape the quote, which is Toybox's lexer behaviour.

## Errors

Diagnostics go to standard error with a `bc: ` prefix and exit status 1, matching the other builtins:

```
bc: divide by zero
bc: <stdin>:3: bad expression: unexpected ")"
bc: bad ibase; must be [2, 16]
```

Parse errors carry `file:line`; runtime errors carry the message alone, like Toybox's runtime errors. Usage
errors (unknown option, unimplemented option) are reported the same way with exit status 1, which is Toybox's
status for `bc` errors.

## Known differences from Toybox bc

- No math library (`-l`): `s()`, `c()`, `a()`, `l()`, `e()` and `j()` are not defined. This is the intended
  follow-up PR. `-l` fails instead of pretending to load it.
- No POSIX extension checking (`-s`, `-w`): they fail rather than pass unchecked non-POSIX constructs.
- `read()` is rejected at parse time; Toybox reads and evaluates an expression from the input.
- `limits` runs as a statement instead of printing while parsing, so it does not print from a branch that is
  never executed.
- The escape table is the obvious one: `\e` is ESC, `\"` is a quote and an unknown escape keeps its
  backslash. Toybox's table has `\e` print a backslash and its next `"` print a quote (`\q`), which looks
  like a typo in that table; base-10 digits, letter clamping and every other escape match.
- `void` functions exist here but not in Toybox, and a `void` function may not `return` a value even without
  parentheses (Toybox only diagnoses `return (x)`).
- `BC_ENV_ARGS`, `-e`/`-f`-style source options and the Toybox version/copyright banner do not exist; the
  `-i` banner is one `bc (Misa) 0.1.0` line.
- No array parameters, array autos or whole-array arguments; those constructions are parse errors instead of
  being silently ignored.
- Standard input is accumulated until the buffered text parses, so a multi-line `define`, `if` or `while`
  typed or piped into `bc` runs; Toybox's line-at-a-time stdin driver reports an end-of-file parse error
  there.
- An `else` joins its `if` when both are in the same unit: a file or `-` operand is one unit (and a `{}`
  block skips the newline), but on standard input, which is parsed a line at a time, `if (0) 1` and a
  following `else 2` are separate units, so the `else` is a syntax error there and the `if` line stands
  alone. Use `-` (`bc -`) or braces when a script needs that layout.
- `POSIXLY_CORRECT` does not switch on POSIX extension checking (that mode is unimplemented).
- Limits, cancellation and `limits` output report this implementation's maxima, not Toybox's `UINT_MAX`
  based ones. Numbers beyond the limits fail with an explicit error rather than exhausting memory.
- A backslash-newline inside standard input continues the line, but an unterminated string is an error rather
  than being carried to the next line.

## Cancellation and work bounds

`Context.Cancel` (closed by the caller) stops a running program: the interpreter checks it every 1024
evaluation steps, every loop iteration and before every loop condition, and reports `bc: interrupted` with
exit status 1. A failing stdout (`EPIPE`, `/dev/full`) stops the program with exit status 1 instead of
spinning. Use `tick` bounds in `bc_exec.go` as the single place to add further cooperative checks.
