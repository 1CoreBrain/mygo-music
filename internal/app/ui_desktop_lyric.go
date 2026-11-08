package app

import (
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-music/internal/lyrics"
)

// desktopLyric is the lyric strip over the desktop: a transparent, always-on-top
// window showing the line being sung and the line after it, one row above the
// other, with the rows taking turns as the song goes on. Its lock, at its top in
// the middle, lets the pointer through it to what is under it; resting the
// pointer on the strip brings its panel, its lock and its corner out. Dragging
// that corner makes the strip, and its lyrics, bigger.
type desktopLyric struct {
	a   *app
	win *mygo.Window

	// locked is the lock as last applied. While it is on, the strip takes none
	// of the pointer but the unlock button it shows under it.
	locked bool
	// hover is whether the pointer rests on the strip, which brings its panel,
	// its lock and its corner out.
	hover bool
	// take is whether the pointer rests on that button, so the strip takes the
	// pointer for it; anywhere else the clicks go through.
	take bool
	// justLocked holds the unlock button back until the pointer has left the
	// strip, so that the click locking it does not offer to unlock it at once.
	justLocked bool
	// pressing is the unlock button being pressed, whose release the strip must
	// take to the end.
	pressing bool
	polling  bool

	// resizing is a drag of the strip's corner, from the size resizeFrom, at
	// resizeAt of the screen.
	resizing   bool
	resizeFrom mygo.Size
	resizeAt   mygo.Point

	// size is the size the strip's content takes. A drag shrinks it at once,
	// while the window waits, as shown: it may be bigger than the strip, never
	// smaller.
	size mygo.Size
	// settle counts the polls the window has stayed bigger than the strip,
	// which it shrinks after, once a frame shows the smaller size painted.
	settle int
}

const (
	desktopLyricW = 760
	desktopLyricH = 160

	// The smallest and largest the strip, and so its lyrics, may be dragged.
	lyricMinW = 280
	lyricMinH = 90
	lyricMaxW = 2400
	lyricMaxH = 900

	// lyricSettlePolls is how many polls the window waits before it shrinks onto
	// a smaller strip: mygo paints the frame it painted last while a window
	// resizes, so the strip must be painted at its size before the window takes
	// it, or the plate's corners show cut square.
	lyricSettlePolls = 2
)

func newDesktopLyric(a *app) *desktopLyric {
	d := &desktopLyric{a: a, locked: a.settings.DesktopLyricsLocked}
	x, y := desktopLyricPlace()
	d.win = mygo.NewWindow(mygo.WindowOptions{
		Title: T("Desktop Lyrics"),
		// A panel floats over other apps, full screens included, and stays
		// out of the Dock and the app switcher (macOS).
		Panel:         runtime.GOOS == "darwin",
		AlwaysOnTop:   runtime.GOOS != "darwin",
		SkipTaskbar:   runtime.GOOS != "darwin",
		Hidden:        true,
		Frameless:     true,
		Transparent:   true,
		DisableShadow: true,
		DisableResize: true,
		X:             x,
		Y:             y,
		Width:         desktopLyricW,
		Height:        desktopLyricH,
		// Where the listener left it, and how big.
		StateKey: "desktop-lyrics",
		Content:  ui.View(d.view),
	})
	// Escape and the like close a panel: this one stays, until the app quits.
	d.win.OnClose(func(e *mygo.CloseEvent) {
		if !a.quitting {
			e.PreventDefault()
		}
	})
	// It starts out of the pointer's way, until one rests on it.
	d.win.SetIgnoreMouseEvents(true)
	// The size the listener left it at is the size the strip shows.
	if w, h := d.win.Size(); w > 0 && h > 0 {
		d.size = mygo.Size{Width: w, Height: h}
	}
	if a.settings.DesktopLyrics {
		d.win.ShowInactive()
		d.poll()
	}
	return d
}

// desktopLyricPlace puts the strip near the bottom of the primary display, where
// songs are read, until the listener moves it there.
func desktopLyricPlace() (int, int) {
	b := mygo.Screen.PrimaryDisplay().Bounds
	return int(b.X) + (int(b.Width)-desktopLyricW)/2, int(b.Y) + int(b.Height) - desktopLyricH - 96
}

// setEnabled shows the strip, or hides it, as the settings say.
func (d *desktopLyric) setEnabled(on bool) {
	if d.win == nil {
		return // as in tests
	}
	if on == d.win.IsVisible() {
		d.win.Invalidate()
		return
	}
	if on {
		d.win.ShowInactive()
		d.hover, d.take = false, false
		d.win.SetIgnoreMouseEvents(true)
		d.win.Invalidate()
		d.poll()
		return
	}
	d.hover, d.take = false, false
	d.win.Hide()
}

