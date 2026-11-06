// The plugins listed on the plugins page: public repositories on GitHub with
// the topic, as the README asks plugins to have. Kept free of the page so
// vite.config.ts can use it as the site is built.

export const pluginTopic = 'mygo-music-plugin'
export const pluginQuery = `topic:${pluginTopic} archived:false`

export type Plugin = {
  repo: string
  name: string
  owner: string
  avatar: string
  description: string
  url: string
  stars: number
  pushedAt: string
}

// The plugins of an answer from GitHub's repository search.
export function pluginsFrom(answer: { items: any[] }): Plugin[] {
  return answer.items.map((r) => ({
    repo: r.full_name,
    name: r.name,
    owner: r.owner.login,
    avatar: r.owner.avatar_url,
    description: r.description ?? '',
    url: r.html_url,
    stars: r.stargazers_count,
    pushedAt: r.pushed_at,
  }))
}

// The plugins as they are now, from the browser. Unsigned requests to the
// search have a low rate limit, so this can fail; the page then keeps the
// list it was built with.
export async function fetchPlugins(): Promise<Plugin[]> {
  const res = await fetch(
    `https://api.github.com/search/repositories?q=${encodeURIComponent(pluginQuery)}&sort=stars&per_page=100`,
  )
  if (!res.ok) throw new Error(`GitHub answered ${res.status}`)
  return pluginsFrom(await res.json())
}

// A plugin's name without the app's in front, as repositories tend to be
// named: mygo-music-youtube-music is "youtube-music".
export function shortName(name: string) {
  return name.replace(/^mygo-music-(plugin-)?/i, '') || name
}

// The link that opens the app to install a plugin, as the README describes.
export const installLink = (repo: string) => `mygo-music://install-plugin?repo=${repo}`
