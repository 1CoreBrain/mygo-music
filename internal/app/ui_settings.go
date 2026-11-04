package app

import (
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-music/internal/i18n"
	"mygo-music/internal/local"
	"mygo-music/internal/music"
	"mygo-music/internal/plugin"
)

// openSettings shows the settings window, opening it the first time.
func (a *app) openSettings() {
	// It lists all the sources, with what each can do.
	for _, lib := range a.libs {
		a.learn(lib)
	}
	if a.win == nil {
		return // no app to show a window in, as in tests
	}
	if a.settingsWin != nil {
		a.settingsWin.Show()
		a.settingsWin.Focus()
		a.measureCache()
		return
	}
	a.measureCache()
	a.settingsWin = mygo.NewWindow(mygo.WindowOptions{
		Title:           T("Settings"),
		Width:           600,
		Height:          720,
		MinWidth:        520,
		MinHeight:       420,
		StateKey:        "settings",
		DisableMaximize: true,
		DisableMinimize: true,
		// The app's menu has nothing for this window: on Linux and
		// Windows it stays out of sight until Alt shows it.
		AutoHideMenuBar: true,
		BackgroundColor: "light-dark(#f4f4f6, #1c1c1f)",
		Content:         ui.View(a.settingsView),
	})
	a.settingsWin.OnClosed(func() {
		a.settingsWin = nil
		a.syncDock()
	})
	a.syncDock()
}

// changed applies the settings everywhere and keeps them.
func (a *app) changed() {
	a.settings.save()
	a.settings.applyAppearance()
	if a.notch != nil {
		a.notch.setEnabled(a.settings.Notch)
	}
	a.redraw()
}

// setLanguage speaks another language from now on: the views say it as they
// redraw; the menus, and what the sources wrote out in it, are made anew.
func (a *app) setLanguage(code string) {
	a.settings.Language = code
	i18n.SetLanguage(code)
	a.changed()
	if a.win != nil {
		a.setMenu()
		a.refreshTray()
	}
	if a.settingsWin != nil {
		a.settingsWin.SetTitle(T("Settings"))
	}
	if lib := a.libOf(local.ID); lib != nil {
		a.libraryChanged(lib)
	}
}

// choice is a select of labels, to translate, reporting the index of the one
// chosen once it changes.
func choice(c *ui.Context, key, label string, labels []string, at int) (int, bool) {
	names := make([]string, len(labels))
	for i, l := range labels {
		names[i] = T(l)
	}
	chosen := names[max(at, 0)]
	if ui.Select(c.Key(key), &chosen, names).Label(label).Width(170).Changed() {
		for i, n := range names {
			if n == chosen {
				return i, true
			}
		}
	}
	return at, false
}

// settingsRow is a setting of the settings window: what it is, what it means,
// and its control.
type settingsRow func(label, desc string, control func())

