import { createSignal, onCleanup, onMount, type JSX } from 'solid-js'
import { AppIcon } from './AppIcon'
import { GitHubMark, Icon } from './Icon'
import { ThemeSwitch } from '../theme'
import { mygo, repo, site } from '../site'
import { LangLink, LangMenu, useT } from '../i18n'

// langPath is where the language menu links to, for a page that isn't one
// of the site's own, like the 404.
export function Layout(props: { children: JSX.Element; langPath?: string }) {
  return (
    <>
      <Header langPath={props.langPath} />
      <main>{props.children}</main>
      <Footer />
    </>
  )
}

function Header(props: { langPath?: string }) {
  const t = useT()
  // Hairline and frosted backing only once the page has scrolled under it.
  const [scrolled, setScrolled] = createSignal(false)
  onMount(() => {
    const on = () => setScrolled(window.scrollY > 8)
    on()
    window.addEventListener('scroll', on, { passive: true })
    onCleanup(() => window.removeEventListener('scroll', on))
  })
  return (
    <header class="nav" classList={{ scrolled: scrolled() }}>
      <div class="nav-inner">
        <LangLink to="/" class="brand" aria-label={site.name}>
          <AppIcon size={28} />
          <span>{site.name}</span>
        </LangLink>
        <div class="nav-end">
          <LangLink to="/plugins" class="nav-link" aria-label={t('Plugins', '插件')}>
            <Icon name="plugin" size={16} />
            <span>{t('Plugins', '插件')}</span>
          </LangLink>
          <LangMenu path={props.langPath} />
          <ThemeSwitch class="nav-theme" />
          <a class="icon-link" href={repo} aria-label={t('Source code on GitHub', 'GitHub 上的源代码')}>
            <GitHubMark size={19} />
          </a>
        </div>
      </div>
    </header>
  )
}

function Footer() {
  const t = useT()
  return (
    <footer class="footer">
      <div class="footer-inner">
        <span>
          {site.name} · {t('Open source under', '以')} <a href={`${repo}/blob/main/LICENSE`}>AGPL-3.0</a>
          {t(' · Made with ', ' 开源 · 用 ')}
          <a href={mygo}>MyGo</a>
          {t('', ' 写成')}
        </span>
        <a class="icon-link" href={repo} aria-label={t('Source code on GitHub', 'GitHub 上的源代码')}>
          <GitHubMark size={17} />
        </a>
      </div>
    </footer>
  )
}
