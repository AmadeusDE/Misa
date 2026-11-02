package kittygfx

// Limits bounds parsing, decoding and terminal-side graphics resources.
// A zero or negative field means "use the default", see Normalize.
type Limits struct {
	MaxWidth         int   // per image, pixels
	MaxHeight        int   // per image, pixels
	MaxImageBytes    int64 // decoded bytes per image
	MaxTerminalBytes int64 // decoded bytes for every image in one terminal
	MaxImages        int   // stored images per terminal
	MaxPlacements    int   // placements per terminal
	MaxInFlightBytes int64 // encoded bytes of all incomplete transfers
	MaxPayload       int   // encoded payload bytes in one APC
}

const (
	defaultMaxWidth         = 16384
	defaultMaxHeight        = 16384
	defaultMaxImageBytes    = 256 << 20
	defaultMaxTerminalBytes = 512 << 20
	defaultMaxImages        = 256
	defaultMaxPlacements    = 1024
	defaultMaxInFlightBytes = 360 << 20
	defaultMaxPayload       = 8 << 10
)

// DefaultLimits returns the limits Misa's terminal and image output use.
func DefaultLimits() Limits {
	return Limits{
		MaxWidth:         defaultMaxWidth,
		MaxHeight:        defaultMaxHeight,
		MaxImageBytes:    defaultMaxImageBytes,
		MaxTerminalBytes: defaultMaxTerminalBytes,
		MaxImages:        defaultMaxImages,
		MaxPlacements:    defaultMaxPlacements,
		MaxInFlightBytes: defaultMaxInFlightBytes,
		MaxPayload:       defaultMaxPayload,
	}
}

// Normalize replaces every zero or negative field with its default.
func (l Limits) Normalize() Limits {
	d := DefaultLimits()
	if l.MaxWidth <= 0 {
		l.MaxWidth = d.MaxWidth
	}
	if l.MaxHeight <= 0 {
		l.MaxHeight = d.MaxHeight
	}
	if l.MaxImageBytes <= 0 {
		l.MaxImageBytes = d.MaxImageBytes
	}
	if l.MaxTerminalBytes <= 0 {
		l.MaxTerminalBytes = d.MaxTerminalBytes
	}
	if l.MaxImages <= 0 {
		l.MaxImages = d.MaxImages
	}
	if l.MaxPlacements <= 0 {
		l.MaxPlacements = d.MaxPlacements
	}
	if l.MaxInFlightBytes <= 0 {
		l.MaxInFlightBytes = d.MaxInFlightBytes
	}
	if l.MaxPayload <= 0 {
		l.MaxPayload = d.MaxPayload
	}
	return l
}