func (a *app) settingsView(c *ui.Context) {
	t := *c.Theme()
	p := paletteFor(&t)
	t.Accent, t.AccentText = p.accent, p.onAccent
	c.SetTheme(&t)
	c.Root().Background(p.sidebar)
	c.OnShortcut(ui.Cmd, ui.KeyW, func() { a.settingsWin.Close() })
	s := &a.settings

	ui.Scroll(c).Fill().Padding(24, 28, 32, 28).Gap(8).Children(func() {
		group := func(title string, rows func()) {
			ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(p.muted).Margin(14, 4, 4, 4)
			ui.Column(c).Radius(12).Background(p.bg).Border(1, p.hairline).Dividers(1, p.hairline).Children(rows)
		}
		var row settingsRow = func(label, desc string, control func()) {
			ui.Row(c).Padding(12, 16).Gap(16).AlignItems(ui.Center).MinHeight(48).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, label).FontSize(13.5).FontWeight(500)
					if desc != "" {
						ui.Text(c, desc).FontSize(12).TextColor(p.muted).LineHeight(1.4)
					}
				})
				control()
			})
		}

		group(T("General"), func() {
			row(T("Language"), "", func() {
				names := make([]string, len(i18n.Languages))
				at := 0
				for i, l := range i18n.Languages {
					names[i] = l.Name
					if l.Code == s.Language {
						at = i
					}
				}
				chosen := names[at]
				if ui.Select(c.Key("language"), &chosen, names).Label(T("Language")).Width(170).Changed() {
					for _, l := range i18n.Languages {
						if l.Name == chosen {
							a.setLanguage(l.Code)
						}
					}
				}
			})
			// Only on macOS: elsewhere the icon is always in the notification
			// area.
			if runtime.GOOS == "darwin" {
				row(T("App Icon"), T("Shown in the menu bar, it opens the window again after you close it"), func() {
					labels := make([]string, len(appIcons))
					at := 0
					for i, ic := range appIcons {
						labels[i] = ic.label
						if ic.id == s.AppIcon {
							at = i
						}
					}
					if i, ok := choice(c, "app-icon", T("App Icon"), labels, at); ok {
						s.AppIcon = appIcons[i].id
						a.changed()
						a.changeAppIcon()
					}
				})
			}
		})

		group(T("Sources"), func() { a.sourceSettings(c, row, p) })

		// Only on macOS, the one with a notch player.
		if a.notch != nil {
			group(T("Notch Player"), func() {
				row(T("Show the Notch Player"), T("A small player at the notch on top of the screen, which opens as the pointer rests on it"), func() {
					if ui.Switch(c.Key("notch"), &s.Notch).Label(T("Show the Notch Player")).Changed() {
						a.changed()
					}
				})
				row(T("Show Activity While Playing"), T("The cover and levels beside the notch, which can cover menu bar icons there"), func() {
					if ui.Switch(c.Key("activity"), &s.NotchActivity).Label(T("Show Activity While Playing")).Disabled(!s.Notch).Changed() {
						a.changed()
					}
				})
				row(T("Keep It Showing Without a Notch"), T("When off, move the pointer to the top center of the screen to bring it up, without covering the menu bar"), func() {
					if ui.Switch(c.Key("virtual"), &s.NotchVirtual).Label(T("Keep It Showing Without a Notch")).Disabled(!s.Notch).Changed() {
						a.changed()
					}
				})
			})
		}

		group(T("Playback"), func() {
			row(T("Quality"), T("What's asked of sources that have a choice, from the next song on"), func() {
				labels := make([]string, len(qualityLevels))
				at := 1
				for i, q := range qualityLevels {
					labels[i] = q.label
					if q.level == s.Quality {
						at = i
					}
				}
				if i, ok := choice(c, "quality", T("Quality"), labels, at); ok {
					s.Quality = qualityLevels[i].level
					a.changed()
				}
			})
			if a.anyCan("comments") {
				row(T("Top Comments in Quiet Parts"), T("In intros, breaks and outros, a song's top comments show in place of the lyrics"), func() {
					if ui.Switch(c.Key("hot-comments"), &s.HotComments).Label(T("Top Comments in Quiet Parts")).Changed() {
						a.changed()
					}
				})
			}
			row(T("Even Out Loudness"), T("Adjust each song's volume by what its source measured of it, for them to sound as loud as each other"), func() {
				if ui.Switch(c.Key("normalize"), &s.Normalize).Label(T("Even Out Loudness")).Changed() {
					a.pl.SetVolume(a.volume)
					a.changed()
				}
			})
		})

		group(T("Cache"), func() {
			row(T("Song Cache"), Tf("Songs from elsewhere are kept here to play again at once, without the network. %s", a.cacheUsage()), func() {
				labels := make([]string, len(cacheLimits))
				at := 2
				for i, l := range cacheLimits {
					labels[i] = l.label
					if l.mb == s.CacheMB {
						at = i
					}
				}
				if i, ok := choice(c, "cache", T("Song Cache"), labels, at); ok {
					s.CacheMB = cacheLimits[i].mb
					a.changed()
					a.applyCacheLimit()
				}
			})
			row(T("Clear the Cache"), T("Delete all the songs kept here"), func() {
				if ui.Button(c.Key("clear-cache"), T("Clear")).Disabled(a.cacheUsed <= 0).Clicked() {
					a.clearCache()
				}
			})
		})

		group(T("Appearance"), func() {
			row(T("Appearance"), "", func() {
				modes := []string{"system", "light", "dark"}
				i := 0
				for k, m := range modes {
					if m == s.Appearance {
						i = k
					}
				}
				if segmented(c.Key("appearance"), &i, T("System"), T("Light"), T("Dark")) {
					s.Appearance = modes[i]
					a.changed()
				}
			})
		})

		group(T("Experiments"), func() {
			row(T("Smoke on the Progress Bar"), T("The progress bar turns into a light trailing smoke, moving with the music. Uses a little more power."), func() {
				if ui.Switch(c.Key("smoke"), &s.Smoke).Label(T("Smoke on the Progress Bar")).Changed() {
					a.changed()
				}
			})
			row(T("Fog Backdrop"), T("Now Playing's backdrop turns into fog in the cover's colors, rising with the bass. Still being polished; uses a little more power."), func() {
				if ui.Switch(c.Key("fog"), &s.Fog).Label(T("Fog Backdrop")).Changed() {
					a.changed()
				}
			})
		})

		if a.updates {
			group(T("Updates"), func() {
				desc := T("Check for a new version once a day")
				if t := lastUpdateCheck(); !t.IsZero() {
					desc = Tf("Check for a new version once a day. Last checked: %s", i18n.Ago(t, time.Now()))
				}
				row(T("Check for Updates Automatically"), desc, func() {
					on := updateChecks()
					if ui.Switch(c.Key("update-checks"), &on).Label(T("Check for Updates Automatically")).Changed() {
						setUpdateChecks(on)
					}
				})
				row(T("Download and Install Automatically"), T("Download new versions in the background, to take effect the next time the app opens"), func() {
					on := updateDownloads()
					if ui.Switch(c.Key("update-downloads"), &on).Label(T("Download and Install Automatically")).Disabled(!updateChecks()).Changed() {
						setUpdateDownloads(on)
					}
				})
				row(Tf("Current Version %s", mygo.App.Version()), "", func() {
					if ui.Button(c.Key("check-updates"), T("Check for Updates")).Clicked() {
						checkForUpdates()
					}
				})
			})
		}
	})
}

