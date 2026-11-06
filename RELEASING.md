# Releasing

Releases are built by [`.github/workflows/release.yml`](.github/workflows/release.yml). Installed apps update to a release once it's published.

## Cut a release

1. Make sure `main` is clean and the tests pass:

   ```bash
   go test ./internal/...
   ```

2. Add a section for the new version at the top of [`CHANGELOG.md`](CHANGELOG.md), in the same style as the ones below it, and add its compare link at the bottom. Leave out website and test-only changes.

3. Bump `version` in [`mygo.config.ts`](mygo.config.ts).

4. Commit with the bare version as the message, tag it, and push both:

   ```bash
   git commit -am "0.1.6"
   git tag v0.1.6
   git push origin main v0.1.6
   ```

   The tag must match the version in `mygo.config.ts`, or the workflow stops. You can also run the workflow by hand (`gh workflow run release.yml`) instead of pushing a tag. It then tags the version in `mygo.config.ts` itself.

5. Wait for the workflow (`gh run watch`). It creates a **draft** release, using the version's `CHANGELOG.md` section as the notes, and uploads the macOS, Windows and Linux builds, their installers, the update manifests and the deltas from earlier versions.

6. Check the draft's notes and assets, then publish it:

   ```bash
   gh release edit v0.1.6 --draft=false --latest
   ```

   Installed apps only pick up the update once it's published.

## Secrets

The workflow needs these repository secrets:

| Secret | Used for |
| --- | --- |
| `MACOS_CERTIFICATE`, `MACOS_CERTIFICATE_PASSWORD` | The Developer ID certificate (base64 `.p12`) that signs the macOS app |
| `APPLE_ID`, `APPLE_TEAM_ID`, `APPLE_APP_PASSWORD` | Notarizing the macOS disk images |
| `MYGO_UPDATER_PRIVATE_KEY` | Signing the updates; its public key is `updates.publicKey` in `mygo.config.ts` |

## Podcast caption libraries

The sherpa-onnx libraries are released separately, as the `sherpa-onnx-<version>` prerelease, and only change when that version changes. See the README.
