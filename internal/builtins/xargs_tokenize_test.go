package builtins

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTokenizeXargs(t *testing.T) {
	nul, comma, newline := byte(0), byte(','), byte('\n')
	tests := []struct {
		name, input, eof string
		delim            *byte
		want             []xargsToken
	}{
		{"empty", " \t\n", "", nil, nil},
		{"words and lines", "  alpha\tbeta\n\n gamma\n", "", nil, []xargsToken{{"alpha", 1}, {"beta", 1}, {"gamma", 2}}},
		{"quotes and escapes", `a' b'"c d" e\ f '' "" 'a\b' \"`, "", nil, []xargsToken{{"a bc d", 1}, {"e f", 1}, {"", 1}, {"", 1}, {`a\b`, 1}, {`"`, 1}}},
		{"escaped newline", "a\\\nb c\n", "", nil, []xargsToken{{"a\nb", 1}, {"c", 1}}},
		{"trailing backslash", "x\\", "", nil, []xargsToken{{"x", 1}}},
		{"logical eof", "first STOP ignored\nlater", "STOP", nil, []xargsToken{{"first", 1}}},
		{"logical eof at end", "first STOP", "STOP", nil, []xargsToken{{"first", 1}}},
		{"nul", "one\x00\x00two\nlast\x00", "", &nul, []xargsToken{{"one", 1}, {"", 2}, {"two\nlast", 3}}},
		{"literal delimiter", "'a b',,c\\d,tail", "c\\d", &comma, []xargsToken{{"'a b'", 1}, {"", 2}, {`c\d`, 3}, {"tail", 4}}},
		{"continued lines", "a  \nb\n\nc\n", "", nil, []xargsToken{{"a", 1}, {"b", 1}, {"c", 2}}},
		{"newline delimiter", "a\n\nb\n", "", &newline, []xargsToken{{"a", 1}, {"", 2}, {"b", 3}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tokenizeXargs(context.Background(), strings.NewReader(tt.input), tt.delim, tt.eof)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, %v; want %#v", got, err, tt.want)
			}
		})
	}
}

func TestTokenizeXargsErrors(t *testing.T) {
	for _, input := range []string{"'unfinished", "\"unfinished", "'line\nnext'"} {
		if _, err := tokenizeXargs(context.Background(), strings.NewReader(input), nil, ""); err == nil {
			t.Errorf("%q: expected quote error", input)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tokenizeXargs(ctx, strings.NewReader("input"), nil, ""); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled context: %v", err)
	}
	broken := errors.New("read failed")
	if _, err := tokenizeXargs(context.Background(), errorReader{broken}, nil, ""); !errors.Is(err, broken) {
		t.Errorf("reader error: %v", err)
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

var _ io.Reader = errorReader{}

func TestXargsTokenizerStreamsPipe(t *testing.T) {
	for _, delim := range []*byte{nil, func() *byte { b := byte(0); return &b }()} {
		reader, writer := io.Pipe()
		defer reader.Close()
		defer writer.Close()
		tokenizer := newXargsTokenizer(context.Background(), reader, delim, "", false)
		input := "first "
		if delim != nil {
			input = "first\x00"
		}
		go func() { _, _ = io.WriteString(writer, input) }()
		result := make(chan xargsToken, 1)
		errs := make(chan error, 1)
		go func() {
			tok, err := tokenizer.Next()
			result <- tok
			errs <- err
		}()
		select {
		case tok := <-result:
			if err := <-errs; err != nil || tok != (xargsToken{"first", 1}) {
				t.Fatalf("got %#v, %v", tok, err)
			}
		case <-time.After(time.Second):
			t.Fatal("Next waited for the pipe to close after a complete argument")
		}
	}
}

func TestXargsTokenizerReplacement(t *testing.T) {
	input := "  red   blue  \n\n 'quoted word' x\\ y\n''\nSTOP\nignored\n"
	tok := newXargsTokenizer(context.Background(), strings.NewReader(input), nil, "STOP", true)
	want := []xargsToken{{"red   blue  ", 1}, {"quoted word x y", 2}, {"", 3}}
	for _, expected := range want {
		got, err := tok.Next()
		if err != nil || got != expected {
			t.Fatalf("got %#v, %v; want %#v", got, err, expected)
		}
	}
	if _, err := tok.Next(); err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
	zero := byte(0)
	tok = newXargsTokenizer(context.Background(), strings.NewReader(" a b\x00'c'\x00"), &zero, "", true)
	for _, expected := range []xargsToken{{" a b", 1}, {"'c'", 2}} {
		got, err := tok.Next()
		if err != nil || got != expected {
			t.Fatalf("literal replacement: got %#v, %v; want %#v", got, err, expected)
		}
	}
}
