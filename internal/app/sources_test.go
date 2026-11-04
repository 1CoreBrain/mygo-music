package app

import (
	"errors"
	"mygo-music/internal/plugin"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo/ui"

	"mygo-music/internal/i18n"
	"mygo-music/internal/music"
)

// addOther adds a second source, called Other, whose pages and sidebar a test
// tells from those of fake.
func addOther(a *app) (*fakeSource, *library) {
	other := newFakeSource("other")
	other.info.Name = "Other"
	return other, addFake(a, other)
}

// chooseSource chooses a source with the sidebar's switcher.
func chooseSource(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	if err := tt.Click("Source"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(name); err != nil {
		t.Fatalf("%v; the menu shows %q", err, tt.Menu())
	}
}

// The sidebar's switcher only shows once there's another source to choose.
func TestSwitcherNeedsAnotherSource(t *testing.T) {
	a := newTestApp()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if _, ok := tt.Find("Source"); ok {
		t.Error("a switcher with nothing to switch to")
	}
	addOther(a)
	tt.Frame()
	if _, ok := tt.Find("Source"); !ok {
		t.Error("no switcher with two sources")
	}
}

// Choosing another source makes it active and shows its Home and its playlists,
// which it's asked for the first time then, and remembers it; back returns to
// the page of the source before, which is active again.
func TestSwitchSource(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	without(a, "isLiked")
	other, lib := addOther(a)
	lib.started = false
	other.playlists = func() ([]music.Playlist, error) {
		return []music.Playlist{{ID: "1", Name: "Other's list", Mine: true}}, nil
	}
	other.home = func() (*music.Browse, error) {
		return &music.Browse{Sections: []music.Section{{Title: "Other's shelf", Albums: []music.Album{{ID: "8", Name: "Other's album"}}}}}, nil
	}
	a.mine = []music.Playlist{{ID: "10", Name: "My list", Mine: true}}
	a.playlists["10"] = &music.Playlist{ID: "10", Name: "My list", Mine: true}
	list := route{page: pagePlaylist, src: "fake", id: "10"}
	a.open(list)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()

	chooseSource(t, tt, "Other")
	if a.library != lib || a.route != (route{page: pageHome, src: "other"}) || a.settings.Source != "other" {
		t.Fatalf("active %q on %+v, remembered %q", a.library.id, a.route, a.settings.Source)
	}
	until(t, ch, func() bool { return len(lib.mine) > 0 && lib.home != nil })
	tt.Frame()
	if _, ok := tt.Find("Other's list"); !ok {
		t.Errorf("no playlist of its in the sidebar: %q", tt.Texts())
	}
	if _, ok := tt.Find("My list"); ok {
		t.Error("the other source's playlist stayed in the sidebar")
	}
	if _, ok := tt.Find("Other's album"); !ok {
		t.Errorf("no Home of its: %q", tt.Texts())
	}

	tt.Key(0, ui.KeyBack)
	tt.Frame()
	if a.library != a.libs[0] || a.route != list || a.settings.Source != "fake" {
		t.Fatalf("back: active %q on %+v, remembered %q", a.library.id, a.route, a.settings.Source)
	}
	if _, ok := tt.Find("My list"); !ok {
		t.Error("its playlist didn't come back to the sidebar")
	}
}

// Going to the source that's active goes nowhere, and a page of a source gone
// since shows Home of the one that's active.
func TestSwitchToNoOtherSource(t *testing.T) {
	a := newTestApp()
	a.switchSource("fake")
	a.switchSource("gone")
	if len(a.history) != 0 || a.route.page != pageHome {
		t.Fatalf("on %+v, history %v", a.route, a.history)
	}
	a.show(route{page: pageArtist, src: "gone", id: "1"})
	if a.route != (route{page: pageHome, src: "fake"}) || a.library != a.libs[0] {
		t.Errorf("on %+v, active %q", a.route, a.library.id)
	}
}

// What a source answers after the listener switched to another lands in its own
// library, and shows when they come back, without asking again.
func TestAnswerLandsInItsOwnLibrary(t *testing.T) {
	asked := make(chan string, 4)
	a := newTestApp()
	ch := posts(a)
	without(a, "isLiked")
	first := a.library
	fake(a).playlist = func(id string) (*music.Playlist, error) {
		asked <- id
		return &music.Playlist{ID: id, Name: "Late list", Tracks: []music.Track{song("1", "Late song")}}, nil
	}
	_, lib := addOther(a)
	a.open(route{page: pagePlaylist, id: "7"})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	wait(t, asked, "the playlist")

	chooseSource(t, tt, "Other")
	runPost(t, ch)
	tt.Frame()
	if first.playlists["7"] == nil || lib.playlists["7"] != nil {
		t.Fatalf("landed in the first %v, in the other %v", first.playlists["7"] != nil, lib.playlists["7"] != nil)
	}
	if a.library != lib {
		t.Errorf("the answer switched the source to %q", a.library.id)
	}
	if _, ok := tt.Find("Late song"); ok {
		t.Error("the answer showed on the other source's page")
	}

	tt.Key(0, ui.KeyBack)
	tt.Frame()
	if _, ok := tt.Find("Late song"); !ok || len(asked) > 0 {
		t.Errorf("shows %q, asked %d more", tt.Texts(), len(asked))
	}
}

// A song playing from a source that isn't active likes with its own source, and
// its heart answers from its own library.
func TestHeartOfASongOfAnotherSource(t *testing.T) {
	liked := make(chan string, 1)
	a := newTestApp()
	other, lib := addOther(a)
	lib.account = &music.Account{ID: "1"}
	other.like = func(id string, on bool) error { liked <- id; return nil }
	fake(a).like = func(string, bool) error { t.Error("liked with the active source"); return nil }
	tr := song("7", "宝船")
	tr.Source = "other"
	a.pl.Cue([]music.Track{tr})
	a.pl.SetLyrics(nil)
	tt := ui.NewTester(a.view, 1200, 800)
	settled(tt)
	if err := tt.Click("Like"); err != nil {
		t.Fatal(err)
	}
	if id := wait(t, liked, "the like"); id != "7" || !lib.liked["7"] || a.libs[0].liked["7"] {
		t.Errorf("liked %s; in its library %v, in the active one %v", id, lib.liked["7"], a.libs[0].liked["7"])
	}
}

// Each thing a source can't do is gone with its capability, where it shows
// while it can.
func TestHiddenWithoutCapability(t *testing.T) {
	// The song's own heart, for a page that has no other.
	playing := func(a *app) {
		a.pl.Cue([]music.Track{song("7", "宝船")})
		a.pl.SetLyrics(nil)
	}
	page := func(r route) func(a *app) { return func(a *app) { a.route = r } }
	loaded := func(a *app) {
		a.account = &music.Account{ID: "1", Name: "小明"}
		a.artists["12"] = &artistData{page: &music.ArtistPage{Artist: music.Artist{ID: "12", Name: "乙"}, Tracks: []music.Track{song("1", "一")}}}
		a.albums["8"] = &albumData{album: &music.AlbumPage{Album: music.Album{ID: "8", Name: "迷跡波"}}}
		a.playlists["30"] = &music.Playlist{ID: "30", Name: "华语经典"}
		a.shows["5"] = &showData{show: &music.Show{ID: "5", Name: "夜航"}, page: paged{loaded: true}}
	}
	for _, c := range []struct {
		name, capability, label string
		set                     func(a *app)
	}{
		{"liked songs", "liked", "Liked Songs", nil},
		{"radio", "stations", "Radio", nil},
		{"podcasts", "shows", "Podcasts", nil},
		{"saved albums", "savedAlbums", "Saved Albums", nil},
		{"search", "search", "Search", nil},
		{"new playlist", "createPlaylist", "New Playlist", nil},
		{"heart", "like", "Like", playing},
		{"follow", "followArtist", "Follow", page(route{page: pageArtist, src: "fake", id: "12"})},
		{"save playlist", "savePlaylist", "Save", page(route{page: pagePlaylist, src: "fake", id: "30"})},
		{"save album", "saveAlbum", "Save", page(route{page: pageAlbum, src: "fake", id: "8"})},
		{"subscribe", "subscribeShow", "Subscribe", page(route{page: pageShow, src: "fake", id: "5"})},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := newTestApp()
			loaded(a)
			if c.set != nil {
				c.set(a)
			}
			tt := ui.NewTester(a.view, 1200, 800)
			settled(tt)
			if _, ok := tt.Find(c.label); !ok {
				t.Fatalf("no %q while it can: %q", c.label, tt.Texts())
			}
			without(a, c.capability)
			settled(tt)
			if _, ok := tt.Find(c.label); ok {
				t.Errorf("%q shows without %s", c.label, c.capability)
			}
		})
	}
}

