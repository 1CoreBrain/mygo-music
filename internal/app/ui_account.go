package app

import (
	"fmt"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-music/internal/music"
	"rsc.io/qr"
)

// mygoOpenFolders asks for folders of music with the native dialog.
func mygoOpenFolders(a *app) ([]string, error) {
	return mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:             T("Add a Music Folder"),
		ButtonLabel:       T("Add"),
		Directory:         true,
		Multiple:          true,
		CreateDirectories: false,
	})
}

// signInDialog is signing in to a source. The source does it its own way, in
// the listener's browser or wherever, which can take minutes.
type signInDialog struct {
	open    bool
	lib     *library
	waiting bool
	gen     int // counts the ones started, to tell the last
	err     string
	prompt  music.SignInPrompt // what the source wants shown
	qr      *qr.Code           // prompt's QR, encoded
}

// loadAccount asks a source who is signed in.
func (a *app) loadAccount(lib *library) {
	if !lib.can("account") || lib.accountBusy {
		return
	}
	lib.accountBusy = true
	go func() {
		acc, err := lib.src.Account()
		a.post(func() {
			lib.accountBusy = false
			if err != nil {
				a.problem(lib, err, T("Couldn't reach this plugin"))
				return
			}
			was := lib.account
			lib.account = acc
			if was != nil && acc == nil || was == nil && acc != nil {
				a.afterAccount(lib)
			}
		})
	}()
}

// afterAccount loads anew what's personal to who listens to a source.
func (a *app) afterAccount(lib *library) {
	lib.forget()
	lib.needsSignIn, lib.askedSignIn = false, false
	a.loadMine(lib)
	a.loadHome(lib)
	if a.route.src == lib.id {
		a.reload(a.route)
	}
}

// promptSignIn asks the listener to sign in to a source, showing the window it
// asks in. A source that has no accounts has nothing to ask.
func (a *app) promptSignIn(lib *library) {
	if !lib.can("signIn") {
		a.toast = Tf("%s isn't available right now", lib.name())
		return
	}
	a.showMain()
	a.signIn = signInDialog{open: true, lib: lib, gen: a.signIn.gen + 1}
}

// startSignIn signs in to the source asked about, off the main thread: it
// waits for the listener at the source. A sign-in that ends after they closed
// the dialog still counts.
func (a *app) startSignIn() {
	d := &a.signIn
	lib := d.lib
	if lib == nil || d.waiting {
		return
	}
	d.waiting, d.err = true, ""
	d.prompt, d.qr = music.SignInPrompt{}, nil
	gen := d.gen
	if p, ok := lib.src.(music.Prompter); ok {
		p.OnSignInPrompt(func(prompt music.SignInPrompt) {
			a.post(func() {
				if a.signIn.gen == gen {
					a.signInPrompted(prompt)
				}
			})
		})
	}
	go func() {
		acc, err := lib.src.SignIn()
		a.post(func() {
			current := a.signIn.gen == gen
			if current {
				a.signIn.waiting = false
			}
			if err != nil {
				log := err.Error()
				if current {
					a.signIn.err = log
				}
				return
			}
			if current {
				a.signIn.open = false
			}
			lib.account = acc
			a.afterAccount(lib)
			// Signed in to another source from the settings: that's the one they're
			// after. One finishing after they closed the dialog doesn't move them.
			if current && a.library != lib {
				a.switchSource(lib.id)
			}
		})
	}()
}

// signInPrompted shows what the source wants shown. A QR that can't be encoded
// leaves its text, which should say how else to go on.
func (a *app) signInPrompted(prompt music.SignInPrompt) {
	d := &a.signIn
	d.prompt, d.qr = prompt, nil
	if prompt.QR != "" {
		code, err := qr.Encode(prompt.QR, qr.M)
		if err != nil {
			d.err = err.Error()
			return
		}
		d.qr = code
	}
}

// signOut signs out of a source.
func (a *app) signOut(lib *library) {
	lib.account = nil
	a.afterAccount(lib)
	go func() {
		if err := lib.src.SignOut(); err != nil {
			a.post(func() { a.actionFailed(lib, err, T("Couldn't sign out")) })
		}
	}()
}

