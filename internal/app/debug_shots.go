package app

import (
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
)

// With MM_SHOTS set to a directory, the app walks through its views and saves
// what each window shows there, for checking the design. It leaves the
// listener's settings as they were.
func init() {
	dir := os.Getenv("MM_SHOTS")
	if dir == "" {
		return
	}
	shotHook = func(a *app) {
		a.settings.path = ""
		go func() {
			shot := func(name string, w *mygo.Window) {
				png, err := w.CapturePage()
				if err == nil {
					os.WriteFile(filepath.Join(dir, name+".png"), png, 0o644)
				}
			}
			step := func(d time.Duration, fn func()) {
				time.Sleep(d)
				mygo.RunOnMain(func() { fn(); a.redraw() })
			}
			mygo.RunOnMain(func() { a.win.Show(); a.win.Focus() })
			time.Sleep(3 * time.Second)
			shot("1-home", a.win)
			step(0, func() {
				a.volume = 0.15
				a.pl.SetVolume(a.volume)
				if tracks := a.library.recent; len(tracks) > 0 {
					a.pl.PlayQueue(tracks, 0)
				}
			})
			time.Sleep(3 * time.Second)
			shot("2-playing", a.win)
			if a.pl.Current() != nil {
				step(0, func() { a.nowPlaying = true })
				time.Sleep(3 * time.Second)
				shot("3-now-playing", a.win)
				step(0, func() { a.npPane = paneQueue })
				time.Sleep(2 * time.Second)
				shot("4-queue", a.win)
			}
			mygo.RunOnMain(mygo.App.Quit)
		}()
	}
}