// Now Playing has no Comments tab for a source without comments, and leaves the
// comments for the queue if they were showing.
func TestCommentsGoWithoutCapability(t *testing.T) {
	asked := make(chan string, 1)
	a := newTestApp()
	fake(a).comments = func(track string, _ int) (*music.CommentPage, error) {
		asked <- track
		return &music.CommentPage{}, nil
	}
	without(a, "comments")
	a.pl.Cue([]music.Track{song("7", "宝船")})
	a.pl.SetLyrics(nil)
	a.nowPlaying, a.npPane = true, paneComments
	tt := ui.NewTester(a.view, 1200, 800)
	settled(tt)
	if _, ok := tt.Find("Comments"); ok {
		t.Error("a Comments tab without comments")
	}
	if a.npPane != paneQueue {
		t.Errorf("on pane %d", a.npPane)
	}
	if len(asked) > 0 {
		t.Error("asked a source without comments for them")
	}
}

// The menus of a playlist offer what the source can do of renaming and deleting,
// and no menu at all without either.
func TestPlaylistMenuGoesWithoutCapability(t *testing.T) {
	menu := func(t *testing.T, a *app, tt *ui.Tester) []string {
		t.Helper()
		tt.Frame()
		if err := tt.RightClick("睡前"); err != nil {
			t.Fatal(err)
		}
		items := tt.Menu()
		tt.CloseMenu()
		return items
	}
	a := editTestApp()
	a.route, a.homeLoading = route{page: pageHome, src: "fake"}, true
	tt := ui.NewTester(a.view, 1200, 800)
	if items := menu(t, a, tt); len(items) < 2 {
		t.Fatalf("the menu is %q", items)
	}
	without(a, "renamePlaylist")
	if err := tt.RightClick("睡前"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Rename…"); err == nil {
		t.Error("offered to rename without being able to")
	}
	if err := tt.ChooseMenuItem("Delete Playlist…"); err != nil {
		t.Errorf("no deleting without renaming: %v", err)
	}
	a.deleting.open = false
	without(a, "deletePlaylist")
	if items := menu(t, a, tt); items != nil {
		t.Errorf("a menu of %q with nothing to do", items)
	}

	a.route = route{page: pagePlaylist, src: "fake", id: "10"}
	tt.Frame()
	if _, ok := tt.Find("More"); ok {
		t.Error("the playlist's page has a menu with nothing in it")
	}
}

