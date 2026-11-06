# Website

The home page of MyGo Music, written with TanStack Start and Solid. It's prerendered into static files at build time and deployed to Cloudflare (Workers static assets, no Worker script).

```bash
bun install          # at the root of the repository
bun run dev          # http://localhost:3000
bun run build        # into website/dist/client
bun run deploy       # build, and deploy with wrangler
```

The download page links to the installers of the latest release, fetched from GitHub as the site is built, so deploy again after publishing a release. The build fails without a published release. Set `GH_TOKEN` or `GITHUB_TOKEN` for the API's rate limit, or to read the releases while the repository is private.
