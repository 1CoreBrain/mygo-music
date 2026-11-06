package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mygo-music/internal/music"
)

func demo(t *testing.T) *Plugin {
	t.Helper()
	p, err := Load(filepath.Join("..", "..", "examples", "demo-plugin"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

// The example the README points at does all that a plugin can, through the real
// host, so copying it starts from something that works.
func TestDemoPluginDoesEverything(t *testing.T) {
	p := demo(t)
	must := func(what string, err error) {
		t.Helper()
		if err != nil {
			t.Errorf("%s: %v", what, err)
		}
	}
	for _, name := range []string{
		"signIn", "signOut", "account", "setSetting", "home", "browse", "search", "suggest", "recent", "played",
		"liked", "isLiked", "like", "likeEpisode", "album", "artist", "saveAlbum", "savedAlbums", "followArtist", "playlists",
		"playlist", "createPlaylist", "renamePlaylist", "deletePlaylist", "addToPlaylist", "removeFromPlaylist",
		"movePlaylistTrack", "savePlaylist", "shows", "show", "subscribeShow", "stations", "stationNext", "comments",
		"likeComment", "postComment", "lyrics", "stream", "remotePlay", "remotePause", "remoteResume", "remoteSeek",
		"remoteVolume", "remoteState", "remoteDevices", "remoteUse",
	} {
		if !p.Can(name) {
			t.Errorf("doesn't export %s", name)
		}
	}

	if a, err := p.SignIn(); err != nil || a.Name == "" {
		t.Errorf("signIn: %+v, %v", a, err)
	}
	if a, err := p.Account(); err != nil || a == nil {
		t.Errorf("account: %+v, %v", a, err)
	}
	must("signOut", p.SignOut())
	if a, err := p.Account(); err != nil || a != nil {
		t.Errorf("account after signing out: %+v, %v", a, err)
	}

	home, err := p.Home()
	must("home", err)
	if home == nil || len(home.Sections) != 4 || home.Sections[0].Tracks[0].Source != "demo" {
		t.Errorf("home: %+v", home)
	}
	if b, err := p.Browse("discover"); err != nil || len(b.Sections) == 0 {
		t.Errorf("browse: %+v, %v", b, err)
	}

	if r, err := p.Search("song", music.SearchTracks, 0); err != nil || len(r.Tracks) != 3 {
		t.Errorf("search: %+v, %v", r, err)
	}
	if r, err := p.Search("show", music.SearchShows, 0); err != nil || len(r.Shows) != 1 {
		t.Errorf("search shows: %+v, %v", r, err)
	}
	if s, err := p.Suggest("so"); err != nil || len(s) != 3 {
		t.Errorf("suggest: %v, %v", s, err)
	}

	must("like", p.Like("t2", true))
	if liked, err := p.IsLiked([]string{"t1", "t2"}); err != nil || liked["t1"] || !liked["t2"] {
		t.Errorf("isLiked: %v, %v", liked, err)
	}
	if page, err := p.Liked(0); err != nil || len(page.Tracks) != 1 || page.Tracks[0].Source != "demo" {
		t.Errorf("liked: %+v, %v", page, err)
	}
	if r, err := p.Recent(); err != nil || len(r) != 2 {
		t.Errorf("recent: %v, %v", r, err)
	}
	p.Played(music.Track{ID: "t1"})

	must("saveAlbum", p.SaveAlbum("al1", true))
	if al, err := p.Album("al1"); err != nil || !al.Album.Saved || len(al.Tracks) != 3 {
		t.Errorf("album: %+v, %v", al, err)
	}
	if l, err := p.SavedAlbums(0); err != nil || len(l.Albums) != 1 {
		t.Errorf("savedAlbums: %+v, %v", l, err)
	}
	must("followArtist", p.FollowArtist("a1", true))
	if ar, err := p.Artist("a1"); err != nil || !ar.Artist.Following || len(ar.Albums) != 1 {
		t.Errorf("artist: %+v, %v", ar, err)
	}

	made, err := p.CreatePlaylist("New")
	must("createPlaylist", err)
	if made == nil {
		t.Fatal("no playlist made")
	}
	must("renamePlaylist", p.RenamePlaylist(made.ID, "Renamed"))
	must("addToPlaylist", p.AddToPlaylist(made.ID, []string{"t1", "t2", "t3"}))
	must("movePlaylistTrack", p.MovePlaylistTrack(made.ID, 0, 2))
	must("removeFromPlaylist", p.RemoveFromPlaylist(made.ID, []string{"t3"}))
	got, err := p.Playlist(made.ID)
	must("playlist", err)
	if got == nil || got.Name != "Renamed" || len(got.Tracks) != 2 || got.Tracks[0].ID != "t2" || got.Tracks[1].ID != "t1" {
		t.Errorf("playlist after changes: %+v", got)
	}
	must("savePlaylist", p.SavePlaylist("p1", true))
	must("deletePlaylist", p.DeletePlaylist(made.ID))
	if ps, err := p.Playlists(); err != nil || len(ps) != 1 {
		t.Errorf("playlists: %+v, %v", ps, err)
	}

	must("subscribeShow", p.SubscribeShow("s1", true))
	if s, err := p.Shows(); err != nil || len(s) != 1 || !s[0].Subscribed {
		t.Errorf("shows: %+v, %v", s, err)
	}
	if s, err := p.Show("s1", 0); err != nil || !s.Show.Subscribed || len(s.Episodes) != 2 ||
		s.Episodes[0].ShowID != "s1" || s.Episodes[0].Source != "demo" {
		t.Errorf("show: %+v, %v", s, err)
	}

	if s, err := p.Stations(); err != nil || len(s) != 1 {
		t.Errorf("stations: %+v, %v", s, err)
	}
	if n, err := p.StationNext("st1"); err != nil || len(n) == 0 || n[0].Source != "demo" {
		t.Errorf("stationNext: %+v, %v", n, err)
	}

	if c, err := p.Comments("t1", 0); err != nil || len(c.Comments) != 1 {
		t.Errorf("comments: %+v, %v", c, err)
	}
	must("likeComment", p.LikeComment("t1", "c1", true))
	if c, err := p.PostComment("t1", "hi"); err != nil || c.Text != "hi" {
		t.Errorf("postComment: %+v, %v", c, err)
	}

	if l, err := p.Lyrics("t1"); err != nil || l.LRC == "" {
		t.Errorf("lrc: %+v, %v", l, err)
	}
	if l, err := p.Lyrics("t2"); err != nil || len(l.Lines) != 1 || len(l.Lines[0].Words) != 3 {
		t.Errorf("timed lyrics: %+v, %v", l, err)
	}

	// The stream is a file that's there, for the app to play.
	s, err := p.Stream("t1", "high")
	must("stream", err)
	if s == nil || s.Type != "mp3" || !strings.HasPrefix(s.URL, "file://") {
		t.Fatalf("stream: %+v", s)
	}
	if _, err := os.Stat(strings.TrimPrefix(s.URL, "file://")); err != nil {
		t.Errorf("stream's file: %v", err)
	}

	must("remotePlay", p.RemotePlay([]string{"t1", "t2"}, 1, 500))
	must("remotePause", p.RemotePause())
	must("remoteSeek", p.RemoteSeek(900))
	must("remoteVolume", p.RemoteVolume(40))
	st, err := p.RemoteState()
	must("remoteState", err)
	if st == nil || st.TrackID != "t2" || st.Playing || st.Position != 900 || st.Volume != 40 {
		t.Errorf("remoteState: %+v", st)
	}
	must("remoteResume", p.RemoteResume())
	if d, err := p.RemoteDevices(); err != nil || len(d) != 1 {
		t.Errorf("remoteDevices: %+v, %v", d, err)
	}
	must("remoteUse", p.RemoteUse("demo"))
}

// The errors the example shows are the ones the app acts on.
func TestDemoPluginErrors(t *testing.T) {
	p := demo(t)
	if _, err := p.Stream("nope", ""); err == nil {
		t.Fatal("streamed a track that's not there")
	} else if !strings.Contains(err.Error(), music.ErrUnavailable.Error()) {
		t.Fatalf("got %v", err)
	}
}
