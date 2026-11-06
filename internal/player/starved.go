package player

import (
	"sync/atomic"
	"time"
)

// starvedAfter is how long the playing stream may wait for its download before
// the player says it's buffering: decoding takes milliseconds, and it's asked
// for more only as there's room for it, so a call this long waits on bytes
// that haven't come in, after a seek past them, or from a network slower
// than the song.
const starvedAfter = 250 * time.Millisecond

// watched is the stream that plays, timing what it's asked.
type watched struct {
	stream
	now func() time.Time
	// ended hears that a call that took starvedAfter or longer returned,
	// for what said it's buffering to say so no more.
	ended func()
	since atomic.Int64 // when the call on its way began, in Unix nanoseconds, or 0
}

func (w *watched) Read(dst []float32) (int, error) {
	defer w.begin()()
	return w.stream.Read(dst)
}

func (w *watched) SetPosition(frame int64) error {
	defer w.begin()()
	return w.stream.SetPosition(frame)
}

func (w *watched) begin() func() {
	start := w.now()
	w.since.Store(start.UnixNano())
	return func() {
		w.since.Store(0)
		if w.ended != nil && w.now().Sub(start) >= starvedAfter {
			w.ended()
		}
	}
}

// starved reports whether the call on its way has waited starvedAfter.
func (w *watched) starved() bool {
	since := w.since.Load()
	return since != 0 && w.now().UnixNano()-since >= int64(starvedAfter)
}
