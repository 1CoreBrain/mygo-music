package local

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"mygo-music/internal/i18n"
	"mygo-music/internal/music"
)

// entry is what's known of a file, as index.json keeps it.
type entry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Mod  int64  `json:"mod"` // in nanoseconds since 1970

	Title       string `json:"title"`
	Artist      string `json:"artist,omitempty"` // as the tag has it
	Album       string `json:"album,omitempty"`
	AlbumArtist string `json:"albumArtist,omitempty"`
	Year        int    `json:"year,omitempty"`
	Track       int    `json:"track,omitempty"`
	Disc        int    `json:"disc,omitempty"`
	Duration    int64  `json:"duration,omitempty"` // milliseconds

	Cover   string `json:"cover,omitempty"`   // the picture inside, once kept
	Sidecar string `json:"sidecar,omitempty"` // the picture beside it

	Gain float64 `json:"gain,omitempty"`
	Peak float64 `json:"peak,omitempty"`
}

func (e *entry) image() string { return cmp.Or(e.Cover, e.Sidecar) }

// catalog is everything listed, sorted and ready to answer from. Once built
// it's never changed, for any goroutine to read without a lock.
type catalog struct {
	items      []*item // by name
	byID       map[string]*item
	byPath     map[string]*item
	albums     []*album // by name
	albumByID  map[string]*album
	artists    []*artist // by name
	artistByID map[string]*artist
}

type item struct {
	entry
	id       string
	artists  []music.Artist
	album    *album // nil without an album tag
	imageURL string
	sortName string
	search   string // what a search looks through, in lower case
}

type album struct {
	music.Album
	// various says the tracks are by different artists and none is credited.
	// It isn't in Album's Artists, since its name goes by the language at the
	// time of asking.
	various  bool
	items    []*item // by disc, track and name
	sortName string
}

type artist struct {
	music.Artist
	items    []*item  // by album, disc, track and name
	albums   []*album // by name
	sortName string
}

// makeID is an ID that stays the same across scans, for likes and playlists to
// keep meaning the same track.
func makeID(kind, key string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + key))
	return hex.EncodeToString(sum[:8])
}

// albumKey groups tracks: by the album artist if there is one, who can spread
// an album over folders; or else by folder, or every "Greatest Hits" would be
// one album.
func albumKey(e *entry) string {
	name := strings.ToLower(e.Album)
	if e.AlbumArtist != "" {
		return name + "\x00" + strings.ToLower(e.AlbumArtist)
	}
	return name + "\x00" + filepath.Dir(e.Path)
}

// artistSplit is what's between artists in a tag. Not commas: "Earth, Wind &
// Fire" is one.
var artistSplit = regexp.MustCompile(`(?i) / |;|、| feat\. | ft\. `)

// splitArtists returns the names in an artist tag, each once.
func splitArtists(tag string) []string {
	var names []string
	seen := map[string]bool{}
	for _, name := range artistSplit.Split(tag, -1) {
		name = strings.TrimSpace(name)
		if key := strings.ToLower(name); name != "" && !seen[key] {
			seen[key] = true
			names = append(names, name)
		}
	}
	return names
}

func buildCatalog(entries []entry) *catalog {
	c := &catalog{
		byID:       make(map[string]*item, len(entries)),
		byPath:     make(map[string]*item, len(entries)),
		albumByID:  map[string]*album{},
		artistByID: map[string]*artist{},
	}
	// By path, for whose spelling of a name comes first not to change between
	// builds.
	sorted := slices.Clone(entries)
	slices.SortFunc(sorted, func(a, b entry) int { return cmp.Compare(a.Path, b.Path) })

	for i := range sorted {
		it := &item{entry: sorted[i], id: makeID("track", sorted[i].Path)}
		if img := it.image(); img != "" {
			it.imageURL = music.FileURL(img)
		}
		it.sortName = strings.ToLower(it.Title)
		names := make([]string, 0, 1)
		for _, name := range splitArtists(it.Artist) {
			a := c.artistNamed(name)
			a.items = append(a.items, it)
			it.artists = append(it.artists, a.Artist)
			names = append(names, a.Name)
		}
		it.search = strings.ToLower(it.Title + "\n" + strings.Join(names, "\n") + "\n" + it.Album)
		if it.Album != "" {
			id := makeID("album", albumKey(&it.entry))
			al := c.albumByID[id]
			if al == nil {
				al = &album{Album: music.Album{ID: id, Name: it.Album}, sortName: strings.ToLower(it.Album)}
				c.albumByID[id] = al
				c.albums = append(c.albums, al)
			}
			al.items = append(al.items, it)
			it.album = al
		}
		c.items = append(c.items, it)
		c.byID[it.id] = it
		c.byPath[it.Path] = it
	}

	for _, al := range c.albums {
		c.finishAlbum(al)
	}
	slices.SortFunc(c.items, func(a, b *item) int {
		return cmp.Or(cmp.Compare(a.sortName, b.sortName), cmp.Compare(a.Path, b.Path))
	})
	byName := func(a, b *album) int {
		return cmp.Or(cmp.Compare(a.sortName, b.sortName), cmp.Compare(a.ID, b.ID))
	}
	slices.SortFunc(c.albums, byName)
	for _, a := range c.artistByID {
		c.artists = append(c.artists, a)
		slices.SortFunc(a.items, byAlbum)
		slices.SortFunc(a.albums, byName)
		for _, al := range a.albums {
			if al.ImageURL != "" {
				a.ImageURL = al.ImageURL
				break
			}
		}
	}
	slices.SortFunc(c.artists, func(a, b *artist) int {
		return cmp.Or(cmp.Compare(a.sortName, b.sortName), cmp.Compare(a.ID, b.ID))
	})
	return c
}

