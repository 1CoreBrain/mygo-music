import { For, Show, createSignal, onMount } from 'solid-js'
import { Layout } from '../components/Layout'
import { GitHubMark, Icon } from '../components/Icon'
import { release, repo } from '../site'
import { Tr, useLang, useT } from '../i18n'

type OS = 'mac' | 'windows' | 'linux'

// A download of a platform: the asset of the latest release it is, found by
// its name, as `mygo build` names installers.
type Build = { label: [string, string]; kind: [string, string]; match: (name: string) => boolean; primary?: boolean }

const has = (...parts: string[]) => (name: string) => {
  const n = name.toLowerCase()
  return parts.every((p) => n.includes(p))
}

const platforms: { os: OS; name: string; note: [string, string]; builds: Build[] }[] = [
  {
    os: 'mac',
    name: 'macOS',
    note: ['macOS 13 or later, signed and notarized by Apple', 'macOS 13 或更新，已签名并经过 Apple 公证'],
    builds: [
      { label: ['Apple Silicon', 'Apple 芯片'], kind: ['.dmg', '.dmg'], match: has('arm64', '.dmg'), primary: true },
      { label: ['Intel', 'Intel'], kind: ['.dmg', '.dmg'], match: has('amd64', '.dmg') },
    ],
  },
  {
    os: 'windows',
    name: 'Windows',
    note: ['Installs for the current user, without asking for an administrator', '安装到当前用户，无需管理员权限'],
    builds: [
      { label: ['x64', 'x64'], kind: ['Installer', '安装程序'], match: has('setup', 'amd64', '.exe'), primary: true },
      { label: ['ARM64', 'ARM64'], kind: ['Installer', '安装程序'], match: has('setup', 'arm64', '.exe') },
    ],
  },
  {
    os: 'linux',
    name: 'Linux',
    note: [
      '.deb for Debian and Ubuntu; for other distributions, the archive or the command below',
      'Debian 与 Ubuntu 用 .deb；其他发行版用压缩包或下面的命令',
    ],
    builds: [
      { label: ['x64', 'x64'], kind: ['.deb', '.deb'], match: has('amd64', '.deb'), primary: true },
      { label: ['ARM64', 'ARM64'], kind: ['.deb', '.deb'], match: has('arm64', '.deb') },
      { label: ['x64', 'x64'], kind: ['.tar.gz', '.tar.gz'], match: has('linux', 'amd64', '.tar.gz') },
      { label: ['ARM64', 'ARM64'], kind: ['.tar.gz', '.tar.gz'], match: has('linux', 'arm64', '.tar.gz') },
    ],
  },
]

const installSh = `curl -fsSL ${repo}/releases/latest/download/install.sh | sh`
const installPs = `irm ${repo}/releases/latest/download/install.ps1 | iex`

// A build's asset; a build the release has none of links to the release.
const asset = (b: Build) => release.assets.find((a) => b.match(a.name))

function detectOS(): OS | undefined {
  const ua = navigator.userAgent
  if (/iPhone|iPad|Android/.test(ua)) return undefined
  if (/Mac/.test(ua)) return 'mac'
  if (/Windows/.test(ua)) return 'windows'
  if (/Linux|X11/.test(ua)) return 'linux'
}

function megabytes(bytes: number) {
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function day(iso: string, lang: 'en' | 'zh') {
  const d = new Date(iso)
  if (lang === 'zh') return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日`
  return d.toLocaleDateString('en-US', { year: 'numeric', month: 'long', day: 'numeric', timeZone: 'UTC' })
}

export function Download() {
  const t = useT()
  const lang = useLang()
  // The page is rendered at build time; the system of the visitor comes
  // once it runs in the browser.
  const [os, setOS] = createSignal<OS>()
  onMount(() => setOS(detectOS()))

  return (
    <Layout>
      <section class="download">
        <div class="section-head download-head">
          <p class="eyebrow">{t('Download', '下载')}</p>
          <h1 class="display">
            <Tr
              en={
                <>
                  <span>Put your music</span> <span>on the desktop.</span>
                </>
              }
              zh={
                <>
                  <span>把音乐，</span>
                  <span>装进桌面。</span>
                </>
              }
            />
          </h1>
          <p class="fine download-version">
            {t(
              `Version ${release.tag_name.replace(/^v/, '')} · Released ${day(release.published_at, 'en')}`,
              `版本 ${release.tag_name.replace(/^v/, '')} · 发布于 ${day(release.published_at, 'zh')}`,
            )}
          </p>
        </div>

        <div class="platforms">
          <For each={platforms}>
            {(p) => (
              <article class="platform">
                <div class="platform-head">
                  <h2>{p.name}</h2>
                  <Show when={os() === p.os}>
                    <span class="pill">{t('Your system', '你的系统')}</span>
                  </Show>
                </div>
                <p class="platform-note">{t(...p.note)}</p>
                <div class="platform-builds">
                  <For each={p.builds}>
                    {(b) => (
                      <a
                        class="build"
                        classList={{ primary: b.primary }}
                        href={asset(b)?.browser_download_url ?? `${repo}/releases/tag/${release.tag_name}`}
                      >
                        <Icon name="download" size={16} />
                        <span class="build-label">{t(...b.label)}</span>
                        <span class="build-kind">{t(...b.kind)}</span>
                        <Show when={asset(b)}>{(a) => <span class="build-size">{megabytes(a().size)}</span>}</Show>
                      </a>
                    )}
                  </For>
                </div>
              </article>
            )}
          </For>
        </div>

        <div class="install">
          <h2>{t('Install with one command', '用一行命令安装')}</h2>
          <p>
            {t(
              'Downloads the latest version into your home folder, where it keeps itself up to date.',
              '下载最新版本，装到你的用户目录，以后同样自动更新。',
            )}
          </p>
          <Command label={t('macOS and Linux', 'macOS 与 Linux')} text={installSh} />
          <Command label="Windows PowerShell" text={installPs} />
        </div>

        <p class="fine download-more">
          <GitHubMark size={14} />
          <a href={`${repo}/releases`}>{t('All releases and what changed', '所有版本与更新说明')}</a>
        </p>
      </section>
    </Layout>
  )
}

function Command(props: { label: string; text: string }) {
  const t = useT()
  const [copied, setCopied] = createSignal(false)
  let timer: ReturnType<typeof setTimeout> | undefined
  const copy = () => {
    navigator.clipboard?.writeText(props.text).then(() => {
      setCopied(true)
      clearTimeout(timer)
      timer = setTimeout(() => setCopied(false), 1600)
    })
  }
  return (
    <div class="command">
      <span class="command-label">{props.label}</span>
      <div class="command-line">
        <code>{props.text}</code>
        <button type="button" class="command-copy" onClick={copy} aria-label={t(`Copy the install command for ${props.label}`, `拷贝${props.label}的安装命令`)}
        >
          {copied() ? t('Copied', '已拷贝') : t('Copy', '拷贝')}
        </button>
      </div>
    </div>
  )
}
