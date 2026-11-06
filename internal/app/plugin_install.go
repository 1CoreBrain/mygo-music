package app

import (
	"errors"
	"log"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-music/internal/plugin"
)

// Plugins install from GitHub repositories, and update from where they came, or
// from a folder or zip on this machine.

// These reach GitHub, and the disk; tests replace them.
var (
	fetchPlugin  = plugin.Fetch
	latestPlugin = plugin.Latest
	openPlugin   = plugin.Open
)

// pickPlugin asks the listener for a plugin's folder, or a zip of one, with a
// native dialog, on a goroutine, for it blocks; tests replace it.
var pickPlugin = func(a *app, folder bool, got func(string)) {
	go func() {
		opts := mygo.OpenDialogOptions{Title: T("Install a Plugin"), ButtonLabel: T("Install"), Directory: folder}
		if !folder {
			opts.Filters = []mygo.FileFilter{{Name: T("Zip Archive"), Extensions: []string{"zip"}}}
		}
		paths, err := mygo.Dialog.Open(opts)
		if err == nil && len(paths) > 0 {
			a.post(func() { got(paths[0]) })
		}
	}()
}

// pluginCheckEvery is how often plugins are checked for new versions: once a day,
// as the app is. GitHub's API allows few requests without an account.
const pluginCheckEvery = 24 * time.Hour

// pluginInstaller is what's going on with installing plugins.
type pluginInstaller struct {
	repo     string          // what's written in the field
	busy     bool            // one's downloading
	err      string          // what went wrong last
	pending  *plugin.Pending // one downloaded, for the listener to allow
	updating map[string]bool
}

// installFromRepo downloads what's written in the field, to ask about it.
func (a *app) installFromRepo() {
	in := &a.installer
	repo, err := plugin.ParseRepo(in.repo)
	if err != nil {
		in.err = T("Write a repository as owner/repo, or its address on GitHub")
		return
	}
	in.busy, in.err = true, ""
	go func() {
		pending, err := fetchPlugin(repo, pluginsDir())
		a.post(func() {
			in.busy = false
			if err != nil {
				in.err = installError(err)
				return
			}
			in.pending = pending
		})
	}()
}

// installFromDisk asks for a plugin's folder, or a zip of one, and copies it in
// to ask about it, as one from GitHub is asked about.
func (a *app) installFromDisk(folder bool) {
	in := &a.installer
	pickPlugin(a, folder, func(path string) {
		in.busy, in.err = true, ""
		a.cancelInstall()
		go func() {
			pending, err := openPlugin(path, pluginsDir())
			a.post(func() {
				in.busy = false
				if err != nil {
					in.err = err.Error()
					return
				}
				in.pending = pending
			})
		}()
	})
}

// installError says what went wrong, in the listener's language if it can.
func installError(err error) string {
	if errors.Is(err, plugin.ErrNoRepo) {
		return T("There's no such public repository on GitHub")
	}
	return err.Error()
}

// confirmInstall installs the plugin the listener allowed.
func (a *app) confirmInstall() {
	in := &a.installer
	pending := in.pending
	if pending == nil {
		return
	}
	in.pending, in.busy = nil, true
	go func() {
		err := pending.Install(pluginsDir())
		a.post(func() {
			in.busy = false
			if err != nil {
				in.err = err.Error()
				return
			}
			in.repo = ""
			a.pluginInstalled(pending.Manifest.ID)
		})
	}()
}

// cancelInstall forgets the plugin the listener didn't want.
func (a *app) cancelInstall() {
	in := &a.installer
	if in.pending != nil {
		go in.pending.Discard()
		in.pending = nil
	}
}

// pluginOf is a library's plugin, if it's one that's installed.
func pluginOf(lib *library) *plugin.Plugin {
	p, _ := lib.src.(*plugin.Plugin)
	return p
}

