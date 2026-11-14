// The configuration of the app, which `mygo dev` and `mygo build` evaluate with
// Bun, of the command running. The development app (`mygo dev`) opens links of a
// scheme of its own, for the installed app to keep mygo-music:// links.
export default ({ command }: { command: string }) => ({
  name: "MyGo Music",
  identifier: "dev.egoist.music",
  slug: "mygo-music",
  version: "0.1.22",
  out: "build",
  urlSchemes: [command === "dev" ? "mygo-music-dev" : "mygo-music"],
  // The fork's own updates: its releases, signed with its key (mygo keygen),
  // kept as MYGO_UPDATER_PRIVATE_KEY in the repository's secrets.
  updates: {
    publicKey: "qZgzLRaE73nUZ73fx6s+3V64KmhFnXCCiTjlWmhlC/I=",
    github: "1CoreBrain/mygo-music",
  },
  macos: {
    icon: "icon/AppIcon.icon",
    minimumSystemVersion: "13.0",
    signingIdentity: "Developer ID Application: xinyu zeng (GJE9R5VE87)",
    notarize: { keychainProfile: "notary" },
  },
  linux: {
    comment: "A music player for your files and more",
    categories: ["AudioVideo", "Audio", "Player"],
    webview: false,
  },
});
