package plugin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type zipped struct {
	name string
	body string
	mode os.FileMode
}

func makeZip(t *testing.T, files ...zipped) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, f := range files {
		h := &zip.FileHeader{Name: f.name, Method: zip.Deflate}
		mode := f.mode
		if mode == 0 {
			mode = 0o644
		}
		h.SetMode(mode)
		out, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		out.Write([]byte(f.body))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// github is a GitHub with one repository: with a release of tag, or with commits
// alone if tag is empty, serving archive.
type github struct {
	tag, sha string
	assets   []string
	archive  []byte
	notFound bool
}

func (g *github) start(t *testing.T) {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case g.notFound:
			http.NotFound(w, r)
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			if g.tag == "" {
				http.NotFound(w, r)
				return
			}
			type asset struct {
				Name string `json:"name"`
				URL  string `json:"browser_download_url"`
			}
			assets := []asset{}
			for _, a := range g.assets {
				assets = append(assets, asset{a, srv.URL + "/asset/" + a})
			}
			json.NewEncoder(w).Encode(map[string]any{"tag_name": g.tag, "zipball_url": srv.URL + "/zip", "assets": assets})
		case strings.HasSuffix(r.URL.Path, "/commits/HEAD"):
			json.NewEncoder(w).Encode(map[string]string{"sha": g.sha})
		default:
			w.Write(g.archive)
		}
	}))
	t.Cleanup(srv.Close)
	was := githubAPI
	githubAPI = srv.URL
	t.Cleanup(func() { githubAPI = was })
}

const goodManifest = `{"id":"good","name":"Good","version":"1.0.0","permissions":{"network":["api.example.com"]}}`

func goodArchive(t *testing.T, top string) []byte {
	return makeZip(t,
		zipped{name: top + "plugin.json", body: goodManifest},
		zipped{name: top + "index.js", body: `export function suggest() { return ["hi"]; }`},
		zipped{name: top + "bin/tool", body: "#!/bin/sh\n", mode: 0o755},
	)
}

