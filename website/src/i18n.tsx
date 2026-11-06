import { Link, useLocation } from '@tanstack/solid-router'
import { For, createSignal, onCleanup, onMount, type JSX } from 'solid-js'
import { Icon } from './components/Icon'

// The site is in English at / and in Chinese under /zh/, each page
// prerendered in both. The language of a page is the one of its path.
export type Lang = 'en' | 'zh'

const key = 'lang'

const isZh = (path: string) => path === '/zh' || path.startsWith('/zh/')

// Runs in <head> before the page paints. A visitor who chose a language
// with the switch gets it; one who hasn't, and reads Chinese, is sent from
// an English page to its Chinese one. Chinese pages aren't sent back to
// English by the browser's language, so that a link to one, or a crawler,
// gets the page it asked for.
export const langScript = `try{var p=location.pathname,z=p==='/zh'||p.indexOf('/zh/')===0,l=localStorage.getItem('${key}');if(l!=='en'&&l!=='zh')l=null;var w=l||(/^zh\\b/i.test(navigator.language)?'zh':'en');if(w==='zh'&&!z)location.replace('/zh'+(p==='/'?'':p)+location.search+location.hash);else if(l==='en'&&z)location.replace((p.slice(3)||'/')+location.search+location.hash)}catch(e){}`

// The page's path in a language, from its path in either.
export function pathIn(lang: Lang, path: string) {
  const base = isZh(path) ? path.slice(3) || '/' : path
  return lang === 'zh' ? (base === '/' ? '/zh' : `/zh${base}`) : base
}

export function useLang() {
  const location = useLocation()
  return (): Lang => (isZh(location().pathname) ? 'zh' : 'en')
}

// t picks the words of the page's language: t('Download', '下载').
export function useT() {
  const lang = useLang()
  return <T,>(en: T, zh: T): T => (lang() === 'zh' ? zh : en)
}

// Tr shows words with markup in the page's language. Only that one is
// made: Solid reads a prop when it's used, and hydrating the page looks for
// the nodes of whatever is made, so making both, as t(<>…</>, <>…</>)
// would, fails on the one that isn't there.
export function Tr(props: { en: JSX.Element; zh: JSX.Element }) {
  const lang = useLang()
  return <>{lang() === 'zh' ? props.zh : props.en}</>
}

// A link to a page of the site, in the language of this one.
export function LangLink(props: { to: string; class?: string; children: JSX.Element; 'aria-label'?: string }) {
  const lang = useLang()
  return (
    <Link to={pathIn(lang(), props.to) as '/'} class={props.class} aria-label={props['aria-label']}>
      {props.children}
    </Link>
  )
}

const languages: { lang: Lang; name: string; hreflang: string }[] = [
  { lang: 'en', name: 'English', hreflang: 'en' },
  { lang: 'zh', name: '简体中文', hreflang: 'zh-CN' },
]

// The language menu of the header: a button naming the page's language,
// opening a list of both, each a link to this page in it. Picking one
// remembers it, so the head script doesn't send the visitor back. The
// links are there while the menu is closed too, hidden, for the prerender
// to find the pages in Chinese by. A page that isn't one of the site's,
// like the 404, gives `path` to link to instead.
export function LangMenu(props: { path?: string }) {
  const lang = useLang()
  const t = useT()
  const location = useLocation()
  const [open, setOpen] = createSignal(false)
  let root!: HTMLDivElement
  let button!: HTMLButtonElement
  const items = () => [...root.querySelectorAll<HTMLAnchorElement>('[role=menuitemradio]')]

  const close = (refocus = false) => {
    setOpen(false)
    if (refocus) button.focus()
  }
  const show = (focus: 'current' | 'first' | 'last' = 'current') => {
    setOpen(true)
    const all = items()
    const i = focus === 'first' ? 0 : focus === 'last' ? all.length - 1 : languages.findIndex((l) => l.lang === lang())
    // Focus once the menu is visible; a hidden link takes no focus.
    requestAnimationFrame(() => all[Math.max(0, i)]?.focus())
  }

  onMount(() => {
    const away = (e: PointerEvent) => open() && !root.contains(e.target as Node) && close()
    document.addEventListener('pointerdown', away)
    onCleanup(() => document.removeEventListener('pointerdown', away))
  })

  const onMenuKey = (e: KeyboardEvent) => {
    const all = items()
    const i = all.indexOf(document.activeElement as HTMLAnchorElement)
    const move = (to: number) => {
      e.preventDefault()
      all[(to + all.length) % all.length]?.focus()
    }
    if (e.key === 'ArrowDown') move(i + 1)
    else if (e.key === 'ArrowUp') move(i - 1)
    else if (e.key === 'Home') move(0)
    else if (e.key === 'End') move(all.length - 1)
    else if (e.key === 'Escape') {
      e.preventDefault()
      close(true)
    } else if (e.key === 'Tab') close()
  }

  return (
    <div class="lang" classList={{ open: open() }} ref={root}>
      <button
        ref={button}
        type="button"
        class="lang-button"
        aria-haspopup="menu"
        aria-expanded={open()}
        aria-label={t('Language: English', '语言：简体中文')}
        onClick={() => (open() ? close() : show())}
        onKeyDown={(e) => {
          if (e.key === 'ArrowDown') {
            e.preventDefault()
            show('first')
          } else if (e.key === 'ArrowUp') {
            e.preventDefault()
            show('last')
          }
        }}
      >
        <Icon name="globe" size={15} />
        <span class="lang-current">{t('English', '中文')}</span>
        <Icon name="down" size={12} stroke={2.2} class="lang-chevron" />
      </button>
      <div class="lang-menu" role="menu" aria-label={t('Language', '语言')} onKeyDown={onMenuKey}>
        <For each={languages}>
          {(l) => (
            <Link
              to={pathIn(l.lang, props.path ?? location().pathname) as '/'}
              role="menuitemradio"
              aria-checked={l.lang === lang()}
              tabindex={open() ? 0 : -1}
              hreflang={l.hreflang}
              lang={l.hreflang}
              class={l.lang === lang() ? 'lang-item on' : 'lang-item'}
              onClick={() => {
                try {
                  localStorage.setItem(key, l.lang)
                } catch {}
                close()
              }}
            >
              <span>{l.name}</span>
              <svg class="lang-check" width="14" height="14" viewBox="0 0 24 24" aria-hidden="true">
                <path d="M5 12.5 10 17.5 19 7" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
            </Link>
          )}
        </For>
      </div>
    </div>
  )
}

// The head of a page in both languages: its title and description, and
// where the page is in each.
export function pageHead(path: string, lang: Lang, title: string, description: string) {
  return {
    meta: [
      { title },
      { name: 'description', content: description },
      { property: 'og:title', content: title },
      { property: 'og:description', content: description },
      { property: 'og:locale', content: lang === 'zh' ? 'zh_CN' : 'en_US' },
    ],
    links: [
      { rel: 'alternate', hreflang: 'en', href: pathIn('en', path) },
      { rel: 'alternate', hreflang: 'zh-CN', href: pathIn('zh', path) },
      { rel: 'alternate', hreflang: 'x-default', href: pathIn('en', path) },
    ],
  }
}
