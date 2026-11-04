package app

import (
	"runtime"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"mygo-music/internal/music"
)

const sidebarWidth = 232

func (a *app) view(c *ui.Context) {
	t := *c.Theme()
	a.pal = paletteFor(&t)
	p := a.pal
	t.Accent, t.AccentText = p.accent, p.onAccent
	t.Selection = p.accent.Alpha(0.3)
	c.SetTheme(&t)

	a.shortcuts(c)

	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	root := ui.Box(c).Fill()
	if !c.Focused(a.searchBox) {
		// Space plays and pauses wherever the focus is, as in Music. On
		// the container around everything, it comes before a button the
		// pointer focused, which would take Space as a click; on the
		// window, it would come after.
		root.OnShortcut(0, ui.KeySpace, a.pl.TogglePlay)
		// And where nothing has the focus, as once Now Playing covers the
		// button that had it.
		c.OnShortcut(0, ui.KeySpace, a.pl.TogglePlay)
	}
	npOpen, npSettled := a.npState(c)
	root.Children(func() {
		page := ui.Row(c).Fill().AlignItems(ui.Stretch)
		page.Children(func() {
			if npOpen && npSettled {
				// Covered by Now Playing: not built, which would cost each
				// of its frames as much again as the stage itself. Where
				// it is scrolled is the app's (pageScroll, navScroll,
				// trackList), so it shows again as it was.
				return
			}
			side := ui.Column(c).Width(sidebarWidth)
			if !c.Vibrancy() {
				side.Background(p.sidebar)
			}
			side.Children(func() { a.sidebar(c) })
			// Keyed, so that a popover of the sidebar opening or closing
			// keeps what the page and its bars hold, their edges' fades.
			ui.Box(c.Key("main")).Grow(1).Background(p.bg).BorderWidth(0, 0, 0, 1).BorderColor(p.hairline).Children(func() {
				a.content(c)
				// The bars float over the page, which scrolls under them:
				// positioned, they paint over it.
				a.pageTop(c)
				a.playerBar(c)
			})
		})
		a.nowPlayingLayer(c, npOpen, npSettled)
	})
	a.signInDialog(c)
	a.playlistNameDialog(c)
	a.deletePlaylistDialog(c)
	if a.toast != "" {
		c.Toast(a.toast)
		a.toast = ""
	}
	a.askLikes()
	a.syncWindows()
}

func (a *app) shortcuts(c *ui.Context) {
	// The side buttons of a mouse, the keys of keyboards that have them
	// and swipes between pages, and the keys of Music.
	c.OnShortcut(0, ui.KeyBack, a.back)
	c.OnShortcut(0, ui.KeyForward, a.goForward)
	c.OnShortcut(ui.Cmd, ui.KeyBracketLeft, a.back)
	c.OnShortcut(ui.Cmd, ui.KeyBracketRight, a.goForward)
	c.OnShortcut(ui.Cmd, ui.KeyF, func() {
		a.nowPlaying = false
		a.searchBox.Focus(c)
	})
	if a.nowPlaying {
		c.OnShortcut(0, ui.KeyEscape, func() { a.nowPlaying = false })
		// ⌘ and a number choose Now Playing's tab of that place.
		if a.pl.Current() != nil {
			for i, t := range a.npTabs() {
				c.OnShortcut(ui.Cmd, ui.Key1+ui.Key(i), func() { a.choosePane(t.pane) })
			}
		}
	}
}

// open opens a page, remembering the one before for the back button, and
// closes Now Playing over it, though it is the page open. A page of another
// source makes that source the active one.
func (a *app) open(r route) {
	a.nowPlaying = false
	if r.src == "" {
		r.src = a.route.src
	}
	if r == a.route {
		return
	}
	a.history, a.forward = append(a.history, a.route), nil
	a.show(r)
}

