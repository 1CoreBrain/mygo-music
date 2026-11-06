# Changelog

All notable changes to MyGo Music are written down here. Versions follow [Semantic Versioning](https://semver.org).

## [0.1.11] - 2026-11-07

### Changed

- Checking plugins for updates uses your GitHub token, from `GITHUB_TOKEN`, `GH_TOKEN` or `gh auth token`, so frequent checks no longer run into GitHub's limit. Plugins from private repositories can be installed and updated with it too.
- Updating a plugin installs the version its check found, rather than asking GitHub again.

### Fixed

- On Windows and Linux, the + in Settings > Plugins no longer sits under the window's close button.

## [0.1.10] - 2026-11-06

### Fixed

- Chinese titles, artists, albums and lyrics in older MP3 tags, written as GBK, no longer show as garbled Latin letters, and ID3v1 tags with accents or Chinese are no longer left blank. Your library is read again once to pick this up.

## [0.1.9] - 2026-11-06

### Added

- Check a plugin for updates whenever you like, from its ⋯ menu in Settings > Plugins, instead of waiting for the daily check. Its row says whether it's up to date.
- A plugin's repository in Settings > Plugins is a link to it on GitHub.

### Changed

- The warning about a plugin starting programs is no longer shown in red.

## [0.1.8] - 2026-11-06

### Added

- A plugin can say the oldest version of MyGo Music it works with, as `minAppVersion` in `plugin.json`. An older app won't install it, and says to update first.
- Plugins can be translated: the app shows a plugin's name, settings and sidebar in your language from its `locales/zh-CN.json`, and plugins can translate their pages with `host.t` and `host.language`. Changing the language reloads the plugin's page in it.

## [0.1.7] - 2026-11-06

### Changed

- A plugin's `process` permission can list the programs it may start, such as `["bin/helper-*", "ffmpeg"]`, and it can't start any other, even once you've allowed it. Installing or allowing such a plugin names those programs. `true` still means any program.
- Updating a plugin asks again when the new version wants to start a program the old one didn't.

## [0.1.6] - 2026-11-06

### Fixed

- Signing in from Settings opens the sign-in dialog, and a plugin's sign-in page, over the Settings window instead of behind it, over the main window.
- Settings no longer offers Sign In for a plugin you haven't switched to yet that's already signed in.

## [0.1.5] - 2026-11-06

### Added

- Install plugins from a `mygo-music://install-plugin?repo=owner/repo` link. It opens Settings > Plugins and asks before anything is installed.
- Settings > Plugins has a + menu to install from GitHub, a folder or a zip, or to show the plugins folder. Each plugin's ⋯ menu shows its folder or removes the plugin, which moves it to the Trash after asking.
- Songs with no cover, or whose cover failed to load, show a note cover in Now Playing.

### Changed

- Installing from GitHub takes any GitHub page address or `git@` address, not just the repository's root.
- A plugin's update error shows next to that plugin.
- Primary buttons and settings use the app's accent colour.

## [0.1.4] - 2026-11-06

### Added

- The next song is fetched ahead of time, so it starts right away when its turn comes.
- You can seek in a song that's still downloading. Seeking past what has arrived shows that the song is loading instead of going quiet.
- Plugins can sign in on a web page the app shows (`host.webSignIn`), for services whose website is the only way in.
- Install plugins from a folder or a zip on this computer.

### Fixed

- A sign-in that fails straight away no longer makes the dialog flash and jump.
- The daily card's section keeps a gap above the cards below it.

## [0.1.3] - 2026-11-06

### Fixed

- The daily card's grooves ripple more slowly, so the GPU stays mostly idle on Home while its songs play.

## [0.1.2] - 2026-11-06

### Added

- Now Playing grows out of the player bar's cover, and you can pull it down to close it. With reduced motion turned on, it fades in place.
- The search field has a clear button, and its recent searches popup matches the macOS search menu.

### Fixed

- The window no longer shakes while it's dragged.
- Clicks anywhere on the search field reach it, including the icon and the edges.
- Now Playing's bars fade as it closes instead of showing over the player bar.
- The wall's cover stays in place as it flies out of the player bar.
- The sidebar's dropdowns open and close at once.

### Changed

- The hairline between the sidebar and the page is gone.

## [0.1.1] - 2026-11-06

### Added

- A website with English and Chinese download pages.

## [0.1.0] - 2026-11-06

The first release: a native music player that plays your local files and gets everything else from plugins.

[0.1.11]: https://github.com/egoist/mygo-music/compare/v0.1.10...v0.1.11
[0.1.10]: https://github.com/egoist/mygo-music/compare/v0.1.9...v0.1.10
[0.1.9]: https://github.com/egoist/mygo-music/compare/v0.1.8...v0.1.9
[0.1.8]: https://github.com/egoist/mygo-music/compare/v0.1.7...v0.1.8
[0.1.7]: https://github.com/egoist/mygo-music/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/egoist/mygo-music/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/egoist/mygo-music/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/egoist/mygo-music/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/egoist/mygo-music/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/egoist/mygo-music/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/egoist/mygo-music/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/egoist/mygo-music/releases/tag/v0.1.0
