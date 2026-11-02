//go:build !unix

package imgview

import (
	"os"
	"os/signal"
)

func standaloneSignals(ch chan<- os.Signal) { signal.Notify(ch, os.Interrupt) }
func standaloneResizeSignal(os.Signal) bool { return false }
func standaloneSuspend()                    {}
func standalonePixels(*os.File) (int, int)  { return 0, 0 }