// show shows the page of r.
func (a *app) show(r route) {
	if r.src == "" {
		r.src = a.route.src
	}
	lib := a.libOf(r.src)
	if lib == nil {
		// Its source went away since: the active one's Home instead.
		r = route{page: pageHome, src: a.library.id}
		lib = a.library
	}
	a.activate(lib)
	a.route = r
	if r.page == pageSearch {
		a.query, a.searched = r.query, r
	}
	a.load(r)
}

// load loads what the page of r shows, unless it has.
func (a *app) load(r route) {
	lib := a.libOf(r.src)
	if lib == nil {
		return
	}
	switch r.page {
	case pageHome:
		a.freshenHome()
	case pagePlaylist:
		a.loadPlaylist(lib, r.id, false)
	case pageSearch:
		a.loadSearch(r, false)
	case pageArtist:
		a.loadArtist(lib, r.id)
	case pageAlbum:
		a.loadAlbum(lib, r.id)
	case pageShow:
		a.loadShow(lib, r.id)
	case pageShows:
		a.loadSubscribed(lib)
	case pageStations:
		a.loadStations(lib)
	case pageLiked:
		if !lib.likedPaged.loaded {
			a.loadLikedPage(lib)
		}
	case pageSavedAlbums:
		if !lib.savedAlbumsPaged.loaded {
			a.loadSavedAlbums(lib)
		}
	case pageBrowse:
		a.loadBrowse(lib, r.id, false)
	case pageSection:
		if r.id != "" {
			a.loadBrowse(lib, r.id, false)
		}
	}
}

// back goes to the page before, as the back button of a mouse does, or
// closes Now Playing, which covers the pages.
func (a *app) back() {
	if a.nowPlaying {
		a.nowPlaying = false
		return
	}
	if len(a.history) == 0 {
		return
	}
	a.forward = append(a.forward, a.route)
	r := a.history[len(a.history)-1]
	a.history = a.history[:len(a.history)-1]
	a.show(r)
}

// goForward goes to the page back left, as the forward button of a mouse
// does.
func (a *app) goForward() {
	if len(a.forward) == 0 {
		return
	}
	a.history = append(a.history, a.route)
	r := a.forward[len(a.forward)-1]
	a.forward = a.forward[:len(a.forward)-1]
	a.nowPlaying = false
	a.show(r)
}

func (a *app) sidebar(c *ui.Context) {
	// Under the window buttons where they sit over the sidebar, as the
	// traffic lights do; elsewhere, as close to the top as to the sides.
	top := float32(14)
	if c.TitleBar().Left > 0 {
		top = topBar(c) + 4
	}
	// Where nothing in it takes the press, the sidebar moves the window, as
	// the title bar it reaches up into would.
	ui.Column(c).Fill().Padding(top, 14, 14, 14).Gap(4).DragWindow().Children(func() {
		a.sourceSwitcher(c)
		if a.can("search") {
			a.searchField(c)
		}

		ui.Scroll(c.Key("nav")).TrackScroll(&a.navScroll).Grow(1).Margin(0, -6).Padding(0, 6).Gap(4).Children(func() {
			a.navItem(c, iconHome, T("Home"), route{page: pageHome})
			// What the source adds itself comes first: for the local files,
			// their songs, albums and artists.
			if a.can("browse") {
				for _, n := range a.src.Info().Navigation {
					a.navItem(c, navIcon(n.Icon), n.Title, route{page: pageBrowse, id: n.Page})
				}
			}
			if a.can("liked") {
				a.navItem(c, iconHeart, T("Liked Songs"), route{page: pageLiked})
			}
			if a.can("savedAlbums") {
				a.navItem(c, iconAlbum, T("Saved Albums"), route{page: pageSavedAlbums})
			}
			if a.can("stations") {
				a.navItem(c, iconRadio, T("Radio"), route{page: pageStations})
			}
			if a.can("shows") {
				a.navItem(c, iconPodcast, T("Podcasts"), route{page: pageShows})
			}
			if a.searched.query != "" && a.searched.src == a.id {
				a.navItem(c, iconSearch, T("Search Results"), a.searched)
			}
			if a.can("playlists") {
				a.myPlaylists(c, T("My Playlists"), a.mine, true, &a.mineCollapsed)
				a.myPlaylists(c, T("Saved Playlists"), a.saved, false, &a.savedCollapsed)
			}
		})

		// What plays, as a card at the foot of the sidebar, over who listens.
		if tr := a.pl.Current(); tr != nil {
			a.sidebarNowPlaying(c, tr)
		}
		a.accountRow(c)
	})
}