// A song's menu doesn't offer what its source can't do with playlists.
func TestSongMenuGoesWithoutCapability(t *testing.T) {
	a := editTestApp()
	a.playlists["10"].Tracks = append(a.playlists["10"].Tracks, song("3", "三"))
	without(a, "addToPlaylist", "removeFromPlaylist", "movePlaylistTrack")
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.RightClick("宝船"); err != nil {
		t.Fatal(err)
	}
	for _, item := range []string{"Add to Playlist", "Remove from Playlist", "Move Up", "Move Down"} {
		for _, label := range tt.Menu() {
			if label == item {
				t.Errorf("offered %q without being able to", item)
			}
		}
	}
	tt.CloseMenu()
}

// A source answering an error as a page loads shows the page's error, with Retry,
// which loads it again, and the page once it answers.
func TestPageErrorRetries(t *testing.T) {
	offline := errors.New("offline")
	for _, c := range []struct {
		name string
		r    route
		set  func(f *fakeSource, fails func() error)
	}{
		{"playlist", route{page: pagePlaylist, id: "7"}, func(f *fakeSource, fails func() error) {
			f.playlist = func(id string) (*music.Playlist, error) {
				if err := fails(); err != nil {
					return nil, err
				}
				return &music.Playlist{ID: id, Name: "Answered"}, nil
			}
		}},
		{"artist", route{page: pageArtist, id: "7"}, func(f *fakeSource, fails func() error) {
			f.artist = func(id string) (*music.ArtistPage, error) {
				if err := fails(); err != nil {
					return nil, err
				}
				return &music.ArtistPage{Artist: music.Artist{ID: id, Name: "Answered"}}, nil
			}
		}},
		{"album", route{page: pageAlbum, id: "7"}, func(f *fakeSource, fails func() error) {
			f.album = func(id string) (*music.AlbumPage, error) {
				if err := fails(); err != nil {
					return nil, err
				}
				return &music.AlbumPage{Album: music.Album{ID: id, Name: "Answered"}}, nil
			}
		}},
		{"show", route{page: pageShow, id: "7"}, func(f *fakeSource, fails func() error) {
			f.show = func(id string, _ int) (*music.ShowPage, error) {
				if err := fails(); err != nil {
					return nil, err
				}
				return &music.ShowPage{Show: music.Show{ID: id, Name: "Answered"}}, nil
			}
		}},
		{"browse", route{page: pageBrowse, id: "7"}, func(f *fakeSource, fails func() error) {
			f.browse = func(string) (*music.Browse, error) {
				if err := fails(); err != nil {
					return nil, err
				}
				return &music.Browse{Title: "Answered"}, nil
			}
		}},
		{"liked", route{page: pageLiked}, func(f *fakeSource, fails func() error) {
			f.liked = func(int) (*music.Page, error) {
				if err := fails(); err != nil {
					return nil, err
				}
				return &music.Page{Tracks: []music.Track{song("1", "Answered")}}, nil
			}
		}},
		{"stations", route{page: pageStations}, func(f *fakeSource, fails func() error) {
			f.stations = func() ([]music.Station, error) {
				if err := fails(); err != nil {
					return nil, err
				}
				return []music.Station{{ID: "1", Name: "Answered"}}, nil
			}
		}},
		{"podcasts", route{page: pageShows}, func(f *fakeSource, fails func() error) {
			f.shows = func() ([]music.Show, error) {
				if err := fails(); err != nil {
					return nil, err
				}
				return []music.Show{{ID: "1", Name: "Answered"}}, nil
			}
		}},
		{"saved albums", route{page: pageSavedAlbums}, func(f *fakeSource, fails func() error) {
			f.savedAlbums = func(int) (*music.AlbumList, error) {
				if err := fails(); err != nil {
					return nil, err
				}
				return &music.AlbumList{Albums: []music.Album{{ID: "1", Name: "Answered"}}}, nil
			}
		}},
		{"search", route{page: pageSearch, query: "q", kind: music.SearchTracks}, func(f *fakeSource, fails func() error) {
			f.search = func(string, music.SearchKind, int) (*music.SearchResult, error) {
				if err := fails(); err != nil {
					return nil, err
				}
				return &music.SearchResult{Tracks: []music.Track{song("1", "Answered")}}, nil
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := newTestApp()
			ch := posts(a)
			without(a, "isLiked")
			var tries atomic.Int32
			c.set(fake(a), func() error {
				if tries.Add(1) == 1 {
					return offline
				}
				return nil
			})
			a.open(c.r)
			tt := ui.NewTester(a.view, 1200, 800)
			tt.Frame()
			runPost(t, ch)
			tt.Frame()
			tt.Frame()
			if !tt.HasText("offline") {
				t.Errorf("doesn't say what's wrong: %q", tt.Texts())
			}
			if err := tt.Click("Retry"); err != nil {
				t.Fatalf("%v: %q", err, tt.Texts())
			}
			runPost(t, ch)
			tt.Frame()
			tt.Frame()
			if tries.Load() != 2 {
				t.Errorf("asked %d times", tries.Load())
			}
			if _, ok := tt.Find("Retry"); ok || !tt.HasText("Answered") {
				t.Errorf("once it answered, shows %q", tt.Texts())
			}
		})
	}
}

// Home that a source fails to answer shows the error, with Retry.
func TestHomeErrorRetries(t *testing.T) {
	var tries atomic.Int32
	a := newTestApp()
	ch := posts(a)
	without(a, "recent")
	fake(a).home = func() (*music.Browse, error) {
		if tries.Add(1) == 1 {
			return nil, errors.New("offline")
		}
		return &music.Browse{Sections: []music.Section{{Title: "Shelf", Albums: []music.Album{{ID: "1", Name: "Answered"}}}}}, nil
	}
	a.loadHome(a.library)
	runPost(t, ch)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if !tt.HasText("offline") {
		t.Errorf("doesn't say what's wrong: %q", tt.Texts())
	}
	if err := tt.Click("Retry"); err != nil {
		t.Fatal(err)
	}
	runPost(t, ch)
	tt.Frame()
	if _, ok := tt.Find("Answered"); !ok {
		t.Errorf("once it answered, shows %q", tt.Texts())
	}
}

// A source that answers ErrSignIn opens the dialog once, however often it says
// so after, and the page offers to sign in for when it's put away.
func TestErrSignInAsksOnce(t *testing.T) {
	var tries atomic.Int32
	a := newTestApp()
	ch := posts(a)
	fake(a).playlist = func(string) (*music.Playlist, error) { tries.Add(1); return nil, music.ErrSignIn }
	a.open(route{page: pagePlaylist, id: "7"})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	runPost(t, ch)
	if !a.signIn.open || a.signIn.lib != a.library || !a.needsSignIn {
		t.Fatalf("dialog %+v, needs signing in %v", a.signIn, a.needsSignIn)
	}
	settled(tt)
	if err := tt.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	settled(tt)
	if a.signIn.open {
		t.Fatal("Cancel left the dialog open")
	}

	if err := tt.Click("Retry"); err != nil {
		t.Fatal(err)
	}
	runPost(t, ch)
	settled(tt)
	if a.signIn.open || tries.Load() != 2 {
		t.Fatalf("asked again: dialog open %v, tried %d times", a.signIn.open, tries.Load())
	}

	if err := tt.Click("Sign In"); err != nil {
		t.Fatalf("%v: %q", err, tt.Texts())
	}
	if !a.signIn.open {
		t.Error("the page's Sign In opened no dialog")
	}
}

// A source that answers ErrSetup needs setting up, which shows on the page and
// its settings, and goes once a setting is kept.
func TestErrSetupOffersSettings(t *testing.T) {
	kept := make(chan string, 1)
	a := newTestApp()
	ch := posts(a)
	fake(a).info.Settings = []music.Setting{{Key: "key", Label: "API Key"}}
	fake(a).playlist = func(string) (*music.Playlist, error) { return nil, music.ErrSetup }
	fake(a).setSetting = func(key, value string) error { kept <- key + "=" + value; return nil }
	a.library.askedSetup = true // opening the window takes a real app
	a.open(route{page: pagePlaylist, id: "7"})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	runPost(t, ch)
	tt.Frame()
	if !a.needsSetup {
		t.Fatal("doesn't need setting up")
	}
	if _, ok := tt.Find("Open Settings"); !ok {
		t.Errorf("the page offers no settings: %q", tt.Texts())
	}

	settings := ui.NewTester(a.settingsView, 600, 1400)
	settings.Frame()
	// The field, under the title labelled as it is.
	r, ok := settings.Find("API Key")
	if !ok {
		t.Fatal("no field")
	}
	settings.ClickAt(r.X+20, r.Y+r.H+18)
	settings.Type("secret")
	settings.Key(0, ui.KeyEnter)
	if got := wait(t, kept, "the setting"); got != "key=secret" {
		t.Fatalf("kept %s", got)
	}
	until(t, ch, func() bool { return !a.needsSetup })
	if a.askedSetup {
		t.Error("it wouldn't ask next time it needs it")
	}
}

// Sign in to X in the account row opens the dialog, whose Sign In calls the source,
// and the account shows; Sign Out goes back.
func TestSignInAndOut(t *testing.T) {
	asked := make(chan struct{}, 1)
	signedOut := make(chan struct{}, 1)
	a := newTestApp()
	ch := posts(a)
	f := fake(a)
	f.signIn = func() (*music.Account, error) {
		asked <- struct{}{}
		return &music.Account{ID: "1", Name: "小明"}, nil
	}
	f.signOut = func() error { signedOut <- struct{}{}; return nil }
	a.homeLoading = true // for the dialog's Sign In to be the one that shows
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("Sign in to fake"); err != nil {
		t.Fatal(err)
	}
	settled(tt)
	if !a.signIn.open {
		t.Fatal("no dialog")
	}
	if err := tt.Click("Sign In"); err != nil {
		t.Fatal(err)
	}
	wait(t, asked, "signing in")
	until(t, ch, func() bool { return a.account != nil })
	if a.signIn.open || a.signIn.waiting {
		t.Errorf("the dialog stayed: %+v", a.signIn)
	}
	settled(tt)
	if _, ok := tt.Find("小明"); !ok {
		t.Fatalf("no account in %q", tt.Texts())
	}
	if _, ok := tt.Find("Sign in to fake"); ok {
		t.Error("still offers to sign in")
	}

	if err := tt.Click("小明"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Sign Out"); err != nil {
		t.Fatal(err)
	}
	wait(t, signedOut, "signing out")
	settled(tt)
	if a.account != nil {
		t.Error("still signed in")
	}
	if _, ok := tt.Find("Sign in to fake"); !ok {
		t.Errorf("doesn't offer to sign in again: %q", tt.Texts())
	}
}

// A sign-in that fails says why in its dialog, ready to try again.
func TestSignInFails(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	fake(a).signIn = func() (*music.Account, error) { return nil, errors.New("the code expired") }
	a.promptSignIn(a.library)
	a.startSignIn()
	if !a.signIn.waiting {
		t.Fatal("not waiting")
	}
	runPost(t, ch)
	if !a.signIn.open || a.signIn.waiting || a.signIn.err != "the code expired" || a.account != nil {
		t.Errorf("the dialog is %+v, account %+v", a.signIn, a.account)
	}
}

// A sign-in that ends after the dialog was put away still counts, with what's the
// listener's loaded anew.
func TestSignInEndsAfterDialog(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	f := fake(a)
	f.signIn = func() (*music.Account, error) { return &music.Account{ID: "1", Name: "小明"}, nil }
	f.playlists = func() ([]music.Playlist, error) { return []music.Playlist{{ID: "10", Name: "通勤", Mine: true}}, nil }
	a.promptSignIn(a.library)
	a.startSignIn()
	a.signIn.open = false
	until(t, ch, func() bool { return a.account != nil && len(a.mine) > 0 })
	if a.signIn.open {
		t.Error("the dialog came back")
	}
}

// Who's signed in is asked of the source once it starts: signed in already, it's
// known without the dialog, and one who isn't asks nothing of the listener.
func TestAccountAskedOnStart(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	fake(a).account = func() (*music.Account, error) { return &music.Account{ID: "1", Name: "小明"}, nil }
	a.library.started = false
	a.startLibrary(a.library)
	until(t, ch, func() bool { return a.account != nil })
	if a.signIn.open {
		t.Error("asked the listener to sign in")
	}
}

// The active source's pages load once what it can do is known.
func TestLearnStartsTheActiveSource(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	home := make(chan struct{}, 1)
	fake(a).home = func() (*music.Browse, error) { home <- struct{}{}; return nil, nil }
	a.library.caps, a.library.started = map[string]bool{}, false
	a.learn(a.library)
	until(t, ch, func() bool { return len(a.library.caps) > 0 })
	wait(t, home, "Home")
}

// A source that won't start can do nothing, and says why, for the settings.
func TestLearnSaysWhyASourceWontStart(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	fake(a).caps = map[string]bool{}
	fake(a).account = func() (*music.Account, error) { return nil, errors.New("no such binary") }
	a.library.caps, a.library.started = map[string]bool{}, false
	a.learn(a.library)
	until(t, ch, func() bool { return a.library.startErr != "" })
	if a.library.startErr != "no such binary" || len(a.library.caps) != 0 {
		t.Errorf("it says %q, can do %v", a.library.startErr, a.library.caps)
	}
}

// Changing the language says the app's words in it from the next frame on, and
// English again once chosen.
func TestSetLanguage(t *testing.T) {
	defer i18n.SetLanguage(i18n.English)
	a := newTestApp()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()

	onMainThread(func() { a.setLanguage(i18n.Chinese) })
	if i18n.Language() != i18n.Chinese || a.settings.Language != i18n.Chinese {
		t.Fatalf("language %q, kept %q", i18n.Language(), a.settings.Language)
	}
	tt.Frame()
	if _, ok := tt.Find(T("Liked Songs")); !ok {
		t.Errorf("no %q in %q", T("Liked Songs"), tt.Texts())
	}

	onMainThread(func() { a.setLanguage(i18n.English) })
	tt.Frame()
	if _, ok := tt.Find("Liked Songs"); !ok {
		t.Errorf("English didn't come back: %q", tt.Texts())
	}
}

// A plugin asking to start programs may not until the listener allows it in the
// settings, and the same button takes that back.
func TestSettingsAllowPrograms(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	src := &guardedSource{fakeSource: newFakeSource("guarded")}
	src.perms.Process = true
	src.perms.Network = []string{"api.example.com"}
	addFake(a, src)

	settings := ui.NewTester(a.settingsView, 600, 1400)
	settings.Frame()
	if err := settings.Click("Allow Programs"); err != nil {
		t.Fatal(err)
	}
	runPost(t, ch)
	if !src.Granted() {
		t.Fatal("not allowed after allowing")
	}
	settings.Frame()
	if err := settings.Click("Stop Allowing"); err != nil {
		t.Fatal(err)
	}
	runPost(t, ch)
	if src.Granted() {
		t.Fatal("still allowed after stopping")
	}
}

// guardedSource is a fake source with permissions, as a plugin has.
type guardedSource struct {
	*fakeSource
	perms   plugin.Permissions
	mu      sync.Mutex
	granted bool
}

func (s *guardedSource) Permissions() plugin.Permissions { return s.perms }

func (s *guardedSource) Granted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.granted
}

func (s *guardedSource) Grant(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.granted = on
	return nil
}