// signInDialog says which source the listener signs in to, while it waits for
// them there.
func (a *app) signInDialog(c *ui.Context) {
	p := a.pal
	d := &a.signIn
	ui.DialogBase(c.Key("sign-in"), &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(360).Padding(24).Radius(18).Gap(10).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.94}, Exit: &ui.Motion{Scale: 0.96}})
		name := ""
		if d.lib != nil {
			name = d.lib.name()
		}
		ui.Text(c, Tf("Sign in to %s", name)).FontSize(17).FontWeight(700)
		text := T("You'll finish signing in with the plugin, which may open your browser.")
		switch {
		case d.err != "":
			text = d.err
		case d.waiting && d.prompt.Text != "":
			text = d.prompt.Text
		case d.waiting:
			text = T("Waiting for you to finish signing in…")
		}
		ui.Text(c, text).FontSize(13).TextColor(p.muted).LineHeight(1.45)
		if code := d.qr; code != nil && d.waiting && d.err == "" {
			// On white whatever the appearance, for cameras to read it.
			ui.Row(c).Justify(ui.Center).Margin(6, 0, 2, 0).Children(func() {
				ui.Box(c).Size(208, 208).Radius(12).Background(ui.Hex("#ffffff")).
					Draw(func(pt *ui.Painter, r ui.Rect) { drawQR(pt, r, code, 14) })
			})
		}
		ui.Row(c).Gap(10).Justify(ui.End).Margin(8, 0, 0, 0).Children(func() {
			if pillButton(c.Key("cancel"), nil, T("Cancel"), p.hover.Over(p.bg), p.text).Clicked() {
				d.open = false
			}
			if !d.waiting && pillButton(c.Key("ok"), nil, T("Sign In"), p.accent, p.onAccent).Clicked() {
				a.startSignIn()
			}
		})
	})
}

// drawQR draws a QR code in r, as crisp squares inside a quiet zone of pad.
func drawQR(p *ui.Painter, r ui.Rect, code *qr.Code, pad float32) {
	n := code.Size
	cell := (min(r.W, r.H) - 2*pad) / float32(n)
	x0 := r.X + (r.W-cell*float32(n))/2
	y0 := r.Y + (r.H-cell*float32(n))/2
	ink := ui.Hex("#141416")
	for y := range n {
		for x := range n {
			if code.Black(x, y) {
				// A hair larger, so neighbors join without seams.
				p.Fill(ui.Rect{X: x0 + float32(x)*cell, Y: y0 + float32(y)*cell, W: cell + 0.3, H: cell + 0.3}, ink, 0)
			}
		}
	}
}

// accountRow sits at the foot of the sidebar: who is signed in to the source,
// with a dropdown of the settings and signing out; or a way to sign in, if it
// has accounts; or else the way to the settings.
func (a *app) accountRow(c *ui.Context) {
	p := a.pal
	lib := a.library
	b := ui.ButtonBase(c.Key("account")).Height(40).PaddingX(8).Gap(10).Radius(10).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false)
	if b.Hovered() || a.accountOpen && lib.account != nil {
		b.Background(p.hover)
	}
	if lib.account == nil {
		a.accountOpen = false
		if lib.can("signIn") {
			label := Tf("Sign in to %s", lib.name())
			b.Label(label).Children(func() {
				ui.Box(c).Size(26, 26).Radius(13).Background(p.press).Center().Children(func() {
					ui.Icon(c, iconUser).Size(14, 14).TextColor(p.muted)
				})
				ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(p.muted).SingleLine().Shrink(1).MinWidth(0)
			})
			if b.Clicked() {
				a.promptSignIn(lib)
			}
			return
		}
		b.Label(T("Settings")).Children(func() {
			ui.Box(c).Size(26, 26).Center().Children(func() {
				ui.Icon(c, iconSettings).Size(15, 15).TextColor(p.muted)
			})
			ui.Text(c, T("Settings")).FontSize(13).FontWeight(500).TextColor(p.muted)
		})
		if b.Clicked() {
			a.openSettings()
		}
		return
	}
	acc := lib.account
	b.Label(acc.Name).Expanded(a.accountOpen).Children(func() {
		a.avatar(c, acc.ImageURL, 26, p.press)
		ui.Text(c, acc.Name).FontSize(13).FontWeight(600).SingleLine().Grow(1).Shrink(1).MinWidth(0)
		ui.Icon(c, iconUpDown).Size(13, 13).TextColor(p.faint)
	})
	if b.Clicked() {
		a.accountOpen = !a.accountOpen
	}
	a.accountMenu(c, b)
}

// accountMenu drops up from the account row, as wide as it, in the colors
// of the volume popover: the settings, then signing out.
func (a *app) accountMenu(c *ui.Context, row ui.Element) {
	pop := popoverColors(a.pal.dark)
	ui.PopoverBase(c.Key("account-menu"), row, &a.accountOpen, func(panel ui.Element) {
		panel.AttachTo(row, ui.AnchorTopStart, ui.AnchorBottomStart).Margin(0, 0, 6, 0).Padding(5).Gap(1).
			Radius(12).Background(pop.fill).Border(0.5, pop.edge).Shadow(0, 8, 24, -4, ui.RGBA(0, 0, 0, 0.2)).
			Role(ui.RoleMenu).FocusGroup(ui.Vertical).Label(T("Account")).TransformOrigin(0.5, 1).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.96, Y: 4}, Exit: &ui.Motion{Scale: 0.98}})
		if w := row.Bounds().W; w > 0 {
			panel.Width(w)
		}
		shortcut := "Ctrl+,"
		if runtime.GOOS == "darwin" {
			shortcut = "⌘,"
		}
		if a.menuItem(c, iconSettings, T("Settings…"), shortcut).AutoFocus().Clicked() {
			a.accountOpen = false
			a.openSettings()
		}
		ui.Box(c).Height(0.5).Margin(4, 8).Background(pop.edge)
		if a.menuItem(c, iconSignOut, T("Sign Out"), "").Clicked() {
			a.accountOpen = false
			a.signOut(a.library)
		}
	})
}

