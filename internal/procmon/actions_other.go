//go:build !unix && !windows

package procmon

var platformActions = Actions{Signals: windowsSignals[:1]}
