package term

import (
	"errors"

	"github.com/AmadeusDE/Misa/internal/kittygfx"
)

// SetGraphicsEnabled decides whether this terminal accepts Kitty graphics from
// its program. It is off by default: until a frontend can draw images, the
// terminal must not claim that it can, so graphics are swallowed instead.
func (t *Terminal) SetGraphicsEnabled(on bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.gfx.closed {
		// A closed terminal keeps no state: bytes still queued for the parser
		// must not be able to store an image again.
		return
	}
	t.gfx.setEnabled(on)
	t.gfxJobs = nil
}

// GraphicsLimits returns the resource caps in force. A frontend uses it to
// report what it actually applied, not what it asked for.
func (t *Terminal) GraphicsLimits() kittygfx.Limits {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.kittyLimits()
}

// GraphicsEnabled reports whether the terminal accepts Kitty graphics.
func (t *Terminal) GraphicsEnabled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.gfx.enabled
}

// SetGraphicsLimits replaces the resource caps. Shrinking them evicts what no
// longer fits and discards transfers that are still in flight.
func (t *Terminal) SetGraphicsLimits(l kittygfx.Limits) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.gfx.setLimits(graphicsLimitsFrom(l))
	t.gfxJobs = nil
}

func (t *Terminal) kittyLimits() kittygfx.Limits {
	l := t.gfx.limits
	return kittygfx.Limits{
		MaxWidth:         l.imageWidth,
		MaxHeight:        l.imageHeight,
		MaxImageBytes:    l.imageBytes,
		MaxTerminalBytes: l.totalBytes,
		MaxImages:        l.images,
		MaxPlacements:    l.placements,
		MaxInFlightBytes: l.inFlightBytes,
		MaxPayload:       l.payloadBytes,
	}
}

var (
	errGraphicsNoImage   = errors.New("no such image")
	errGraphicsNeedID    = errors.New("image id or number required")
	errGraphicsBadSource = errors.New("source rectangle outside the image")
	errGraphicsTooLong   = errors.New("payload too long")
	errGraphicsDelete    = errors.New("malformed delete specifier")
)

// gfxCode maps an error to the Kitty reply code and message.
func gfxCode(err error) (code, message string) {
	switch {
	case err == nil:
		return "", ""
	case errors.Is(err, kittygfx.ErrUnsupported):
		// Anything this terminal does not implement is refused as a bad
		// request: the subset is fixed, so there is no feature to negotiate.
		return "EINVAL", err.Error()
	case errors.Is(err, kittygfx.ErrTooLarge), errors.Is(err, errGraphicsTooLong):
		return "ENOSPC", err.Error()
	case errors.Is(err, errGraphicsNoImage):
		return "ENOENT", err.Error()
	}
	return "EINVAL", err.Error()
}

// gfxReply answers a graphics command. The caller holds t.mu. Quiet levels in
// the command decide whether anything is sent.
func (t *Terminal) gfxReply(cmd kittygfx.Command, err error) {
	if !t.gfx.enabled {
		return
	}
	code, message := gfxCode(err)
	if out := kittygfx.Reply(cmd, code, message); out != nil {
		t.respond(string(out))
	}
}

// startAPC begins an APC string (ESC _). Only a body starting with G is
// interpreted; every other APC keeps the old swallow-until-ST behaviour.
func (t *Terminal) startAPC() {
	t.apc = t.apc[:0]
	t.apcGfx = false
	t.apcDecided = false
	t.apcOverflow = false
	t.state = stAPC
}

// apcByte collects one APC byte. The payload never reaches the screen and the
// buffer stops growing at the configured payload limit.
func (t *Terminal) apcByte(b byte) {
	if b == '\x1b' {
		t.state = stAPCEsc
		return
	}
	if b == '\a' {
		t.finishAPC()
		t.state = stNormal
		return
	}
	if !t.gfx.enabled {
		return
	}
	if !t.apcDecided {
		t.apcDecided = true
		t.apcGfx = b == 'G'
		return
	}
	if !t.apcGfx || t.apcOverflow {
		return
	}
	// The cap counts the whole APC body, header keys included, while
	// kittygfx.Parse applies MaxPayload to the payload after the ';'. The
	// terminal is therefore a little stricter than a standalone parser: a body
	// that only just fits the protocol limit is refused here, which bounds the
	// buffer before the keys are even parsed.
	if len(t.apc) >= t.gfx.limits.payloadBytes {
		t.apcOverflow = true
		return
	}
	t.apc = append(t.apc, b)
}

// finishAPC dispatches the collected body at a string terminator.
func (t *Terminal) finishAPC() {
	body, overflow, gfx := t.apc, t.apcOverflow, t.apcGfx
	t.apc, t.apcGfx, t.apcDecided, t.apcOverflow = t.apc[:0], false, false, false
	if !gfx || !t.gfx.enabled || t.gfx.closed {
		return
	}
	if overflow {
		cmd, _ := kittygfx.Parse(body, t.kittyLimits())
		t.gfxReply(cmd, errGraphicsTooLong)
		t.gfx.transfer = nil
		return
	}
	t.dispatchGraphics(body)
	// Decode and commit now, before the next byte: text, scrolling and screen
	// switches later in the same write must see this command's effect. Only
	// processMu holders run the parser, so nothing else touches parse state
	// while the lock is released.
	t.mu.Unlock()
	t.drainGraphics()
	t.mu.Lock()
}

