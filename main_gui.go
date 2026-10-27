//go:build !nogui

package main

import "github.com/AmadeusDE/Misa/internal/gui"

func runGUI(dir string, argv []string, title string) int { return gui.Run(dir, argv, title) }
