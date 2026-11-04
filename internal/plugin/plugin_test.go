package plugin

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"mygo-music/internal/music"
)

// The test binary is the program plugins start: with this set, it echoes lines.
const echoEnv = "MYGO_MUSIC_TEST_ECHO"

func TestMain(m *testing.M) {
	if os.Getenv(echoEnv) != "" {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			fmt.Println("echo:" + sc.Text())
		}
		return
	}
	os.Exit(m.Run())
}

func write(t *testing.T, manifest, source string) (dir, data string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "p")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o600)
	name := "index.js"
	if strings.Contains(manifest, `index.ts`) {
		name = "index.ts"
	}
	os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600)
	return dir, t.TempDir()
}

func load(t *testing.T, manifest, source string) *Plugin {
	t.Helper()
	dir, data := write(t, manifest, source)
	p, err := Load(dir, data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

const plain = `{"id":"x"}`

// An export answers plainly or through a promise, and the tracks it answers
// are of the plugin, whatever they say, as that's what they'll play through.
func TestAnswersStampedWithThePlugin(t *testing.T) {
	p := load(t, plain, `
export function search({ query }) {
  return { tracks: [{ id: "1", name: query, source: "lying" }] };
}
export async function recent() {
  await new Promise((r) => setTimeout(r, 1));
  return [{ id: "2", name: "later" }];
}`)
	r, err := p.Search("hello", music.SearchTracks, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Tracks) != 1 || r.Tracks[0].Name != "hello" || r.Tracks[0].Source != "x" {
		t.Fatalf("search: %+v", r.Tracks)
	}
	recent, err := p.Recent()
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 1 || recent[0].Name != "later" || recent[0].Source != "x" {
		t.Fatalf("recent: %+v", recent)
	}
}

// TypeScript runs as it is, so plugins needn't be built.
func TestTypeScript(t *testing.T) {
	p := load(t, `{"id":"x","main":"index.ts"}`, `
export function suggest(p: { query: string }): string[] { return [p.query + "!"]; }`)
	got, err := p.Suggest("a")
	if err != nil || len(got) != 1 || got[0] != "a!" {
		t.Fatalf("got %v, %v", got, err)
	}
}

// The errors the app acts on come through by their code, sync or not, keeping
// what the plugin said, and anything else by its message.
func TestErrors(t *testing.T) {
	p := load(t, plain, `
function fail(code, message) { const e = new Error(message); e.code = code; return e; }
export function stream() { throw fail("unavailable", "not in your country"); }
export async function account() { throw fail("signIn"); }
export function album() { throw new Error("boom"); }
export function artist() { throw "just text"; }`)
	if _, err := p.Stream("1", ""); !errors.Is(err, music.ErrUnavailable) || !strings.Contains(err.Error(), "not in your country") {
		t.Errorf("stream: %v", err)
	}
	if _, err := p.Account(); !errors.Is(err, music.ErrSignIn) {
		t.Errorf("account: %v", err)
	}
	if _, err := p.Album("1"); err == nil || err.Error() != "boom" {
		t.Errorf("album: %v", err)
	}
	if _, err := p.Artist("1"); err == nil || err.Error() != "just text" {
		t.Errorf("artist: %v", err)
	}
}

// What a plugin doesn't export is known without running it, and isn't asked.
func TestWhatItDoesIsItsExports(t *testing.T) {
	p := load(t, plain, `globalThis.ran = 1; export function search() { return {}; }`)
	if !p.Can("search") || p.Can("album") {
		t.Fatal("Can doesn't follow the exports")
	}
	if p.vm != nil {
		t.Fatal("Can started the plugin")
	}
	if _, err := p.Album("1"); !errors.Is(err, music.ErrUnsupported) {
		t.Fatalf("album: %v", err)
	}
}

// A plugin that never returns can't hang the app: the call gives up, and the
// plugin runs again for the next one.
func TestStuckPluginRestarts(t *testing.T) {
	p := load(t, plain, `
export function album() { for (;;) {} }
export function suggest() { return ["fine"]; }`)
	err := p.callWithin(50*time.Millisecond, "album", map[string]string{"id": "1"}, nil)
	if err == nil || !strings.Contains(err.Error(), "in time") {
		t.Fatalf("album: %v", err)
	}
	got, err := p.Suggest("a")
	if err != nil || len(got) != 1 {
		t.Fatalf("after restarting: %v, %v", got, err)
	}
}

// One only waiting on something isn't stuck: it keeps what it has in memory.
func TestWaitingPluginIsLeftRunning(t *testing.T) {
	p := load(t, plain, `
let calls = 0;
export function album() { return new Promise(() => {}); }
export function suggest() { calls++; return [String(calls)]; }`)
	p.Suggest("a")
	if err := p.callWithin(50*time.Millisecond, "album", map[string]string{"id": "1"}, nil); err == nil {
		t.Fatal("a call that never settles answered")
	}
	got, err := p.Suggest("a")
	if err != nil || got[0] != "2" {
		t.Fatalf("it restarted: %v, %v", got, err)
	}
}

// Without the permission fetch reaches nothing; with it, only what it names.
func TestNetworkPermission(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `["from the server"]`)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	source := fmt.Sprintf(`export async function suggest() { return await (await fetch(%q)).json(); }`, srv.URL)

	none := load(t, plain, source)
	if _, err := none.Suggest("a"); err == nil {
		t.Error("fetch got through with no permission")
	}
	other := load(t, `{"id":"x","permissions":{"network":["example.com"]}}`, source)
	if _, err := other.Suggest("a"); err == nil {
		t.Error("fetch reached a host the plugin didn't name")
	}
	allowed := load(t, fmt.Sprintf(`{"id":"x","permissions":{"network":[%q]}}`, u.Hostname()), source)
	got, err := allowed.Suggest("a")
	if err != nil || len(got) != 1 {
		t.Errorf("fetch to the host named: %v, %v", got, err)
	}
}

const spawnSource = `
export async function suggest({ query }) {
  const proc = host.spawn(host.setting("program"), []);
  proc.write(query + "\n");
  proc.closeStdin();
  const line = await proc.readLine();
  const end = await proc.readLine();
  return [line, String(end), String(await proc.wait())];
}`

// Starting programs takes both the permission and the listener's say-so.
func TestSpawnNeedsPermissionAndGrant(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(echoEnv, "1")

	asked := load(t, plain, spawnSource)
	asked.Grant(true)
	asked.SetSetting("program", self)
	if _, err := asked.Suggest("a"); err == nil || !strings.Contains(err.Error(), "permission") {
		t.Errorf("without the permission: %v", err)
	}

	p := load(t, `{"id":"x","permissions":{"process":true}}`, spawnSource)
	p.SetSetting("program", self)
	if _, err := p.Suggest("a"); err == nil || !strings.Contains(err.Error(), "allowed") {
		t.Errorf("without the grant: %v", err)
	}
	if err := p.Grant(true); err != nil {
		t.Fatal(err)
	}
	got, err := p.Suggest("hi")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "echo:hi,null,0" {
		t.Errorf("got %v", got)
	}
}

// Programs end with the plugin, rather than staying around after the app.
func TestCloseEndsPrograms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal 0 doesn't tell here")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(echoEnv, "1")
	p := load(t, `{"id":"x","permissions":{"process":true}}`, `
export function suggest() { return [String(host.spawn(host.setting("program"), []).pid)]; }`)
	p.Grant(true)
	p.SetSetting("program", self)
	got, err := p.Suggest("a")
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	fmt.Sscan(got[0], &pid)
	v := p.vm
	p.Close()
	v.mu.Lock()
	left := len(v.programs)
	v.mu.Unlock()
	if left != 0 {
		t.Fatalf("%d programs left", left)
	}
	proc, _ := os.FindProcess(pid)
	deadline := time.Now().Add(2 * time.Second)
	for proc.Signal(syscall.Signal(0)) == nil {
		if time.Now().After(deadline) {
			t.Fatal("the program is still running")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// What a plugin stores and the choice to let it run programs outlive the app.
func TestStorageAndGrantAreKept(t *testing.T) {
	dir, data := write(t, `{"id":"x","permissions":{"process":true}}`, `
export function signOut() { host.storage.set("token", { value: "abc" }); }
export function suggest() { const t = host.storage.get("token"); return [t ? t.value : "none"]; }`)
	p, err := Load(dir, data)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := p.Suggest("a"); got[0] != "none" {
		t.Fatalf("before setting: %v", got)
	}
	if err := p.SignOut(); err != nil {
		t.Fatal(err)
	}
	p.Grant(true)
	p.Close()
	again, err := Load(dir, data)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if got, err := again.Suggest("a"); err != nil || got[0] != "abc" {
		t.Fatalf("after loading again: %v, %v", got, err)
	}
	if !again.Granted() {
		t.Fatal("the grant wasn't kept")
	}
}

// A browser sign-in comes back with its query, and a plugin can only open pages.
func TestSignInRedirect(t *testing.T) {
	var opened string
	SetOpenURL(func(u string) error { opened = u; return nil })
	defer SetOpenURL(func(string) error { return nil })
	p := load(t, `{"id":"x","permissions":{"network":["example.com"]}}`, `
export async function signIn() {
  host.openURL("https://example.com/auth");
  const q = await host.waitForRedirect({ port: 48917, path: "/cb", timeout: 3000 });
  return { id: q.code, name: "me" };
}
export function suggest() { host.openURL("file:///etc/passwd"); return []; }`)
	type result struct {
		a   *music.Account
		err error
	}
	ch := make(chan result, 1)
	go func() {
		a, err := p.SignIn()
		ch <- result{a, err}
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := http.Get("http://127.0.0.1:48917/cb?code=abc")
		if err == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	r := <-ch
	if r.err != nil || r.a.ID != "abc" {
		t.Fatalf("sign in: %+v, %v", r.a, r.err)
	}
	if opened != "https://example.com/auth" {
		t.Fatalf("opened %q", opened)
	}
	if _, err := p.Suggest("a"); err == nil {
		t.Fatal("opened something that's not a page")
	}
}

// One broken plugin is left out, not the end of the others, and a missing name
// or way of playing has a default.
func TestDiscover(t *testing.T) {
	root := t.TempDir()
	for name, files := range map[string][2]string{
		"good":   {`{"id":"good"}`, `export function search() {}`},
		"syntax": {`{"id":"syntax"}`, `export function (`},
		"json":   {`{`, ``},
	} {
		d := filepath.Join(root, name)
		os.MkdirAll(d, 0o700)
		os.WriteFile(filepath.Join(d, "plugin.json"), []byte(files[0]), 0o600)
		os.WriteFile(filepath.Join(d, "index.js"), []byte(files[1]), 0o600)
	}
	ps := Discover(root, t.TempDir())
	if len(ps) != 1 || ps[0].Info().ID != "good" {
		t.Fatalf("got %d plugins", len(ps))
	}
	if i := ps[0].Info(); i.Name != "good" || i.Playback != music.PlaybackStream {
		t.Fatalf("defaults: %+v", i)
	}
}

// A plugin can clear a folder of its own data, as signing out does, but nothing
// outside it, nor all of it, which holds the app's records of it too.
func TestRemoveDataDir(t *testing.T) {
	p := load(t, plain, `
export function suggest({ query }) { host.removeDataDir(query); return []; }`)
	if _, err := p.Suggest("x"); err != nil {
		t.Fatal(err) // starting it makes the data directory
	}
	inside := filepath.Join(p.data, "cache", "file")
	os.MkdirAll(filepath.Dir(inside), 0o700)
	os.WriteFile(inside, []byte("x"), 0o600)
	outside := filepath.Join(filepath.Dir(p.data), "other")
	os.WriteFile(outside, []byte("x"), 0o600)

	for _, bad := range []string{"..", "../other", ".", "", "/etc"} {
		if _, err := p.Suggest(bad); err == nil {
			t.Errorf("removed %q", bad)
		}
	}
	if _, err := os.Stat(outside); err != nil {
		t.Errorf("outside its data: %v", err)
	}
	if _, err := os.Stat(inside); err != nil {
		t.Errorf("after the refusals: %v", err)
	}
	if _, err := p.Suggest("cache"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(inside); !os.IsNotExist(err) {
		t.Errorf("still there: %v", err)
	}
}
