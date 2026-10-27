package config

import (
	"embed"
	"io/fs"
)

//go:embed example
var exampleFS embed.FS

// Example is the shipped sample: ughbot.toml plus prompts/.
func Example() fs.FS {
	sub, _ := fs.Sub(exampleFS, "example")
	return sub
}

// ExampleConfig returns the sample ughbot.toml text.
func ExampleConfig() []byte {
	b, _ := exampleFS.ReadFile("example/ughbot.toml")
	return b
}
