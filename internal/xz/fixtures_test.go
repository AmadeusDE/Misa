package xz

// Test fixtures are small .xz files embedded as base64. They were produced
// with XZ Utils 5.8.1 on the development machine, e.g.
//
//	printf 'hello xz world\n' | xz -c > hello.xz
//	printf 'hello xz world\n' | xz -c --check=crc32 > hello-crc32.xz
//	printf 'hello xz world\n' | xz -c --lzma2=dict=64KiB > hello-dict64k.xz
//	python3 -c '...' | xz -c --block-size=1024 > blocks.xz   # 60 repeated lines
//	python3 -c '...' | xz -c --x86 --lzma2 > x86.xz           # x86 call/jump bytes
//	printf 'hello xz world\n' | xz -c --lzma2=dict=128MiB > bigdict.xz
//	cat hello.xz hello.xz > two-streams.xz
//	( cat hello.xz; printf '\0\0\0\0'; cat hello.xz ) > padding.xz
//
// The uncompressed payloads are rebuilt in Go below (helloText, blockText,
// x86Data), so only the compressed bytes need embedding.

import (
	"bytes"
	"encoding/base64"
	"io"
	"strings"
	"testing"
)

var testFixtures = map[string]string{
	"helloXZ": "/Td6WFoAAATm1rRGBMATDyEBFgAAAAAAAAAAAG75/+kBAA5oZWxsbyB4eiB3b3JsZAoAAAvasw2+" +
		"JGuHAAEvD9eQJaIftvN9AQAAAAAEWVo=",
	"helloCRC32": "/Td6WFoAAAFpIt42BMATDyEBFgAAAAAAAAAAAG75/+kBAA5oZWxsbyB4eiB3b3JsZAoAAMdd1rIA" +
		"ASsP01VJxpBCmQ0BAAAAAAFZWg==",
	"helloSHA256": "/Td6WFoAAArh+wyhBMATDyEBFgAAAAAAAAAAAG75/+kBAA5oZWxsbyB4eiB3b3JsZAoAAOoMlRwR" +
		"eo5smgp9T9E2Ad5tHMgJ3vLyPZVVwgm2dPF3AAFHD3hxAQ8Ym0uaAQAAAAAKWVo=",
	"helloNone": "/Td6WFoAAAD/EtlBBMATDyEBFgAAAAAAAAAAAG75/+kBAA5oZWxsbyB4eiB3b3JsZAoAAAABJw/f" +
		"GvxqBnKeegEAAAAAAFla",
	"helloDict64K": "/Td6WFoAAATm1rRGA8ATDyEBCAAAAAAAxCY6wQEADmhlbGxvIHh6IHdvcmxkCgAAC9qzDb4ka4cA" +
		"ASsP01VJxh+2830BAAAAAARZWg==",
	"blocksXZ": "/Td6WFoAAATm1rRGA8BLgAghARYAAAAA7Vw+M+AD/wBDXQAqGgiiAyVm8Ut4xaIF/y7m2dIgGq00" +
		"+OId6EE2+twGabs85BA0Jwnrs2bj7TKUI0BpMu2GInXye2qVJ23T2zjXZ5AAAAAinJEYJWdBTgPA" +
		"S4AIIQEWAAAAAO1cPjPgA/8AQ10AMxvLciGbvmq9ou8SwgeR6+ctaCHZzRW2lfQArsp9ZiLWBk+j" +
		"1AVYqTT6jzzMh64ITr4ApcTj2t3O0xQvpoZquO4mAAAAwWm9t7/5ZNUDwEuACCEBFgAAAADtXD4z" +
		"4AP/AENdADQZQAXBmzGZSk/JFtCZGYv+JF5CE2er3JhYmQ+bQ9AQGgPwYI8FPj48+xSXZG4CRNwl" +
		"mw2XmOQwjr+SzbM+tUMZ1QAAAI0Qkx0DwScLA8BGoAIhARYAAAAANknx1eABHwA+XQAZjQL8/7KG" +
		"jYrPij7DZ/zSsVKmQsovFH6Asuw9smmAVMmR1ee8rJMXMEA/aNg8QtXth0UjQlfM3y8KfjLAAAAA" +
		"AJIctjP5gSVxAARjgAhjgAhjgAheoAIAAAv/G1esJz4tBAAAAAAEWVo=",
	"x86XZ": "/Td6WFoAAATm1rRGBMGZBeASBAAhARYAAAAAAEOedaLgCV8CkV0AdAE8GTi78Fg32B7YZO7aVqkv" +
		"hcdezdCw4KewRwlf7AzLxh45kEVkyVZpwzH/Xa0JkqnfTO9tRacyqq2q+b8z460TI3ISyXHMN6vx" +
		"S19iecOYSYR0OM9gACaMKdhP0YNoS0ABjM0ogwvEEfqyUii2KhqApcxYG/CHhwQGHRralC5Gpqoy" +
		"dRrQxHF3U6F4t+lWZ/0l5JjWIGw4//rqMwrSdCw2h1cn+r76glU02S5RrgqRtxH+NmoV367IyntE" +
		"vEXbzNFmL/8FkPS2anoye8yQjCcqJ+CjFbPfdIxj/xSidxt4miEqn2GFYbKHbmoYfF5fL0mU/AOU" +
		"CyFtfmWrCih36Fn5PJn0tEXvnpNJxVfPDzCO0ZmypgwZJu/4NTwwMHfL6lZjDr5bQdJ5vh0CoTlz" +
		"vGasbiU4PNl6SVJLRkTXBJPdIkSF5b4VKarNDBa+SGlAcx2R69NMbPIdw/GOpXWi2dhtI19siEQY" +
		"pdMvF8kuD1DDF9kl5wb/n97YN6oJyRJBXA/KsGX2IFvrCRz/D3LxoqeVjXO2V36CQtle66+Yh2bg" +
		"ZrpbnIXL3oveOlpnqE2NRFrCs5gTws6DvwWe4agV4wQuJjq0noQOZIAFACuDQZ4NQ6U79yfGC3cj" +
		"4L0ra56XY9sPJWlKWcDwyHqPAPoWZFOOhmi23OnqG+6dEy+jAAnL++onevl+fYO0oqPnhh5Xt/XJ" +
		"zcI4oKrZH/l7WJLpXsaJ5mSWOirxaHw7O8tt+9hCk2YGVhwJs8Rxu0oAfDrkfbW+aF/KTNn8XU/w" +
		"O6x6SIWGPZn2yQO9etn7xZkseGoBQj1ag16+i98hwi+hEM8EP4uQqscGpM9cOBE3QlUf2sM2Ig03" +
		"1T/NcVVxi8QpMasAAAAAAKpSv08xgQpvAAG1BeASAABN72+IscRn+wIAAAAABFla",
	"deltaXZ": "/Td6WFoAAATm1rRGBMETDwMBACEBFgAAAAAAAMSpkNMBAA5o/QcAA7FYAqZX+AP6+KYAAAvasw2+" +
		"JGuHAAEvD9eQJaIftvN9AQAAAAAEWVo=",
	"bigDictXZ": "/Td6WFoAAATm1rRGBMATDyEBHgAAAAAAAAAAAJq5KPoBAA5oZWxsbyB4eiB3b3JsZAoAAAvasw2+" +
		"JGuHAAEvD9eQJaIftvN9AQAAAAAEWVo=",
	"twoStreamsXZ": "/Td6WFoAAATm1rRGBMATDyEBFgAAAAAAAAAAAG75/+kBAA5oZWxsbyB4eiB3b3JsZAoAAAvasw2+" +
		"JGuHAAEvD9eQJaIftvN9AQAAAAAEWVr9N3pYWgAABObWtEYEwBMPIQEWAAAAAAAAAAAAbvn/6QEA" +
		"DmhlbGxvIHh6IHdvcmxkCgAAC9qzDb4ka4cAAS8P15Aloh+2830BAAAAAARZWg==",
	"paddingXZ": "/Td6WFoAAATm1rRGBMATDyEBFgAAAAAAAAAAAG75/+kBAA5oZWxsbyB4eiB3b3JsZAoAAAvasw2+" +
		"JGuHAAEvD9eQJaIftvN9AQAAAAAEWVoAAAAA/Td6WFoAAATm1rRGBMATDyEBFgAAAAAAAAAAAG75" +
		"/+kBAA5oZWxsbyB4eiB3b3JsZAoAAAvasw2+JGuHAAEvD9eQJaIftvN9AQAAAAAEWVo=",
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	enc, ok := testFixtures[name]
	if !ok {
		t.Fatalf("no fixture %q", name)
	}
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatalf("fixture %q: %v", name, err)
	}
	return data
}

const helloText = "hello xz world\n"

func blockText() []byte {
	return []byte(strings.Repeat("The quick brown fox jumps over the lazy dog. 0123456789\n", 60))
}

// x86Data mirrors the generator used for the x86.xz fixture.
func x86Data() []byte {
	var b []byte
	for i := 0; i < 200; i++ {
		b = append(b, 0xE8, byte(i), byte(i*7), 0x00, 0x00, 0x90, 0x90, 0xE9, 0xff, 0xff, 0xff, 0xff)
	}
	return b
}

func decodeAll(r io.Reader) ([]byte, error) {
	var out bytes.Buffer
	_, err := io.Copy(&out, r)
	return out.Bytes(), err
}