// setLocked lets the pointer through the strip to what is under it, or takes it
// back, as the settings say: either way, only where the pointer rests on it.
func (d *desktopLyric) setLocked(on bool) {
	if d.win == nil {
		return // as in tests
	}
	d.locked = on
	d.hover, d.take, d.pressing, d.resizing = false, false, false, false
	if !on {
		d.justLocked = false
	}
	d.win.SetIgnoreMouseEvents(true)
	d.win.Invalidate()
	d.poll()
}

// scale is how much larger the strip's lyrics are than at its start, from how
// tall the strip shows: the pointer drags its corner to make it bigger.
func (d *desktopLyric) scale() float32 {
	return lyricScale(float32(d.stageSize().Height))
}

// stageSize is the size the strip's content takes: the one it was dragged to,
// which the window may still be larger than.
func (d *desktopLyric) stageSize() mygo.Size {
	if d.size.Width > 0 && d.size.Height > 0 {
		return d.size
	}
	w, h := desktopLyricW, desktopLyricH
	if d.win != nil {
		if ww, hh := d.win.Size(); ww > 0 && hh > 0 {
			w, h = ww, hh
		}
	}
	return mygo.Size{Width: w, Height: h}
}

// lyricScale is how much larger the strip's lyrics are than at its start, by how
// tall the strip is: dragging its corner scales them.
func lyricScale(height float32) float32 {
	return min(max(height/desktopLyricH, 0.75), 3)
}

// lyricResized is the size the strip takes as its corner is dragged from a strip
// of size from by dx across and dy down, within the sizes it may take.
func lyricResized(from mygo.Size, dx, dy, minW, minH, maxW, maxH int) mygo.Size {
	return mygo.Size{
		Width:  min(max(from.Width+dx, minW), maxW),
		Height: min(max(from.Height+dy, minH), maxH),
	}
}

// lyricLimits are the smallest and largest the strip may be dragged to, the
// display's own size the largest.
func lyricLimits() (minW, minH, maxW, maxH int) {
	maxW, maxH = lyricMaxW, lyricMaxH
	if b := mygo.Screen.PrimaryDisplay().Bounds; b.Width > 0 && b.Height > 0 {
		maxW, maxH = max(min(maxW, int(b.Width)), lyricMinW), max(min(maxH, int(b.Height)), lyricMinH)
	}
	return lyricMinW, lyricMinH, maxW, maxH
}

// lockSize is the lock button's size, by how large the strip is.
func lockSize(scale float32) float32 { return min(max(26*scale, 22), 40) }

// lockTop is how far the lock button sits below the strip's top edge.
func lockTop(scale float32) float32 { return 8 * scale }

// lyricLockRect is where the lock sits in a strip whose top left corner is at x,
// y, and whose width is w: at its top, in its middle. The strip places it there
// and takes the pointer for it there, so that the two cannot drift apart.
func lyricLockRect(x, y, w, scale float32) (lx, ly, size float32) {
	size = lockSize(scale)
	return x + (w-size)/2, y + lockTop(scale), size
}

// desktopLyricLines is what the strip shows at cur: the line sung, by its index,
// and the line after it. Before the first line, the first is what comes.
func desktopLyricLines(lines []lyrics.Line, cur int) (sung lyrics.Line, next string) {
	if cur < 0 || cur >= len(lines) {
		if len(lines) > 0 {
			next = lines[0].Text
		}
		return lyrics.Line{}, next
	}
	if cur+1 < len(lines) {
		next = lines[cur+1].Text
	}
	return lines[cur], next
}

// lyricWordLit is how lit a word of the line sung is at pos, from 0 before it is
// sung to 1 by its end: the words light up one by one as they are sung.
func lyricWordLit(w lyrics.Word, pos time.Duration) float32 {
	switch {
	case pos >= w.End:
		return 1
	case pos <= w.Start:
		return 0
	}
	return clamp01(float32((pos - w.Start).Seconds() / max(0.05, (w.End-w.Start).Seconds())))
}