// abortAPC drops an APC that ended in an escape sequence instead of ST, which
// reclaims any transfer it was feeding.
func (t *Terminal) abortAPC() {
	t.apc, t.apcGfx, t.apcDecided, t.apcOverflow = t.apc[:0], false, false, false
	t.gfx.transfer = nil
}

// dispatchGraphics parses one G APC body and queues the work it describes.
// Parsing runs under the terminal lock, so it only buffers: decoding a finished
// transfer and committing it happen in drainGraphics, outside t.mu.
func (t *Terminal) dispatchGraphics(body []byte) {
	cmd, err := kittygfx.Parse(body, t.kittyLimits())
	if err != nil {
		t.gfxReply(cmd, err)
		t.gfx.transfer = nil
		return
	}
	switch cmd.Action {
	case kittygfx.ActionTransmit, kittygfx.ActionTransmitPlace:
		t.graphicsTransmit(cmd)
	case kittygfx.ActionPlace:
		t.graphicsPlace(cmd)
	case kittygfx.ActionDelete:
		t.gfxJobs = append(t.gfxJobs, gfxJob{cmd: cmd, screen: t.shownScreen()})
	case kittygfx.ActionQuery:
		t.gfxJobs = append(t.gfxJobs, gfxJob{cmd: cmd, screen: t.shownScreen(), data: cmd.Payload})
	}
}

// gfxTransfer is one chunked transmit in progress. Its encoded payload is
// bounded by the in-flight limit and reclaimed whenever the store is reset.
type gfxTransfer struct {
	screen screen
	cmd    kittygfx.Command // the first chunk: identity, format and source size
	data   []byte           // base64 payload of the chunks received so far
}

func (tr *gfxTransfer) sameImage(cmd kittygfx.Command) bool {
	if cmd.Has('i') && cmd.ID != tr.cmd.ID {
		return false
	}
	if cmd.Has('I') && cmd.Number != tr.cmd.Number {
		return false
	}
	return true
}

// graphicsTransmit accumulates a chunked transfer and queues the decode of a
// finished one. Only the first chunk needs the image identity; the protocol
// lets later chunks carry nothing but m and their payload.
func (t *Terminal) graphicsTransmit(cmd kittygfx.Command) {
	if tr := t.gfx.transfer; tr != nil && !tr.sameImage(cmd) {
		t.gfx.transfer = nil
	}
	tr := t.gfx.transfer
	if tr == nil && !cmd.Has('i') && !cmd.Has('I') {
		// A chunk with no transfer to join and no identity cannot address an
		// image; it is dropped without a reply rather than reported as an
		// error the client cannot act on.
		return
	}
	if cmd.More {
		if tr == nil {
			first := cmd
			first.Payload = nil
			tr = &gfxTransfer{screen: t.shownScreen(), cmd: first}
			t.gfx.transfer = tr
		}
		if !t.gfxAppend(tr, cmd.Payload) {
			t.gfx.transfer = nil
			t.gfxReply(tr.cmd, errGraphicsTooLong)
		}
		return
	}
	first, data := cmd, []byte(nil)
	if tr != nil {
		first, data = tr.cmd, tr.data
		t.gfx.transfer = nil
	}
	if int64(len(data))+int64(len(cmd.Payload)) > t.gfx.limits.inFlightBytes {
		t.gfxReply(first, errGraphicsTooLong)
		return
	}
	t.queueTransmit(first, append(data, cmd.Payload...))
}

// gfxAppend adds one chunk unless the transfer would pass the in-flight limit.
func (t *Terminal) gfxAppend(tr *gfxTransfer, chunk []byte) bool {
	if int64(len(tr.data))+int64(len(chunk)) > t.gfx.limits.inFlightBytes {
		return false
	}
	tr.data = append(tr.data, chunk...)
	return true
}

// gfxJob is a graphics command waiting for work that must run outside the
// terminal lock. Jobs keep the order the program wrote them in.
type gfxJob struct {
	cmd    kittygfx.Command
	screen screen
	number uint32
	data   []byte // base64 payload of a complete transmit
	place  *gfxPlacement
}