// anyCan reports whether any source does what's named.
func (a *app) anyCan(what string) bool {
	for _, lib := range a.libs {
		if lib.can(what) {
			return true
		}
	}
	return false
}

// sourceSettings lists the sources, each with what it asks of the listener,
// and where more of them come from.
func (a *app) sourceSettings(c *ui.Context, row settingsRow, p palette) {
	for _, lib := range a.libs {
		if lib.id == local.ID {
			a.localSettings(c, row, p)
			continue
		}
		a.pluginSettings(c.Key(lib.id), lib, p)
	}
	row(T("Plugins"), Tf("Drop plugins in %s and open the app again. A plugin reaches only what it asks for, and starts programs only if you allow it.", pluginsDir()), func() {
		if ui.Button(c.Key("reveal-plugins"), T("Show Folder")).Clicked() {
			revealFolder(pluginsDir())
		}
	})
}

// revealFolder shows a folder in the file manager, making it if need be;
// tests replace it.
var revealFolder = func(dir string) {
	os.MkdirAll(dir, 0o755)
	mygo.Shell.OpenPath(dir)
}

// localSettings are the folders of the built-in source.
func (a *app) localSettings(c *ui.Context, row settingsRow, p palette) {
	if a.local == nil {
		return
	}
	folders := a.local.Folders()
	desc := T("Add the folders where your music lives")
	if a.local.Scanning() {
		desc = T("Looking for music…")
	} else if len(folders) > 0 {
		desc = Tf("%d songs found", a.local.TrackCount())
	}
	row(T("Local Files"), desc, func() {
		ui.Row(c).Gap(8).Children(func() {
			if len(folders) > 0 && ui.Button(c.Key("rescan"), T("Rescan")).Disabled(a.local.Scanning()).Clicked() {
				a.local.Rescan()
			}
			if ui.Button(c.Key("add-folder"), T("Add Folder…")).Clicked() {
				a.addMusicFolder()
			}
		})
	})
	for _, dir := range folders {
		ui.Row(c.Key(dir)).Padding(8, 16).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconFolder).Size(15, 15).TextColor(p.muted)
			ui.Text(c, dir).FontSize(12.5).SingleLine().Grow(1).Shrink(1).MinWidth(0)
			if ui.Button(c.Key("remove"), T("Remove")).Clicked() {
				a.local.RemoveFolder(dir)
			}
		})
	}
}