func (d *desktopLyric) view(c *ui.Context) {
	a := d.a
	pl := a.pl
	lines, loaded := pl.Lyrics()
	pos := pl.Position()
	cur := lyrics.Index(lines, pos)
	// Redraw as the next line begins, and, while the lyrics are on their way
	// from the source, until they are here.
	switch {
	case pl.Playing() && cur+1 < len(lines):
		c.After(lines[cur+1].At - pos + 20*time.Millisecond)
	case pl.Playing() && !loaded:
		c.After(300 * time.Millisecond)
	}
	c.Root().Background(ui.Transparent)
	sung, next := desktopLyricLines(lines, cur)
	locked := d.locked
	lang := lyrics.Lang(lines)
	size := d.stageSize()
	scale := d.scale()
	// The strip takes its own size, which the window may be larger than while a
	// drag shrinks it: mygo paints the frame it painted last while a window
	// resizes, so a plate drawn past the window's edge would show its corners
	// cut square.
	stage := ui.Box(c).Key("stage").Absolute().Top(0).Left(0).
		Size(float32(size.Width), float32(size.Height))
	stage.Children(func() {
		col := ui.Column(c).Fill().Justify(ui.Center).AlignItems(ui.Center).Gap(5 * scale)
		if !locked {
			col.DragWindow()
			// The panel under the lyrics, while the pointer rests on the strip,
			// as NetEase's is: a rounded, dark, see-through plate that fades in
			// under the words and away as the pointer leaves. It is the
			// column's own background, which it paints before its words.
			shown := float32(0)
			if d.hover {
				shown = 1
			}
			fade := col.AnimateTo("panel", shown, ui.Animation{Duration: 140 * time.Millisecond})
			if fade > 0.01 {
				col.Draw(func(p *ui.Painter, r ui.Rect) {
					radius := min(20*scale, min(r.W, r.H)/2)
					p.Fill(r, ui.RGBA(10, 10, 12, 0.5*fade), radius)
					p.Stroke(r, ui.RGBA(255, 255, 255, 0.1*fade), radius, 1)
				})
			}
		}
		col.Children(func() {
			if sung.Text == "" && next == "" {
				ui.Text(c, T("Desktop Lyrics")).FontSize(20*scale).FontWeight(700).TextColor(white.Alpha(0.5)).
					TextShadow(0, 2, 8, ui.RGBA(0, 0, 0, 0.55))
				return
			}
			// The lines take the strip's two rows in turn: the one coming up
			// stays where it is and lights up there, and the row the last line
			// left takes the line after it, so that none jumps.
			bottom := lyricBottom(cur)
			if bottom && next != "" {
				desktopLyricNext(c, next, scale)
			}
			if sung.Text != "" {
				desktopLyricSung(c, sung, pos, scale, lang, pl.Playing())
			}
			if sung.Translation != "" {
				desktopLyricTranslation(c, sung.Translation, scale)
			}
			if !bottom && next != "" {
				desktopLyricNext(c, next, scale)
			}
		})
		d.lockButton(c, locked, scale)
		if !locked && d.hover {
			d.resizeGrip(c, scale)
		}
	})
}

// lyricBottom reports whether the line sung takes the strip's lower row of the
// two, the row it waited in as it came up: the lines take them in turn.
func lyricBottom(cur int) bool {
	return cur > 0 && cur%2 == 1
}

// desktopLyricSung is the line being sung: it lights up word by word where the
// lyrics carry the words' own times, as the app's lyrics do.
func desktopLyricSung(c *ui.Context, l lyrics.Line, pos time.Duration, scale float32, lang string, playing bool) {
	if len(l.Words) > 0 {
		ui.RichText(c.Key("words")).Lang(lang).FontSize(30 * scale).FontWeight(800).SingleLine().
			Children(func() {
				for wi, wd := range l.Words {
					ui.Text(c.Key(wi), wd.Text).
						TextColor(white.Alpha(0.38+0.62*lyricWordLit(wd, pos))).
						TextShadow(0, 2, 10, ui.RGBA(0, 0, 0, 0.6))
				}
			})
		if playing {
			// Words brighten smoothly enough at 25 frames a second, as the app's
			// lyrics do.
			c.After(40 * time.Millisecond)
		}
		return
	}
	ui.Text(c, l.Text).Lang(lang).FontSize(30*scale).FontWeight(800).TextColor(white).SingleLine().
		TextShadow(0, 2, 10, ui.RGBA(0, 0, 0, 0.6))
}

// desktopLyricNext is the line after the one sung, dim, waiting in its row.
func desktopLyricNext(c *ui.Context, next string, scale float32) {
	ui.Text(c, next).FontSize(19*scale).TextColor(white.Alpha(0.5)).SingleLine().
		TextShadow(0, 2, 8, ui.RGBA(0, 0, 0, 0.5))
}

// desktopLyricTranslation is the line sung's translation, under it.
func desktopLyricTranslation(c *ui.Context, translation string, scale float32) {
	ui.Text(c, translation).FontSize(18*scale).FontWeight(600).TextColor(white.Alpha(0.8)).SingleLine().
		TextShadow(0, 2, 8, ui.RGBA(0, 0, 0, 0.55))
}