// drainGraphics applies queued graphics work, releasing t.mu while a transfer
// is decoded. The caller holds processMu, so jobs keep their order.
func (t *Terminal) drainGraphics() bool {
	t.mu.Lock()
	gen := t.gfx.gen
	t.mu.Unlock()
	for {
		t.mu.Lock()
		if len(t.gfxJobs) == 0 {
			t.mu.Unlock()
			break
		}
		job := t.gfxJobs[0]
		copy(t.gfxJobs, t.gfxJobs[1:])
		t.gfxJobs[len(t.gfxJobs)-1] = gfxJob{}
		t.gfxJobs = t.gfxJobs[:len(t.gfxJobs)-1]
		epoch, limits := t.gfx.epoch, t.kittyLimits()
		t.mu.Unlock()

		if job.cmd.Action == kittygfx.ActionQuery {
			err := validateQuery(job, limits)
			t.mu.Lock()
			if t.gfx.epoch == epoch {
				t.gfxReply(job.cmd, err)
			}
			t.mu.Unlock()
			continue
		}
		if job.cmd.Action == kittygfx.ActionTransmit || job.cmd.Action == kittygfx.ActionTransmitPlace {
			raster, err := kittygfx.Decode(job.cmd, job.data, limits)
			t.mu.Lock()
			switch {
			case t.gfx.epoch != epoch:
			case err != nil:
				t.gfxReply(job.cmd, err)
			default:
				t.commitTransmit(job, raster)
			}
			t.mu.Unlock()
			continue
		}
		t.mu.Lock()
		if t.gfx.epoch == epoch {
			t.gfxReply(job.cmd, t.applyGraphics(job))
		}
		t.mu.Unlock()
	}
	t.mu.Lock()
	changed := t.gfx.gen != gen
	t.mu.Unlock()
	return changed
}

// commitTransmit installs decoded pixels and any placement that came with them.
func (t *Terminal) commitTransmit(job gfxJob, raster *kittygfx.Raster) {
	bounds := raster.Bounds()
	bytes := int64(bounds.Dx()) * int64(bounds.Dy()) * 3
	if raster.Format() == 32 {
		bytes = int64(bounds.Dx()) * int64(bounds.Dy()) * 4
	}
	id := t.gfx.putImage(job.screen, job.cmd.ID, job.cmd.Number, raster, bytes)
	if job.place != nil {
		job.place.imageID = id
		t.gfx.place(job.place)
		t.gfxAdvance(job.cmd, *job.place)
	}
	t.gfxReply(job.cmd, nil)
}

// applyGraphics runs the jobs that need no decode: place, delete and query.
func (t *Terminal) applyGraphics(job gfxJob) error {
	switch job.cmd.Action {
	case kittygfx.ActionPlace:
		return t.applyPlacement(job)
	case kittygfx.ActionDelete:
		return t.applyDelete(job.cmd, job.screen)
	}
	return nil
}

func (t *Terminal) applyPlacement(job gfxJob) error {
	p := job.place
	if p.imageID == 0 {
		img := t.gfx.imageByNumber(job.screen, job.number)
		if img == nil {
			return errGraphicsNoImage
		}
		p.imageID = img.key.id
	}
	if t.gfx.image(job.screen, p.imageID) == nil {
		return errGraphicsNoImage
	}
	t.gfx.place(p)
	t.gfxAdvance(job.cmd, *p)
	return nil
}

func (t *Terminal) applyDelete(cmd kittygfx.Command, s screen) error {
	// The supported subset is a/A (every placement on this screen) and
	// i/I/n/N (one image, by id or by number, optionally one placement of it).
	// Lowercase selectors keep the image data; uppercase frees it once no
	// placement uses it. Everything else, including the positional and
	// placement-id selectors, is refused rather than half-implemented.
	switch cmd.Delete {
	case 'a':
		t.gfx.deleteOn(s)
		return nil
	case 'A':
		t.gfx.dropScreen(s)
		return nil
	case 'i', 'I', 'n', 'N':
	default:
		return errGraphicsDelete
	}
	byNumber := cmd.Delete == 'n' || cmd.Delete == 'N'
	id := cmd.ID
	if byNumber {
		img := t.gfx.imageByNumber(s, cmd.Number)
		if img == nil {
			return errGraphicsNoImage
		}
		id = img.key.id
	} else if id == 0 {
		return errGraphicsNeedID
	}
	if cmd.PlacementID != 0 {
		t.gfx.deletePlacement(s, id, cmd.PlacementID)
	} else {
		t.gfx.deletePlacementsOf(s, id)
	}
	if cmd.Delete == 'I' || cmd.Delete == 'N' {
		t.gfx.freeIfUnplaced(s, id)
	}
	return nil
}

// validateQuery answers a=q. A query is a capability probe: it may carry a
// throwaway image, which is validated and discarded, and it never looks an
// existing image up.
func validateQuery(job gfxJob, limits kittygfx.Limits) error {
	if job.cmd.Width <= 0 || job.cmd.Height <= 0 || len(job.data) == 0 {
		return nil
	}
	probe := job.cmd
	probe.Action = kittygfx.ActionTransmit
	_, err := kittygfx.Decode(probe, job.data, limits)
	return err
}

// dropGraphics discards queued work and every stored image, as RIS, Close and
// disabling graphics do.
func (t *Terminal) dropGraphics() {
	t.gfxJobs = nil
	t.gfx.clear()
}

// closeGraphics drops the store and refuses graphics from then on. Bytes that
// are still buffered for a closed terminal cannot create state again.
func (t *Terminal) closeGraphics() {
	t.dropGraphics()
	t.gfx.enabled = false
	t.gfx.closed = true
}