// pluginSettings is a plugin: who's signed in, and what it asks of the listener.
func (a *app) pluginSettings(c *ui.Context, lib *library, p palette) {
	info := lib.src.Info()
	ui.Column(c).Padding(12, 16).Gap(10).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).MinHeight(24).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
				name := lib.name()
				if info.Version != "" {
					name += " " + info.Version
				}
				ui.Text(c, name).FontSize(13.5).FontWeight(500)
				switch {
				case lib.startErr != "":
					ui.Text(c, Tf("Couldn't start: %s", lib.startErr)).FontSize(12).TextColor(p.accent).LineHeight(1.4)
				case lib.needsSetup:
					ui.Text(c, T("Fill these in for it to work")).FontSize(12).TextColor(p.accent).LineHeight(1.4)
				case lib.account != nil:
					ui.Text(c, Tf("Signed in as %s", lib.account.Name)).FontSize(12).TextColor(p.muted)
				}
			})
			switch {
			case lib.account != nil && lib.can("signOut"):
				if ui.Button(c.Key("sign-out"), T("Sign Out")).Clicked() {
					a.signOut(lib)
				}
			case lib.account == nil && lib.can("signIn"):
				if ui.PrimaryButton(c.Key("sign-in"), T("Sign In")).Clicked() {
					a.promptSignIn(lib)
					if a.win != nil {
						a.win.Invalidate()
					}
				}
			}
		})
		if g, ok := lib.src.(guarded); ok {
			a.pluginPermissions(c.Key("permissions"), lib, g, p)
		}
		for _, st := range info.Settings {
			a.sourceSetting(c.Key(st.Key), lib, st, p)
		}
	})
}

// guarded is a source with permissions: a plugin.
type guarded interface {
	Permissions() plugin.Permissions
	Granted() bool
	Grant(on bool) error
}

// pluginPermissions says what a plugin can reach, and asks before it may start
// programs, which can do anything the listener can. It may not until they say.
func (a *app) pluginPermissions(c *ui.Context, lib *library, g guarded, p palette) {
	perms := g.Permissions()
	if len(perms.Network) > 0 {
		reach := Tf("Can reach %s", strings.Join(perms.Network, ", "))
		if slices.Contains(perms.Network, "*") {
			reach = T("Can reach any website")
		}
		ui.Text(c, reach).FontSize(12).TextColor(p.muted).LineHeight(1.4)
	}
	if !perms.Process {
		return
	}
	granted := g.Granted()
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, T("This plugin wants to start programs, which can do anything you can. Only allow it if you trust it.")).
			FontSize(12).TextColor(p.accent).LineHeight(1.4).Grow(1).Shrink(1).MinWidth(0)
		label := T("Allow Programs")
		if granted {
			label = T("Stop Allowing")
		}
		if ui.Button(c.Key("grant"), label).Clicked() {
			a.grantPrograms(lib, g, !granted)
		}
	})
}

// grantPrograms changes the choice off the main thread, as the plugin starts
// over for it, and has the source load again with what it can do now.
func (a *app) grantPrograms(lib *library, g guarded, on bool) {
	go func() {
		err := g.Grant(on)
		a.post(func() {
			if err != nil {
				a.actionFailed(lib, err, T("Couldn't change that"))
				return
			}
			a.libraryChanged(lib)
		})
	}()
}

// sourceSetting is a text field for what a source asks of the listener. It's
// kept as it's submitted, or as the focus leaves it.
func (a *app) sourceSetting(c *ui.Context, lib *library, st music.Setting, p palette) {
	key := lib.id + "\x00" + st.Key
	text := a.settingText[key]
	if text == nil {
		v := lib.src.Setting(st.Key)
		text = &v
		a.settingText[key] = text
	}
	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, st.Label).FontSize(12.5).FontWeight(500)
		in := ui.TextInput(c.Key("value"), text).Label(st.Label).FontSize(13)
		if in.Submitted() || !in.Focused() && *text != lib.src.Setting(st.Key) {
			a.keepSetting(lib, st.Key, *text)
		}
		if st.Help != "" {
			ui.Text(c, st.Help).FontSize(11.5).TextColor(p.muted).LineHeight(1.4)
		}
		if st.Link != "" {
			ui.RichText(c).FontSize(11.5).Children(func() { ui.Link(c, st.Link, st.Link) })
		}
	})
}

// keepSetting gives a source a setting, off the main thread, and loads anew
// what it may now be able to show.
func (a *app) keepSetting(lib *library, key, value string) {
	if lib.src.Setting(key) == value {
		return
	}
	go func() {
		err := lib.src.SetSetting(key, value)
		a.post(func() {
			if err != nil {
				a.toast = Tf("Couldn't keep the setting: %s", err.Error())
				return
			}
			lib.needsSetup, lib.askedSetup = false, false
			a.libraryChanged(lib)
			a.loadAccount(lib)
		})
	}()
}
