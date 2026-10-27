// Icon mappings derived from g (github.com/Equationzhao/g), MIT license.
//
// Copyright (c) 2023 Equationzhao
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package lscore

import (
	"os"
	"path/filepath"
	"strings"
)

var dirIcons = map[string]string{
	".git":         "",
	".github":      "",
	".vscode":      "󰨞",
	"node_modules": "",
	"src":          "󰴉",
	"bin":          "",
	"build":        "",
	"dist":         "",
	"docs":         "󰈙",
	"test":         "",
	"tests":        "",
	"vendor":       "",
	"config":       "",
	"public":       "",
	"assets":       "󰉏",
	"images":       "󰉏",
	"img":          "󰉏",
	"lib":          "",
	"target":       "",
	"tmp":          "",
	"cache":        "",
	".cache":       "",
	"pkg":          "",
	"cmd":          "",
	"internal":     "󰉋",
	"scripts":      "",
	"docker":       "󰡨",
	"containers":   "󰡨",
}

var fileIcons = map[string]string{
	"Makefile":            "",
	"CMakeLists.txt":      "",
	"Dockerfile":          "󰡨",
	"Containerfile":       "󰡨",
	"docker-compose.yml":  "󰡨",
	"docker-compose.yaml": "󰡨",
	".gitignore":          "",
	".gitmodules":         "",
	".gitattributes":      "",
	"LICENSE":             "",
	"LICENCE":             "",
	"Justfile":            "",
	"justfile":            "",
	"go.mod":              "",
	"go.sum":              "",
	"Cargo.toml":          "",
	"Cargo.lock":          "",
	"package.json":        "",
	"tsconfig.json":       "",
	"README.md":           "󰈙",
	"CHANGELOG.md":        "󰃻",
	".env":                "",
	".editorconfig":       "",
	"flake.nix":           "",
	"flake.lock":          "",
}

var extIcons = map[string]string{
	".go":    "",
	".rs":    "",
	".py":    "",
	".js":    "",
	".jsx":   "",
	".ts":    "",
	".tsx":   "",
	".c":     "",
	".cpp":   "",
	".cc":    "",
	".h":     "",
	".hpp":   "",
	".java":  "",
	".kt":    "",
	".rb":    "",
	".php":   "",
	".lua":   "",
	".zig":   "",
	".nim":   "",
	".ex":    "",
	".exs":   "",
	".erl":   "",
	".hs":    "",
	".ml":    "",
	".scala": "",
	".swift": "",
	".dart":  "",
	".r":     "󰟔",
	".R":     "󰟔",
	".cs":    "󰌛",
	".fs":    "",
	".v":     "",
	".sh":    "",
	".bash":  "",
	".zsh":   "",
	".fish":  "",
	".ps1":   "󰨊",
	".nix":   "",
	".vim":   "",
	".el":    "",

	".html":   "",
	".htm":    "",
	".css":    "",
	".scss":   "",
	".sass":   "",
	".less":   "",
	".vue":    "",
	".svelte": "",

	".json": "",
	".yaml": "",
	".yml":  "",
	".toml": "",
	".xml":  "󰗀",
	".ini":  "",
	".cfg":  "",
	".conf": "",

	".md":   "󰍔",
	".txt":  "󰈙",
	".rst":  "󰈙",
	".org":  "",
	".tex":  "",
	".pdf":  "",
	".doc":  "󰈬",
	".docx": "󰈬",
	".xls":  "󰈛",
	".xlsx": "󰈛",
	".ppt":  "󰈧",
	".pptx": "󰈧",
	".csv":  "",

	".png":  "󰋩",
	".jpg":  "󰋩",
	".jpeg": "󰋩",
	".gif":  "󰋩",
	".bmp":  "󰋩",
	".ico":  "󰋩",
	".svg":  "󰜡",
	".webp": "󰋩",
	".avif": "󰋩",

	".mp3":  "󰎆",
	".flac": "󰎆",
	".wav":  "󰎆",
	".ogg":  "󰎆",
	".aac":  "󰎆",
	".opus": "󰎆",
	".m4a":  "󰎆",
	".wma":  "󰎆",

	".mp4":  "",
	".mkv":  "",
	".avi":  "",
	".mov":  "",
	".webm": "",
	".flv":  "",
	".wmv":  "",

	".zip": "",
	".tar": "",
	".gz":  "",
	".bz2": "",
	".xz":  "",
	".zst": "",
	".7z":  "",
	".rar": "",
	".deb": "",
	".rpm": "",

	".iso": "󰗮",
	".img": "󰗮",

	".db":     "",
	".sql":    "",
	".sqlite": "",

	".lock": "",
	".log":  "󰌱",
	".pid":  "",

	".o":     "",
	".so":    "",
	".a":     "",
	".dll":   "",
	".dylib": "",
	".wasm":  "",
}

const (
	iconDir     = ""
	iconSymlink = ""
	iconExec    = ""
	iconFile    = ""
	iconPipe    = "󰈲"
	iconSocket  = "󰆨"
	iconDevice  = "󰜫"
)

func Icon(fi *FileInfo) string {
	if fi.IsDir {
		base := strings.ToLower(filepath.Base(fi.Name))
		if icon, ok := dirIcons[base]; ok {
			return icon
		}
		return iconDir
	}

	if fi.IsSymlink {
		return iconSymlink
	}

	if fi.Mode&os.ModeNamedPipe != 0 {
		return iconPipe
	}
	if fi.Mode&os.ModeSocket != 0 {
		return iconSocket
	}
	if fi.Mode&os.ModeDevice != 0 {
		return iconDevice
	}

	name := fi.Name
	if icon, ok := fileIcons[name]; ok {
		return icon
	}

	ext := filepath.Ext(name)
	if ext != "" {
		if icon, ok := extIcons[strings.ToLower(ext)]; ok {
			return icon
		}
	}

	if fi.Mode&0111 != 0 {
		return iconExec
	}

	return iconFile
}
