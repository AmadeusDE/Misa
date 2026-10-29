// Command listbuiltins prints builtins.Names, one per line. android/build-go.sh
// writes that list into the APK so Bootstrap can symlink each multicall name
// at filesDir/bin without executing libmisa.so.
package main

import (
	"fmt"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

func main() {
	for _, name := range builtins.Names() {
		fmt.Println(name)
	}
}
