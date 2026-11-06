import { Link } from '@tanstack/solid-router'
import { createSignal, onMount } from 'solid-js'
import { Layout } from './Layout'
import { Icon } from './Icon'

// The page of a path with none. Cloudflare serves the one 404.html for every
// such path, in English, so the language is picked once it runs: the one of
// the path, else the one chosen, else the browser's.
export function NotFound() {
  const [zh, setZh] = createSignal(false)
  onMount(() => {
    const p = location.pathname
    let chosen: string | null = null
    try {
      chosen = localStorage.getItem('lang')
    } catch {}
    setZh(p === '/zh' || p.startsWith('/zh/') || chosen === 'zh' || (!chosen && /^zh\b/i.test(navigator.language)))
  })
  const t = (en: string, cn: string) => (zh() ? cn : en)
  return (
    <Layout langPath="/">
      <section class="lost">
        <div class="lost-record" aria-hidden="true">
          <div class="vinyl" />
        </div>
        <p class="eyebrow">404</p>
        <h1 class="display">{t('This track is missing', '这首歌找不到了')}</h1>
        <p class="lede">
          {t(
            'Maybe it was moved, or the link has a typo. Head back home and put on something else.',
            '也许它被移走了，也许链接写错了。回到首页，换一首听听。',
          )}
        </p>
        <Link to={zh() ? ('/zh' as '/') : '/'} class="btn btn-primary">
          {t('Back Home', '回到首页')}
          <Icon name="arrow" size={16} />
        </Link>
      </section>
    </Layout>
  )
}