// pluginInstalled takes in what was just installed: a new plugin, or a new
// version of one, which takes its place without opening the app again.
func (a *app) pluginInstalled(id string) {
	in := &a.installer
	delete(a.settings.PluginUpdates, id)
	a.settings.save()
	delete(in.updating, id)
	p, err := plugin.Load(filepath.Join(pluginsDir(), id), userDataDir("plugin-data"))
	if err != nil {
		in.err = err.Error()
		return
	}
	lib := newLibrary(p)
	at := slices.IndexFunc(a.allLibs, func(l *library) bool { return l.id == id })
	if at < 0 {
		a.allLibs = append(a.allLibs, lib)
		a.plugins = append(a.plugins, p)
		a.enabledChanged()
		a.redraw()
		return
	}
	old := a.allLibs[at]
	a.allLibs[at] = lib
	if was := pluginOf(old); was != nil {
		a.plugins = slices.DeleteFunc(a.plugins, func(q *plugin.Plugin) bool { return q == was })
		// Its code is gone from under it: it stops, and the new one starts.
		go was.Close()
	}
	a.plugins = append(a.plugins, p)
	a.enabledChanged()
	if a.library == old {
		a.library = lib
		a.startLibrary(lib)
		a.load(a.route)
	}
	a.redraw()
}

// removePlugin takes a plugin out, what it keeps of its own staying for if it
// comes back. The last one on stays, as there would be nothing to show.
func (a *app) removePlugin(lib *library) {
	p := pluginOf(lib)
	if p == nil || (len(a.libs) == 1 && a.libs[0] == lib) {
		return
	}
	a.allLibs = slices.DeleteFunc(slices.Clone(a.allLibs), func(l *library) bool { return l == lib })
	a.plugins = slices.DeleteFunc(a.plugins, func(q *plugin.Plugin) bool { return q == p })
	a.enabledChanged()
	if a.library == lib {
		a.switchSource(a.libs[0].id)
	}
	delete(a.settings.PluginUpdates, lib.id)
	a.settings.save()
	go func() {
		if err := p.Remove(); err != nil {
			log.Printf("removing plugin %s: %v", lib.id, err)
		}
	}()
	a.redraw()
}

// keepCheckingPlugins checks for new versions now if it's time, and from then on
// as the day comes round, for as long as the app runs.
func (a *app) keepCheckingPlugins() {
	a.checkPluginUpdates()
	time.AfterFunc(pluginCheckEvery, func() { a.post(a.keepCheckingPlugins) })
}

// checkPluginUpdates asks where the plugins came from what versions are there now,
// unless it's been done in the last day, what it found then being kept. What can't
// be reached just shows nothing new.
func (a *app) checkPluginUpdates() {
	if !a.settings.PluginsChecked.IsZero() && time.Since(a.settings.PluginsChecked) < pluginCheckEvery {
		return
	}
	origins := map[string]plugin.Origin{}
	for _, lib := range a.allLibs {
		if p := pluginOf(lib); p != nil {
			if o := p.Origin(); o != nil {
				origins[lib.id] = *o
			}
		}
	}
	if len(origins) == 0 {
		return
	}
	a.settings.PluginsChecked = time.Now()
	a.settings.save()
	go func() {
		found := map[string]string{}
		for id, o := range origins {
			rel, err := latestPlugin(o.Repo)
			if err != nil {
				log.Printf("checking plugin %s: %v", id, err)
				continue
			}
			if plugin.IsNewer(rel.Version, o.Version) {
				found[id] = rel.Version
			}
		}
		a.post(func() {
			a.settings.PluginUpdates = found
			a.settings.save()
			a.redraw()
		})
	}()
}

// updatePlugin installs the new version of a plugin. One that asks for more than
// the listener allowed asks them again, first.
func (a *app) updatePlugin(lib *library) {
	p := pluginOf(lib)
	if p == nil {
		return
	}
	o := p.Origin()
	if o == nil {
		return
	}
	in := &a.installer
	if in.updating == nil {
		in.updating = map[string]bool{}
	}
	in.updating[lib.id], in.err = true, ""
	id := lib.id
	had := p.Permissions()
	go func() {
		pending, err := fetchPlugin(o.Repo, pluginsDir())
		if err == nil && asksForMore(had, pending.Manifest.Permissions) {
			a.post(func() {
				delete(in.updating, id)
				in.pending = pending
			})
			return
		}
		if err == nil {
			err = pending.Install(pluginsDir())
		}
		a.post(func() {
			delete(in.updating, id)
			if err != nil {
				in.err = installError(err)
				return
			}
			a.pluginInstalled(id)
		})
	}()
}

// asksForMore reports whether a new version asks for anything the old didn't.
func asksForMore(had, wants plugin.Permissions) bool {
	if wants.Process && !had.Process {
		return true
	}
	for _, host := range wants.Network {
		if !slices.Contains(had.Network, host) && !slices.Contains(had.Network, "*") {
			return true
		}
	}
	return false
}

