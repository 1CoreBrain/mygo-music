package app

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"mygo-music/internal/local"
	"mygo-music/internal/lyrics"
	"mygo-music/internal/music"
)

// framesDir is where MM_FRAMES says to save a look at the views, which a test
// skips without.
func framesDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("MM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	return dir
}

// With MM_FRAMES set, saves Home as it greets a listener with no music yet.
func TestHomeEmptyFrame(t *testing.T) {
	dir := framesDir(t)
	a := localApp(t)
	tt := ui.NewTester(a.view, 1180, 780)
	settled(tt)
	saveFrame(t, dir, "home-empty.png", tt.Image())
}

// framesTracks are songs with covers, by an invented band.
func framesTracks(a *app) []music.Track {
	covers := []struct{ from, to ui.Color }{
		{ui.RGB(40, 90, 170), ui.RGB(220, 120, 160)},
		{ui.RGB(230, 150, 70), ui.RGB(120, 40, 90)},
		{ui.RGB(30, 130, 110), ui.RGB(210, 220, 140)},
	}
	band := music.Artist{ID: "1", Name: "The Polar Lights"}
	var tracks []music.Track
	for i, name := range []string{"Northern Lights", "Slow Tide", "A Long Way North", "Paper Boats", "The Harbor at Night", "Quiet Hours", "Last Ferry Home", "Snowfield"} {
		url := fmt.Sprintf("cover:%d", i%len(covers))
		giveCover(a, url, coverImage(covers[i%len(covers)].from, covers[i%len(covers)].to))
		tracks = append(tracks, music.Track{
			ID: fmt.Sprint(i + 1), Source: "fake", Name: name, Artists: []music.Artist{band},
			Album:    music.Album{ID: fmt.Sprint(10 + i%len(covers)), Name: "Polar Night", ImageURL: url},
			Duration: int64(180000 + 21000*i),
		})
	}
	return tracks
}

// With MM_FRAMES set, saves a playlist's page.
func TestPlaylistFrame(t *testing.T) {
	dir := framesDir(t)
	a := newTestApp()
	a.account = &music.Account{ID: "1", Name: "Alex"}
	tracks := framesTracks(a)
	a.mine = []music.Playlist{
		{ID: "10", Name: "Evening Walks", ImageURL: "cover:0", Mine: true},
		{ID: "11", Name: "Focus", ImageURL: "cover:1", Mine: true},
	}
	a.saved = []music.Playlist{{ID: "20", Name: "Staff Picks", ImageURL: "cover:2", Saved: true}}
	a.playlists["10"] = &music.Playlist{ID: "10", Name: "Evening Walks", Owner: "Alex", Description: "For the long way home.", ImageURL: "cover:0", Mine: true, Tracks: tracks}
	a.route = route{page: pagePlaylist, src: "fake", id: "10"}
	tt := ui.NewTester(a.view, 1180, 780)
	settled(tt)
	saveFrame(t, dir, "playlist.png", tt.Image())
}

// With MM_FRAMES set, saves Now Playing with lyrics following a song.
func TestNowPlayingFrame(t *testing.T) {
	dir := framesDir(t)
	a := newTestApp()
	a.account = &music.Account{ID: "1", Name: "Alex"}
	tracks := framesTracks(a)
	a.pl.Cue(tracks)
	a.pl.SetLyrics(lyrics.Build(
		"[00:04.00]The sky goes green at the edges\n[00:09.00]And everyone stops to look\n[00:14.00]Nobody says a word\n[00:19.00]Nobody needs to\n[00:24.00]We stand out on the ice\n[00:29.00]Until the light goes home",
		"",
	))
	a.pl.Hold(10 * time.Second)
	a.nowPlaying, a.npPane = true, paneLyrics
	tt := ui.NewTester(a.view, 1180, 780)
	tt.SetDark(true)
	tt.Advance(2 * time.Second)
	tt.Frame()
	saveFrame(t, dir, "now-playing.png", tt.Image())
}

// With MM_FRAMES set, saves the settings as they list the sources: the local
// files and a plugin that asks for a couple of things.
func TestSettingsFrame(t *testing.T) {
	dir := framesDir(t)
	a := newTestApp()
	a.libs = nil
	a.local = local.New(t.TempDir())
	addFake(a, a.local)
	plugin := newFakeSource("radio-garden")
	plugin.info.Name = "Example Source"
	plugin.info.Version = "1.2.0"
	plugin.info.Settings = []music.Setting{
		{Key: "apiKey", Label: "API Key", Help: "Keys are free for personal use.", Link: "https://example.com/keys"},
		{Key: "region", Label: "Region"},
	}
	plugin.vals["region"] = "Nordics"
	a.library = addFake(a, plugin)
	a.account = &music.Account{ID: "1", Name: "Alex"}
	a.settingsPage = "plugins"
	tt := ui.NewTester(a.settingsView, 780, 720)
	settled(tt)
	saveFrame(t, dir, "settings.png", tt.Image())
}