// Installing from a release, as GitHub's archive of it has it in a folder: nothing
// shows until it's installed, then it loads, says where it's from, and keeps a
// program executable.
func TestInstallFromRelease(t *testing.T) {
	g := &github{tag: "v1.2.0", archive: goodArchive(t, "owner-repo-abc/")}
	g.start(t)
	dir := t.TempDir()
	p, err := Fetch("owner/repo", dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != "v1.2.0" || p.Manifest.ID != "good" || len(p.Manifest.Permissions.Network) != 1 {
		t.Fatalf("pending: %+v", p)
	}
	if got := Discover(dir, t.TempDir()); len(got) != 0 {
		t.Fatalf("%d plugins before installing", len(got))
	}
	if err := p.Install(dir); err != nil {
		t.Fatal(err)
	}
	got := Discover(dir, t.TempDir())
	if len(got) != 1 {
		t.Fatalf("%d plugins after installing", len(got))
	}
	defer got[0].Close()
	if o := got[0].Origin(); o == nil || o.Repo != "owner/repo" || o.Version != "v1.2.0" {
		t.Fatalf("origin: %+v", o)
	}
	if s, err := got[0].Suggest(""); err != nil || s[0] != "hi" {
		t.Fatalf("running it: %v, %v", s, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, "good", "bin", "tool"))
		if err != nil || info.Mode()&0o100 == 0 {
			t.Errorf("the program: %v, %v", info, err)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("left behind: %v", entries)
	}
}

// Without a release, it's the latest commit, by which it updates too.
func TestInstallFromCommit(t *testing.T) {
	g := &github{sha: "0123456789abcdef0123", archive: goodArchive(t, "owner-repo-0123/")}
	g.start(t)
	p, err := Fetch("owner/repo", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Discard()
	if p.Version != "0123456789ab" {
		t.Fatalf("version %q", p.Version)
	}
}

// Updating replaces the plugin's files, those gone included, and leaves its data;
// what it replaces runs no more, and a newer version is what says there's one.
func TestUpdateKeepsData(t *testing.T) {
	g := &github{tag: "v1", archive: goodArchive(t, "")}
	g.start(t)
	dir, data := t.TempDir(), t.TempDir()
	p, err := Fetch("owner/repo", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Install(dir); err != nil {
		t.Fatal(err)
	}
	old, err := Load(filepath.Join(dir, "good"), data)
	if err != nil {
		t.Fatal(err)
	}
	if err := old.SetSetting("kept", "yes"); err != nil {
		t.Fatal(err)
	}

	g.tag = "v2"
	g.archive = makeZip(t,
		zipped{name: "plugin.json", body: goodManifest},
		zipped{name: "index.js", body: `export function suggest() { return ["new"]; }`},
	)
	rel, err := Latest("owner/repo")
	if err != nil || rel.Version == old.Origin().Version {
		t.Fatalf("no update seen: %+v, %v", rel, err)
	}
	p, err = Fetch("owner/repo", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Install(dir); err != nil {
		t.Fatal(err)
	}
	now, err := old.Reloaded(data)
	if err != nil {
		t.Fatal(err)
	}
	defer now.Close()
	if s, err := now.Suggest(""); err != nil || s[0] != "new" {
		t.Fatalf("still the old code: %v, %v", s, err)
	}
	if now.Setting("kept") != "yes" {
		t.Error("lost its settings")
	}
	if now.Origin().Version != "v2" {
		t.Errorf("version %q", now.Origin().Version)
	}
	if _, err := os.Stat(filepath.Join(dir, "good", "bin", "tool")); !os.IsNotExist(err) {
		t.Errorf("a file the new version doesn't have stayed: %v", err)
	}
}

// A repository can't take the name of a plugin from elsewhere, and with it what
// that plugin signed in to; the one there stays.
func TestInstallWontTakeAnothersName(t *testing.T) {
	g := &github{tag: "v1", archive: goodArchive(t, "")}
	g.start(t)
	dir := t.TempDir()
	p, err := Fetch("owner/repo", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Install(dir); err != nil {
		t.Fatal(err)
	}
	p, err = Fetch("someone/else", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Install(dir); err == nil {
		t.Fatal("took its name")
	}
	var o Origin
	readJSON(filepath.Join(dir, "good", originFile), &o)
	if o.Repo != "owner/repo" {
		t.Errorf("replaced it with %q", o.Repo)
	}
}

// An archive that isn't what it should be installs nothing: one writing outside
// the folder, with a link, without a manifest, that won't compile, or named as
// the built-in plugin or a path.
func TestInstallRefusesBadArchives(t *testing.T) {
	escape := filepath.Join(t.TempDir(), "escaped")
	for name, archive := range map[string][]byte{
		"outside": makeZip(t, zipped{name: "../../" + filepath.Base(escape), body: "x"}, zipped{name: "plugin.json", body: goodManifest}),
		"link":    makeZip(t, zipped{name: "plugin.json", body: goodManifest}, zipped{name: "l", body: "/etc/passwd", mode: os.ModeSymlink | 0o777}),
		"nothing": makeZip(t, zipped{name: "README.md", body: "hello"}),
		"broken":  makeZip(t, zipped{name: "plugin.json", body: goodManifest}, zipped{name: "index.js", body: "export function ("}),
		"builtin": makeZip(t, zipped{name: "plugin.json", body: `{"id":"local"}`}, zipped{name: "index.js", body: ""}),
		"path":    makeZip(t, zipped{name: "plugin.json", body: `{"id":"../up"}`}, zipped{name: "index.js", body: ""}),
	} {
		g := &github{tag: "v1", archive: archive}
		g.start(t)
		dir := t.TempDir()
		if p, err := Fetch("owner/repo", dir); err == nil {
			p.Discard()
			t.Errorf("%s: fetched it", name)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("%s: left behind %v", name, entries)
		}
	}
	if _, err := os.Stat(escape); !os.IsNotExist(err) {
		t.Errorf("wrote outside: %v", err)
	}
}

// No repository, or a private one, says so.
func TestNoSuchRepo(t *testing.T) {
	g := &github{notFound: true}
	g.start(t)
	if _, err := Fetch("owner/repo", t.TempDir()); err != ErrNoRepo {
		t.Fatalf("got %v", err)
	}
}

// A repository can be written as its address or its name.
func TestParseRepo(t *testing.T) {
	for in, want := range map[string]string{
		"owner/repo":                        "owner/repo",
		" owner/repo ":                      "owner/repo",
		"https://github.com/owner/repo":     "owner/repo",
		"https://github.com/owner/repo.git": "owner/repo",
		"github.com/owner/repo/":            "owner/repo",
		"https://www.github.com/o-1/r_2.x":  "o-1/r_2.x",
	} {
		if got, err := ParseRepo(in); err != nil || got != want {
			t.Errorf("%q: %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "owner", "a/b/c", "https://example.com/a/b", "../x", "a/.."} {
		if got, err := ParseRepo(in); err == nil {
			t.Errorf("%q: read as %q", in, got)
		}
	}
}

// A release's zip for this platform is the one to take, or one for all; one for
// other platforms alone leaves its source.
func TestPickAsset(t *testing.T) {
	for _, c := range []struct {
		names []string
		want  int
	}{
		{[]string{"x-linux-amd64.zip", "x-darwin-arm64.zip", "x.zip"}, 1},
		{[]string{"plugin.zip", "notes.txt"}, 0},
		{[]string{"x-linux-amd64.zip"}, -1},
		{[]string{"x-darwin-arm64.tar.gz"}, -1},
		{nil, -1},
	} {
		if got := pickAsset(c.names, "darwin", "arm64"); got != c.want {
			t.Error(fmt.Sprintf("%v: %d, want %d", c.names, got, c.want))
		}
	}
}

// A release taken back isn't an update, numbers compare as numbers, and a commit,
// having no order, is one if it's different.
func TestIsNewer(t *testing.T) {
	for _, c := range []struct {
		latest, installed string
		want              bool
	}{
		{"v1.2.0", "v1.1.9", true},
		{"v1.10.0", "v1.9.0", true},
		{"v1.1.0", "v1.2.0", false},
		{"v1.2", "1.2.0", false},
		{"1.2.1", "v1.2", true},
		{"v1.2.0", "v1.2.0", false},
		{"abc123def456", "0123456789ab", true},
		{"v1.0.0", "0123456789ab", true},
		{"v2.0.0-beta", "v1.0.0", true},
	} {
		if got := IsNewer(c.latest, c.installed); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v", c.latest, c.installed, got)
		}
	}
}