// menuItem is an item of a dropdown: an icon, its label, and the shortcut
// that does the same, if any.
func (a *app) menuItem(c *ui.Context, icon *ui.SVG, label, shortcut string) ui.Element {
	p := a.pal
	b := ui.ButtonBase(c.Key(label)).Role(ui.RoleMenuItem).Label(label).Height(30).PaddingX(8).Gap(9).Radius(7).
		AlignItems(ui.Center).Justify(ui.Start).FocusRing(false)
	if b.Hovered() || b.FocusVisible() {
		b.Background(p.press)
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(15, 15).TextColor(p.muted)
		ui.Text(c, label).FontSize(13).Grow(1)
		if shortcut != "" {
			ui.Text(c, shortcut).FontSize(12).TextColor(p.faint)
		}
	})
	return b
}

// avatar shows a listener's picture, round, over empty while it loads.
func (a *app) avatar(c *ui.Context, url string, size float32, empty ui.Color) {
	bmp := bitmap(a.art.get(sized(url, int(size*3)), false))
	box := ui.Box(c).Size(size, size).Radius(size / 2).Shrink(0)
	in := artIn(box, bmp)
	if in < 1 {
		box.Background(empty)
	}
	box.Children(func() { artImage(c, bmp, size/2, in) })
}

// myPlaylists lists the listener's playlists under a header that opens
// and closes them: those they made, if mine, with a button to make one
// and a menu to rename them, or else with a menu to unsave them.
func (a *app) myPlaylists(c *ui.Context, title string, lists []music.Playlist, mine bool, collapsed *bool) {
	canCreate := mine && a.can("createPlaylist")
	if len(lists) == 0 && !canCreate {
		return
	}
	p := a.pal
	ui.Row(c.Key(fmt.Sprint("lists-", mine))).AlignItems(ui.Center).Margin(18, 4, 6, 6).Children(func() {
		b := ui.ButtonBase(c.Key("toggle")).Grow(1).Height(26).PaddingX(4).Gap(6).Radius(6).
			AlignItems(ui.Center).Justify(ui.Start).Role(ui.RoleDisclosure).Label(title)
		if b.Clicked() {
			*collapsed = !*collapsed
		}
		b.Expanded(!*collapsed)
		if b.Hovered() {
			b.Background(p.hover)
		}
		b.Children(func() {
			ui.Text(c, title).FontSize(11.5).FontWeight(600).TextColor(p.faint).Grow(1)
			arrow := iconDown
			if *collapsed {
				arrow = iconForward
			}
			ui.Icon(c, arrow).Size(12, 12).TextColor(p.faint)
		})
		if canCreate && iconButton(c.Key("new"), iconPlus, 13, p.faint, T("New Playlist")).Size(26, 26).Radius(6).Clicked() {
			a.newPlaylist(nil)
		}
	})
	if *collapsed {
		return
	}
	for i := range lists {
		pl := &lists[i]
		selected := a.route.page == pagePlaylist && a.route.id == pl.ID && !a.nowPlaying
		b := ui.ButtonBase(c.Key(pl.ID)).Height(34).PaddingX(8).Gap(10).Radius(8).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(pl.Name)
		switch {
		case selected:
			b.Background(p.press)
		case b.Hovered():
			b.Background(p.hover)
		}
		b.Children(func() {
			a.cover(c, pl.ImageURL, 80, 22, 5)
			ui.Text(c, pl.Name).FontSize(13).SingleLine().Shrink(1).MinWidth(0)
		})
		switch {
		case mine && (a.can("renamePlaylist") || a.can("deletePlaylist")):
			b.ContextMenu(func(m *ui.Menu) { a.playlistMenu(m, pl) })
		case !mine && a.can("savePlaylist"):
			b.ContextMenu(func(m *ui.Menu) {
				if m.Item(T("Remove from Saved")).Chosen() {
					a.toggleSaved(*pl)
				}
			})
		}
		if b.Clicked() {
			a.open(route{page: pagePlaylist, id: pl.ID})
		}
	}
}

// playlistMenu has what can be done with a playlist of the listener's, as far
// as the source can.
func (a *app) playlistMenu(m *ui.Menu, pl *music.Playlist) {
	if a.can("renamePlaylist") && m.Item(T("Rename…")).Chosen() {
		a.startRename(pl)
	}
	if a.can("deletePlaylist") {
		if a.can("renamePlaylist") {
			m.Separator()
		}
		if m.Item(T("Delete Playlist…")).Chosen() {
			a.confirmDelete(pl)
		}
	}
}
