package ui

import "github.com/AmadeusDE/Misa/internal/imageout"

// imageOutputProbe is the future root output boundary's capability gate, not a
// pane's terminal-name heuristic. It deliberately remains unbound: Bubble Tea
// v2.0.7 has no supported complete-frame barrier for pre-text deletion and
// post-text graphics. See internal/imageout/TRANSPORT.md. WithOutput and tea.Raw
// alone cannot supply that barrier, so even explicit opt-in must fail closed.
//
// Do not wire a query, worker write, View escape, or mux forwarding path here
// until the output boundary, bounded input replies and downstream consumer are
// proven together. Half-block cell rendering needs no graphics side channel.
func imageOutputProbe(optIn bool) *imageout.Probe {
	return imageout.NewProbe(imageout.ProbeGates{OptIn: optIn})
}