func (a *app) navItem(c *ui.Context, icon *ui.SVG, label string, r route) {
	p := a.pal
	selected := a.route.page == r.page && !a.nowPlaying && (r.page != pageBrowse || a.route.id == r.id)
	b := ui.ButtonBase(c.Key(label)).Height(34).PaddingX(10).Gap(10).Radius(8).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false)
	bg := ui.Transparent
	switch {
	case selected:
		bg = p.press
	case b.Hovered():
		bg = p.hover
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	color := p.text
	if selected {
		color = p.accent
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(17, 17).TextColor(color)
		ui.Text(c, label).FontSize(13.5).FontWeight(500).TextColor(p.text)
	})
	if b.Clicked() {
		a.open(r)
	}
}

func (a *app) sidebarNowPlaying(c *ui.Context, tr *music.Track) {
	p := a.pal
	card := ui.ButtonBase(c.Key("np-card")).Gap(10).Padding(8).Radius(12).AlignItems(ui.Center).FocusRing(false).Label(T("Now Playing"))
	bg := p.hover
	if card.Hovered() {
		bg = p.press
	}
	card.Background(card.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	card.Children(func() {
		a.cover(c, tr.Album.ImageURL, 96, 40, 6).Shadow(0, 2, 6, 0, p.cardShade)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, tr.Name).FontSize(12.5).FontWeight(600).SingleLine()
			a.artistLinks(c, tr, p.muted, p.text).FontSize(11.5).SingleLine()
		})
	})
	if card.Clicked() {
		a.nowPlaying = true
	}
}

// playerBarHeight is the height of the player along the foot of the pages.
const playerBarHeight = 84

// pageGap is the room between the bar over a page and its content.
const pageGap = 12

// insets returns the room the bars over a page take at its top and its
// bottom: pages pad their content by it, and scroll under the bars.
func (a *app) insets(c *ui.Context) (top, bottom float32) {
	top = topBar(c)
	if runtime.GOOS == "darwin" {
		// The edge reaches a little below the traffic lights' strip.
		top += topEdgePad
	}
	return top, playerBarHeight
}

// topEdgePad is the room the edge over a page leaves below the strip, on
// macOS.
const topEdgePad = 6

// buttonsHeight is the height of the window buttons off macOS, which fill
// the edge over a page.
const buttonsHeight = topBarHeight + topEdgePad

// topBar returns the height of the strip along the top of the window, as
// tall as the window buttons and the room above and below them, which
// what sits in it centers on.
func topBar(c *ui.Context) float32 {
	if runtime.GOOS != "darwin" {
		// The whole edge, which the buttons fill: what sits in the strip
		// centers on their glyphs.
		return buttonsHeight
	}
	return max(c.TitleBar().Height, topBarHeight)
}

// topBarHeight is the least height of the strip along the top.
const topBarHeight = 40

// startMark marks where a page's content starts, first in what scrolls:
// once it has moved up, content is under the bar at the top.
func (a *app) startMark(c *ui.Context) {
	b := ui.Box(c.Key("start-mark")).Height(0).Bounds()
	if b.W > 0 {
		top, _ := a.insets(c)
		a.underTop = b.Y < top+pageGap-0.5
	}
}