// lockButton is the lock, at the strip's top in its middle: unlocked, it locks
// the strip; locked, the pointer's rest on the strip brings it out to unlock it
// again. Either way it shows only while the pointer rests on the strip.
func (d *desktopLyric) lockButton(c *ui.Context, locked bool, scale float32) {
	if !d.hover || (locked && d.justLocked) {
		d.pressing = false
		return
	}
	lx, ly, lw := lyricLockRect(0, 0, float32(d.stageSize().Width), scale)
	icon, label := iconLock, T("Lock Desktop Lyrics")
	if locked {
		icon, label = iconUnlock, T("Unlock Desktop Lyrics")
	}
	b := ui.ButtonBase(c.Key("lock")).Absolute().Left(lx).Top(ly).Size(lw, lw).Radius(lw * 0.28).
		FocusRing(false).Background(ui.RGBA(255, 255, 255, 0.14)).Label(label).Cursor(ui.CursorPointer)
	if locked && d.take {
		b.Background(ui.RGBA(255, 255, 255, 0.24))
	}
	b.Children(func() { ui.Icon(c, icon).Size(lw*0.52, lw*0.52).TextColor(white.Alpha(0.85)) })
	// Wherever the pointer goes while the button is pressed, as a click
	// that drifts off it is still its own.
	if button, held := b.PressedButton(); held && button == 0 {
		d.pressing = true
	} else {
		d.pressing = false
	}
	if b.Clicked() {
		if locked {
			d.a.settings.DesktopLyricsLocked = false
		} else {
			d.a.settings.DesktopLyricsLocked = true
			d.justLocked = true
		}
		d.a.changed()
	}
}

// resizeGrip is the strip's corner, toward which the pointer drags it bigger or
// smaller, its lyrics going with it: a small triangle, as a grip's. It takes the
// press itself, so that every move takes the strip to its new size at once: the
// corner is where the pointer stands, which the view's own frames do not follow,
// as mygo rebuilds a view on moves only while the pointer is within the window.
func (d *desktopLyric) resizeGrip(c *ui.Context, scale float32) {
	g := min(max(22*scale, 16), 32)
	b := ui.ButtonBase(c.Key("resize")).Absolute().Right(scale).Bottom(scale).Size(g, g).
		FocusRing(false).Cursor(ui.CursorResizeNWSE).Label(T("Resize Desktop Lyrics"))
	ink := white.Alpha(0.45)
	if b.Hovered() || d.resizing {
		ink = white.Alpha(0.9)
	}
	b.Draw(func(p *ui.Painter, r ui.Rect) {
		// A small triangle in the corner, as a grip's, pointing in at the
		// strip's middle.
		k := 12 * scale
		tri := &ui.Path{}
		tri.MoveTo(r.X+r.W-k, r.Y+r.H-k)
		tri.LineTo(r.X+r.W-2*scale, r.Y+r.H-k)
		tri.LineTo(r.X+r.W-k, r.Y+r.H-2*scale)
		tri.Close()
		p.FillPath(tri, ink)
	})
	b.HandleInput(func(ev ui.InputEvent) bool {
		switch {
		case ev.Kind == ui.InputPointerDown:
			if ev.Button != 0 {
				return false
			}
			d.beginResize()
			return true
		case ev.Kind == ui.InputPointerUp:
			d.endResize()
			return true
		case d.dragsCorner(ev):
			d.resizeStep()
			return true
		}
		return false
	})
}

// dragsCorner reports whether ev is a move of a press dragging the strip's
// corner. The pointer's moves come to what it rests on too, with no button, and
// those drag nothing: the corner follows a press alone.
func (d *desktopLyric) dragsCorner(ev ui.InputEvent) bool {
	return ev.Kind == ui.InputPointerMove && ev.Button == 0 && d.resizing
}

// beginResize starts a drag of the strip's corner, from the size and place it
// has as the pointer presses it.
func (d *desktopLyric) beginResize() {
	d.resizing = true
	d.resizeFrom = d.stageSize()
	d.resizeAt = cursorPoint()
}

// resizeStep takes the strip to the size the pointer's place gives its corner,
// as it is dragged.
func (d *desktopLyric) resizeStep() {
	if d.win == nil {
		return // as in tests
	}
	at := cursorPoint()
	minW, minH, maxW, maxH := lyricLimits()
	d.setSize(lyricResized(d.resizeFrom, at.X-d.resizeAt.X, at.Y-d.resizeAt.Y, minW, minH, maxW, maxH))
}