// installSettings is the way to install a plugin from GitHub, and what's asked
// about one on its way.
func (a *app) installSettings(c *ui.Context, row settingsRow, block settingsBlock, p palette) {
	in := &a.installer
	row(T("Install from GitHub"), T("A repository with a plugin, as owner/repo"), func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			field := ui.TextInput(c.Key("repo"), &in.repo).Label(T("Install from GitHub")).Placeholder("owner/repo").Width(180).FontSize(13)
			label := T("Install")
			if in.busy {
				label = T("Downloading…")
			}
			if ui.Button(c.Key("install"), label).Disabled(in.busy || strings.TrimSpace(in.repo) == "").Clicked() || field.Submitted() && !in.busy {
				a.installFromRepo()
			}
		})
	})
	row(T("Install from This Computer"), T("A plugin's folder, or a zip of one"), func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.Button(c.Key("install-folder"), T("Folder…")).Disabled(in.busy).Clicked() {
				a.installFromDisk(true)
			}
			if ui.Button(c.Key("install-zip"), T("Zip…")).Disabled(in.busy).Clicked() {
				a.installFromDisk(false)
			}
		})
	})
	if in.err != "" {
		block(in.err, func() {
			ui.Text(c, in.err).FontSize(12).TextColor(p.accent).LineHeight(1.4).Padding(10, 16)
		})
	}
	if pending := in.pending; pending != nil {
		block(pending.Manifest.Name, func() { a.installConfirm(c, pending, p) })
	}
}

// installConfirm says what a plugin is and what it asks for, before it's in.
func (a *app) installConfirm(c *ui.Context, pending *plugin.Pending, p palette) {
	ui.Column(c).Padding(12, 16).Gap(8).Background(p.hover).Children(func() {
		ui.Text(c, strings.TrimSpace(pending.Manifest.Name+" "+pending.Version)).FontSize(13.5).FontWeight(600)
		from := pending.Repo
		if from == "" {
			from = pending.Path
		}
		ui.Text(c, Tf("From %s", from)).FontSize(12).TextColor(p.muted).LineHeight(1.4)
		perms := pending.Manifest.Permissions
		switch {
		case slices.Contains(perms.Network, "*"):
			ui.Text(c, T("Can reach any website")).FontSize(12).TextColor(p.muted)
		case len(perms.Network) > 0:
			ui.Text(c, Tf("Can reach %s", strings.Join(perms.Network, ", "))).FontSize(12).TextColor(p.muted).LineHeight(1.4)
		default:
			ui.Text(c, T("Asks for no network")).FontSize(12).TextColor(p.muted)
		}
		if perms.Process {
			ui.Text(c, T("This plugin wants to start programs, which can do anything you can. Only allow it if you trust it.")).
				FontSize(12).TextColor(p.accent).LineHeight(1.4)
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c.Key("cancel-install"), T("Cancel")).Clicked() {
				a.cancelInstall()
			}
			if ui.PrimaryButton(c.Key("confirm-install"), T("Install")).Label(Tf("Install %s", pending.Manifest.Name)).Clicked() {
				a.confirmInstall()
			}
		})
	})
}

// pluginOriginRow says where a plugin came from, with what can be done about that.
func (a *app) pluginOriginRow(c *ui.Context, lib *library, p palette) {
	plug := pluginOf(lib)
	if plug == nil {
		return
	}
	o := plug.Origin()
	if o == nil {
		return
	}
	in := &a.installer
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, o.Repo+" · "+o.Version).FontSize(12).TextColor(p.muted).SingleLine().Grow(1).Shrink(1).MinWidth(0)
		if version, ok := a.settings.PluginUpdates[lib.id]; ok && plugin.IsNewer(version, o.Version) {
			label := Tf("Update to %s", version)
			if in.updating[lib.id] {
				label = T("Updating…")
			}
			if ui.PrimaryButton(c.Key("update"), label).Disabled(in.updating[lib.id]).Clicked() {
				a.updatePlugin(lib)
			}
		}
		last := len(a.libs) == 1 && a.libs[0] == lib
		if ui.Button(c.Key("remove-plugin"), T("Remove")).Disabled(last).Clicked() {
			a.removePlugin(lib)
		}
	})
}
