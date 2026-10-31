// Package launcher retains the completion message used by legacy UI screens.
package launcher

// Finished reports completion of a legacy host process.
type Finished struct {
	Err error
}
