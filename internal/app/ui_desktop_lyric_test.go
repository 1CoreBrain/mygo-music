package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-music/internal/lyrics"
)

// TestLyricBottom checks that the lines take the strip's two rows in turn, so
// that the one coming up stays where it is as it lights up.
func TestLyricBottom(t *testing.T) {
	for _, c := range []struct {
		cur  int
		want bool
	}{
		{-1, false},
		{0, false},
		{1, true},
		{2, false},
		{3, true},
	} {
		if got := lyricBottom(c.cur); got != c.want {
			t.Errorf("line %d: lower row %v, want %v", c.cur, got, c.want)
		}
	}
}

// TestDesktopLyricLines checks what the strip shows before the first line, as
// one is sung, and on the last.
func TestDesktopLyricLines(t *testing.T) {
	lines := []lyrics.Line{
		{At: time.Second, Text: "one", Translation: "the first"},
		{At: 2 * time.Second, Text: "two"},
		{At: 3 * time.Second, Text: "three"},
	}
	tests := []struct {
		name              string
		cur               int
		text, translation string
		next              string
	}{
		{"before the first", -1, "", "", "one"},
		{"on the first", 0, "one", "the first", "two"},
		{"on the second", 1, "two", "", "three"},
		{"on the last", 2, "three", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sung, next := desktopLyricLines(lines, tt.cur)
			if sung.Text != tt.text || sung.Translation != tt.translation || next != tt.next {
				t.Fatalf("line %d: got %q, %q, %q; want %q, %q, %q", tt.cur, sung.Text, sung.Translation, next, tt.text, tt.translation, tt.next)
			}
		})
	}
	// A song without lyrics shows nothing, and is not told it is between lines.
	if sung, next := desktopLyricLines(nil, 0); sung.Text != "" || sung.Translation != "" || next != "" {
		t.Fatalf("no lyrics: got %q, %q, %q", sung.Text, sung.Translation, next)
	}
}