func (c *catalog) artistNamed(name string) *artist {
	id := makeID("artist", strings.ToLower(name))
	a := c.artistByID[id]
	if a == nil {
		a = &artist{Artist: music.Artist{ID: id, Name: name}, sortName: strings.ToLower(name)}
		c.artistByID[id] = a
	}
	return a
}

// byTrack is an album's order.
func byTrack(a, b *item) int {
	return cmp.Or(
		cmp.Compare(a.Disc, b.Disc),
		cmp.Compare(a.Track, b.Track),
		cmp.Compare(a.sortName, b.sortName),
		cmp.Compare(a.Path, b.Path),
	)
}

// byAlbum is an artist's order, with what's on no album after the rest.
func byAlbum(a, b *item) int {
	if (a.album == nil) != (b.album == nil) {
		if a.album == nil {
			return 1
		}
		return -1
	}
	if a.album != nil && a.album != b.album {
		return cmp.Or(cmp.Compare(a.album.sortName, b.album.sortName), cmp.Compare(a.album.ID, b.album.ID))
	}
	return byTrack(a, b)
}

// finishAlbum works out what an album has from its tracks, now it has them
// all, and adds it to its artists.
func (c *catalog) finishAlbum(al *album) {
	slices.SortFunc(al.items, byTrack)
	first := al.items[0]
	// Albums are finished one at a time: this one is last if it's there at all.
	credit := func(a *artist) {
		if n := len(a.albums); n == 0 || a.albums[n-1] != al {
			a.albums = append(a.albums, al)
		}
	}
	disagree := func(it *item) bool {
		return !slices.EqualFunc(it.artists, first.artists, func(a, b music.Artist) bool { return a.ID == b.ID })
	}
	switch {
	case first.AlbumArtist != "":
		for _, name := range splitArtists(first.AlbumArtist) {
			a := c.artistNamed(name)
			al.Artists = append(al.Artists, a.Artist)
			credit(a)
		}
	case slices.ContainsFunc(al.items, disagree):
		al.various = true
	default:
		al.Artists = first.artists
	}
	for _, it := range al.items {
		if al.ImageURL == "" {
			al.ImageURL = it.imageURL
		}
		if al.Released == "" && it.Year > 0 {
			al.Released = strconv.Itoa(it.Year)
		}
		for _, a := range it.artists {
			credit(c.artistByID[a.ID])
		}
	}
}

// public is the album as it's answered, in the language of the moment.
func (al *album) public() music.Album {
	out := al.Album
	if al.various {
		out.Artists = []music.Artist{{Name: i18n.T("Various Artists")}}
	}
	return out
}

// public is the track as it's answered. Its picture is the album's if it has
// none of its own, and it has one without an album too.
func (it *item) public() music.Track {
	t := music.Track{ID: it.id, Source: ID, Name: it.Title, Artists: it.artists, Duration: it.Duration}
	if it.album != nil {
		t.Album = it.album.public()
	}
	if it.imageURL != "" {
		t.Album.ImageURL = it.imageURL
	}
	return t
}

func publicTracks(items []*item) []music.Track {
	tracks := make([]music.Track, len(items))
	for i, it := range items {
		tracks[i] = it.public()
	}
	return tracks
}

func publicAlbums(albums []*album) []music.Album {
	out := make([]music.Album, len(albums))
	for i, al := range albums {
		out[i] = al.public()
	}
	return out
}

// resolve answers the tracks of ids still on disk, in order.
func (c *catalog) resolve(ids []string) []music.Track {
	tracks := make([]music.Track, 0, len(ids))
	for _, id := range ids {
		if it, ok := c.byID[id]; ok {
			tracks = append(tracks, it.public())
		}
	}
	return tracks
}