// endMark marks where a page's content ends, last in what scrolls: while it
// is below the top of the player, content is under the player.
func (a *app) endMark(c *ui.Context) {
	b := ui.Box(c.Key("end-mark")).Height(0).Bounds()
	if b.W > 0 {
		_, h := c.Size()
		_, bottom := a.insets(c)
		a.underBottom = b.Y > h-bottom+0.5
	}
}

// edgeShown returns how much of a scroll edge shows over el: fully while
// content is under its bar, and none at rest, as AppKit shows its edges.
func edgeShown(el ui.Element, under bool) float32 {
	target := float32(0)
	if under {
		target = 1
	}
	return el.AnimateTo("shown", target, ui.Animation{Duration: 180 * time.Millisecond})
}

// pageTop is the strip over the top of a page, frosting what scrolls under
// it as macOS 26's hard scroll edge does: it drags the window, and holds
// the back button once there is a page to go back to.
func (a *app) pageTop(c *ui.Context) {
	top, _ := a.insets(c)
	edge := ui.Box(c.Key("top-edge")).Absolute().Top(0).Left(0).Right(0).Height(top).DragWindow().
		Material(glass.ScrollEdge{Hard: true, Background: a.pal.bg})
	edge.Opacity(edgeShown(edge, a.underTop))
	if len(a.history) > 0 || len(a.forward) > 0 {
		ui.Row(c).Absolute().Top(0).Left(20).Height(topBar(c)).AlignItems(ui.Center).Children(func() {
			back := iconButton(c.Key("back"), iconBack, 18, a.pal.muted, T("Back")).Disabled(len(a.history) == 0)
			if len(a.history) == 0 {
				back.Opacity(0.35)
			}
			if back.Clicked() {
				a.back()
			}
			if len(a.forward) > 0 && iconButton(c.Key("forward"), iconForward, 18, a.pal.muted, T("Forward")).Clicked() {
				a.goForward()
			}
		})
	}
}

func (a *app) content(c *ui.Context) {
	// A new page starts at its top. Not as it is opened: a control in a
	// list, as a page's tabs in its header, opens it as the list builds.
	if a.scrolled != a.route {
		a.trackList, a.pageScroll, a.scrolled = ui.ListState{}, ui.ScrollState{}, a.route
	}
	// Pages mark where their content is, as they scroll.
	a.underTop, a.underBottom = false, false
	switch a.route.page {
	case pageHome:
		a.homePage(c)
	case pagePlaylist:
		a.playlistPage(c)
	case pageSearch:
		a.searchPage(c)
	case pageStations:
		a.stationsPage(c)
	case pageArtist:
		a.artistPage(c)
	case pageAlbum:
		a.albumPage(c)
	case pageShow:
		a.showPage(c)
	case pageShows:
		a.showsPage(c)
	case pageLiked:
		a.likedPage(c)
	case pageSavedAlbums:
		a.savedAlbumsPage(c)
	case pageBrowse:
		a.browsePage(c)
	case pageSection:
		a.sectionPage(c)
	}
}

// centered shows a message, with a retry button if retry is set: what a
// page shows in place of what failed to load. Where the source wants the
// listener to sign in or set it up, it offers that.
func (a *app) centered(c *ui.Context, err string, retry func()) {
	top, bottom := a.insets(c)
	ui.Column(c).Grow(1).Padding(top, 0, bottom, 0).Center().Gap(12).Children(func() {
		ui.Text(c, err).TextColor(a.pal.muted).FontSize(13).TextAlign(ui.Center).MaxWidth(520)
		ui.Row(c).Gap(10).Children(func() {
			a.remedy(c)
			if retry != nil && ui.Button(c.Key("retry"), T("Retry")).Clicked() {
				retry()
			}
		})
	})
}