// setSize takes the strip to s: its content at once, and its window at once
// too where it must grow, as a window bigger than its content shows nothing
// past it.
func (d *desktopLyric) setSize(s mygo.Size) {
	d.size = s
	if d.win == nil {
		return // as in tests
	}
	if w, h := d.win.Size(); s.Width > w || s.Height > h {
		d.win.SetSize(s.Width, s.Height)
	}
	d.win.Invalidate()
}

// catchUp takes the window to the size the strip shows, once the pointer has let
// its corner go and a frame of the smaller size has been painted: mygo paints
// the frame it painted last while a window resizes, so a window shrunk onto a
// plate it no longer holds would cut the plate's corners square.
func (d *desktopLyric) catchUp() {
	if d.win == nil || d.resizing || d.size.Width <= 0 || d.size.Height <= 0 {
		d.settle = 0
		return
	}
	w, h := d.win.Size()
	switch {
	case w == d.size.Width && h == d.size.Height:
		d.settle = 0
	case w < d.size.Width || h < d.size.Height:
		// The content past the window would be lost: grow now.
		d.settle = 0
		d.win.SetSize(d.size.Width, d.size.Height)
		d.win.Invalidate()
	default:
		if d.settle++; d.settle < lyricSettlePolls {
			return
		}
		d.settle = 0
		d.win.SetSize(d.size.Width, d.size.Height)
		d.win.Invalidate()
	}
}

// endResize ends a drag of the strip's corner, the size it took kept.
func (d *desktopLyric) endResize() {
	if !d.resizing {
		return
	}
	d.resizing = false
	if d.win != nil {
		d.win.Invalidate()
	}
}

// poll follows the pointer while the strip shows, as a still one gets none of
// its moves: resting on the strip brings its panel, its lock and its corner
// out, and it takes the pointer then, and, locked, only for the unlock button
// it shows.
func (d *desktopLyric) poll() {
	if d.win == nil || d.a.quitting || d.win.IsDestroyed() || !d.win.IsVisible() {
		d.polling = false
		return
	}
	b := d.win.Bounds()
	size := d.stageSize()
	// The strip's own rectangle, which its window may be larger than, as it
	// catches up with it.
	b.Width, b.Height = size.Width, size.Height
	p := cursorPoint()
	slack := 4
	if d.hover {
		slack = 8
	}
	// A press drags the strip, or its corner, past its edges: it stays hovered
	// until the pointer lets go.
	inside := d.pressing || d.resizing ||
		p.X >= b.X-slack && p.X <= b.X+b.Width+slack && p.Y >= b.Y-slack && p.Y <= b.Y+b.Height+slack
	if !inside {
		d.justLocked = false
	}
	hover := inside && !(d.locked && d.justLocked)
	if hover != d.hover {
		d.hover = hover
		d.win.Invalidate()
	}
	take := false
	if hover {
		if d.locked {
			// The lock's own rect, as the strip draws it.
			lx, ly, lw := lyricLockRect(float32(b.X), float32(b.Y), float32(b.Width), d.scale())
			x, y := float32(p.X), float32(p.Y)
			take = x >= lx-6 && x <= lx+lw+6 && y >= ly-6 && y <= ly+lw+6
		} else {
			take = true
		}
	}
	// A click in progress is not to be cut short by the strip letting the
	// pointer go under it.
	if take != d.take && !d.pressing && !d.resizing {
		d.take = take
		d.win.SetIgnoreMouseEvents(!take)
	}
	// The window takes the size the strip shows, now that the pointer has let
	// its corner go and a frame of that size is painted.
	d.catchUp()
	wait := 60 * time.Millisecond
	if hover {
		wait = 25 * time.Millisecond
	}
	d.pollAgain(wait)
}

// pollAgain polls again in wait, unless a poll is already on its way.
func (d *desktopLyric) pollAgain(wait time.Duration) {
	if d.polling {
		return
	}
	d.polling = true
	time.AfterFunc(wait, func() {
		mygo.RunOnMain(func() {
			d.polling = false
			d.poll()
		})
	})
}

// toggleDesktopLyrics shows or hides the strip.
func (a *app) toggleDesktopLyrics() {
	a.settings.DesktopLyrics = !a.settings.DesktopLyrics
	a.changed()
}

// toggleDesktopLyricLock lets the pointer through the strip, or takes it back.
func (a *app) toggleDesktopLyricLock() {
	a.settings.DesktopLyricsLocked = !a.settings.DesktopLyricsLocked
	if !a.settings.DesktopLyricsLocked && a.desktopLyric != nil {
		a.desktopLyric.justLocked = false
	}
	a.changed()
}