// TestDesktopLyricLock checks that the lock shows only while the pointer rests
// on the strip, that the button at the top locks it, and that the one the
// pointer brings out while it is locked unlocks it again.
func TestDesktopLyricLock(t *testing.T) {
	a := newTestApp()
	d := &desktopLyric{a: a}
	tt := ui.NewTester(d.view, desktopLyricW, desktopLyricH)

	// The pointer away, the strip is bare: no lock, as no panel.
	d.hover = false
	tt.Frame()
	if tt.HasText(T("Lock Desktop Lyrics")) {
		t.Fatal("the lock showed while the pointer was away")
	}

	// Resting the pointer on it brings the lock out.
	d.hover = true
	tt.Frame()
	if err := tt.Click(T("Lock Desktop Lyrics")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.settings.DesktopLyricsLocked {
		t.Fatal("the lock button did not lock the strip")
	}

	// The click locking it does not offer to unlock it again at once, while
	// the pointer is still on it.
	d.locked, d.hover = true, true
	tt.Frame()
	if tt.HasText(T("Unlock Desktop Lyrics")) {
		t.Fatal("the unlock button showed at once, under the click that locked the strip")
	}

	// Locked, the strip shows the unlock button while the pointer rests on it.
	d.justLocked = false
	tt.Frame()
	if err := tt.Click(T("Unlock Desktop Lyrics")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.settings.DesktopLyricsLocked {
		t.Fatal("the unlock button did not unlock the strip")
	}
}

// TestDesktopLyricResizeDrag checks that dragging the strip's corner resizes
// it: the pointer leaves the grip at once, as the corner moves with it, and the
// press must go on wherever the pointer goes.
func TestDesktopLyricResizeDrag(t *testing.T) {
	a := newTestApp()
	d := &desktopLyric{a: a, hover: true}
	at := mygo.Point{X: 100, Y: 100}
	was := cursorPoint
	cursorPoint = func() mygo.Point { return at }
	defer func() { cursorPoint = was }()

	tt := ui.NewTester(d.view, desktopLyricW, desktopLyricH)
	tt.Frame()
	corner, ok := tt.Find(T("Resize Desktop Lyrics"))
	if !ok {
		t.Fatal("the corner grip shows while the pointer rests on the strip")
	}
	x, y := corner.X+corner.W/2, corner.Y+corner.H/2

	tt.Press(x, y)
	tt.Frame()
	if !d.resizing {
		t.Fatal("the press on the corner did not start a resize")
	}
	if want := (mygo.Size{Width: desktopLyricW, Height: desktopLyricH}); d.resizeFrom != want {
		t.Fatalf("resizing from %v; want %v", d.resizeFrom, want)
	}

	// The pointer leaves the grip at once, dragging the strip's corner.
	at = mygo.Point{X: 160, Y: 140}
	tt.Move(x+60, y+40)
	tt.Frame()
	if !d.resizing {
		t.Fatal("the resize ended as the pointer left the grip")
	}

	tt.Release(x+60, y+40)
	tt.Frame()
	if d.resizing {
		t.Fatal("the resize went on after the pointer let go")
	}
}

// TestDesktopLyricCornerDragsCorner checks that only the moves of a press drag
// the corner: the pointer's moves over the resting strip come with no button,
// and once resized the strip to its largest on a hover alone.
func TestDesktopLyricCornerDragsCorner(t *testing.T) {
	d := &desktopLyric{}
	hover := ui.InputEvent{Kind: ui.InputPointerMove, Button: -1}
	if d.dragsCorner(hover) {
		t.Fatal("a move with no button dragged the corner")
	}
	pressed := ui.InputEvent{Kind: ui.InputPointerMove, Button: 0}
	if d.dragsCorner(pressed) {
		t.Fatal("a move outside a press dragged the corner")
	}
	d.resizing = true
	if !d.dragsCorner(pressed) {
		t.Fatal("a move of the press did not drag the corner")
	}
	if d.dragsCorner(hover) {
		t.Fatal("a move with no button dragged the corner while resizing")
	}
}

// TestLyricWordLit checks how a word of the line sung lights up as it goes: dark
// before it is sung, lit by its end, and lit after it.
func TestLyricWordLit(t *testing.T) {
	w := lyrics.Word{Text: "green", Start: 2 * time.Second, End: 3 * time.Second}
	for _, c := range []struct {
		at   time.Duration
		want float32
	}{
		{time.Second, 0},
		{2 * time.Second, 0},
		{2500 * time.Millisecond, 0.5},
		{3 * time.Second, 1},
		{4 * time.Second, 1},
	} {
		if got := lyricWordLit(w, c.at); got < c.want-0.01 || got > c.want+0.01 {
			t.Errorf("at %v: lit %v, want %v", c.at, got, c.want)
		}
	}
	// A word with no length at all is sung as it starts.
	if got := lyricWordLit(lyrics.Word{Text: "a", Start: time.Second, End: time.Second}, time.Second); got != 1 {
		t.Errorf("a word of no length: lit %v, want 1", got)
	}
}

// TestLyricResized checks that dragging the strip's corner resizes it within
// what keeps its lyrics readable and on the screen.
func TestLyricResized(t *testing.T) {
	from := mygo.Size{Width: desktopLyricW, Height: desktopLyricH}
	tests := []struct {
		name   string
		dx, dy int
		want   mygo.Size
	}{
		{"bigger", 120, 40, mygo.Size{Width: desktopLyricW + 120, Height: desktopLyricH + 40}},
		{"smaller", -60, -20, mygo.Size{Width: desktopLyricW - 60, Height: desktopLyricH - 20}},
		{"past the smallest", -5000, -5000, mygo.Size{Width: lyricMinW, Height: lyricMinH}},
		{"past the largest", 5000, 5000, mygo.Size{Width: lyricMaxW, Height: lyricMaxH}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lyricResized(from, tt.dx, tt.dy, lyricMinW, lyricMinH, lyricMaxW, lyricMaxH)
			if got != tt.want {
				t.Fatalf("dragged by %d, %d: got %v; want %v", tt.dx, tt.dy, got, tt.want)
			}
		})
	}
}

// TestLyricScale checks that a taller strip draws larger lyrics, within what
// reads well at the sizes it may take.
func TestLyricScale(t *testing.T) {
	tests := []struct {
		height float32
		want   float32
	}{
		{desktopLyricH, 1},
		{2 * desktopLyricH, 2},
		{40, 0.75},
		{20 * desktopLyricH, 3},
	}
	for _, tt := range tests {
		if got := lyricScale(tt.height); got != tt.want {
			t.Errorf("at height %v: got %v; want %v", tt.height, got, tt.want)
		}
	}
}