// remedy is a button that takes the listener where the source's error can be
// fixed, if it said one.
func (a *app) remedy(c *ui.Context) {
	lib := a.library
	switch {
	case lib.needsSignIn && lib.signedOut():
		if ui.PrimaryButton(c.Key("sign-in"), T("Sign In")).Clicked() {
			a.promptSignIn(lib)
		}
	case lib.needsSetup:
		if ui.PrimaryButton(c.Key("setup"), T("Open Settings")).Clicked() {
			a.openSettings()
		}
	}
}

// Loading what the pages show.

// homeFresh is how long what Home shows stays good. A plugin's picks may change
// by the hour; the app may stay open overnight.
const homeFresh = time.Hour

// loadHome loads what the Home of a source shows: its own page if it makes one
// up, else what it played lately, the playlists being the sidebar's.
func (a *app) loadHome(lib *library) {
	lib.homeLoading, lib.homeErr = true, ""
	at := time.Now()
	go func() {
		var home *music.Browse
		var recent []music.Track
		var homeErr, recentErr error
		if lib.can("home") {
			home, homeErr = lib.src.Home()
		}
		if home == nil || len(home.Sections) == 0 {
			home = nil
			if lib.can("recent") {
				recent, recentErr = lib.src.Recent()
			}
		}
		a.post(func() {
			lib.homeLoading = false
			lib.home, lib.recent = home, recent
			lib.homeAt = at
			switch {
			case homeErr != nil:
				lib.homeErr = a.problem(lib, homeErr, T("Couldn't load this page"))
			case recentErr != nil:
				lib.homeErr = a.problem(lib, recentErr, T("Couldn't load this page"))
			}
		})
	}()
}

// freshenHome loads the home page again a while after it last did.
func (a *app) freshenHome() {
	lib := a.library
	if !lib.homeLoading && !lib.homeAt.IsZero() && time.Since(lib.homeAt) > homeFresh {
		a.loadHome(lib)
	}
}

// loadPlaylist loads a playlist's page, and plays it once it has, if thenPlay.
func (a *app) loadPlaylist(lib *library, id string, thenPlay bool) {
	if pl := lib.playlists[id]; pl != nil {
		if thenPlay {
			a.pl.PlayQueue(pl.Tracks, firstPlayable(pl.Tracks))
		}
		return
	}
	if lib.loadingPl[id] {
		return
	}
	lib.loadingPl[id] = true
	delete(lib.plErr, id)
	go func() {
		pl, err := answered(lib.src.Playlist(id))
		a.post(func() {
			delete(lib.loadingPl, id)
			if err != nil {
				lib.plErr[id] = a.problem(lib, err, T("Couldn't load this playlist"))
				return
			}
			lib.playlists[id] = pl
			if thenPlay {
				a.pl.PlayQueue(pl.Tracks, firstPlayable(pl.Tracks))
			}
		})
	}()
}

func firstPlayable(tracks []music.Track) int {
	for i := range tracks {
		if !tracks[i].Unavailable {
			return i
		}
	}
	return 0
}

// sourceSwitcher chooses the active source, atop the sidebar, once there is
// more than one.
func (a *app) sourceSwitcher(c *ui.Context) {
	if len(a.libs) < 2 {
		return
	}
	p := a.pal
	b := ui.ButtonBase(c.Key("source")).Height(32).PaddingX(10).Gap(8).Radius(9).AlignItems(ui.Center).Justify(ui.Start).
		FocusRing(false).Label(T("Source")).Margin(0, 0, 8, 0).Border(1, p.hairline)
	if b.Hovered() {
		b.Background(p.hover)
	}
	b.Children(func() {
		ui.Text(c, a.name()).FontSize(13).FontWeight(600).SingleLine().Grow(1).Shrink(1).MinWidth(0)
		ui.Icon(c, iconUpDown).Size(13, 13).TextColor(p.faint)
	})
	b.Menu(func(m *ui.Menu) {
		for _, lib := range a.libs {
			if m.Item(lib.name()).Checked(lib == a.library).Chosen() {
				a.switchSource(lib.id)
			}
		}
	})
}
