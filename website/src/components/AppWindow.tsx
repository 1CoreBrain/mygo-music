import { For, Show, createSignal, onMount } from 'solid-js'
import { Icon, Transport, type IconName } from './Icon'
import { Cover } from './Cover'
import { Like } from './Like'
import { clock, playing, position, seek, toggle, track } from '../player'
import { useT } from '../i18n'

const weekdays = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
const weekdaysZh = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六']
const months = [
  'January',
  'February',
  'March',
  'April',
  'May',
  'June',
  'July',
  'August',
  'September',
  'October',
  'November',
  'December',
]

// The greeting over Home, as the app words it by the hour.
function greeting(h: number): [string, string] {
  if (h < 5) return ['Good night', '夜深了']
  if (h < 12) return ['Good morning', '早上好']
  if (h < 18) return ['Good afternoon', '下午好']
  return ['Good evening', '晚上好']
}

// The sidebar of Local Files, the source that comes built in.
const nav: { icon: IconName; label: [string, string] }[] = [
  { icon: 'home', label: ['Home', '首页'] },
  { icon: 'music', label: ['Songs', '歌曲'] },
  { icon: 'album', label: ['Albums', '专辑'] },
  { icon: 'artist', label: ['Artists', '歌手'] },
  { icon: 'heart', label: ['Liked Songs', '我喜欢的音乐'] },
]

const playlists: [string, string][] = [
  ['Evening Walks', '晚间散步'],
  ['Focus', '专注'],
  ['Long Drives', '长途驾驶'],
  ['Rainy Days', '下雨天'],
]

// What played lately: the first is the page's song, the others only show.
const recent = [
  { title: track.title, artist: track.artist, album: track.album, hue: 18, duration: track.duration },
  { title: 'Northern Lights', artist: 'The Polar Lights', album: 'Polar Night', hue: 230, duration: 180 },
  { title: 'Slow Tide', artist: 'The Polar Lights', album: 'Polar Night', hue: 250, duration: 201 },
]

const albums = [
  { name: track.album, artist: track.artist, hue: 18 },
  { name: 'Polar Night', artist: 'The Polar Lights', hue: 230 },
  { name: 'Low Tide Hours', artist: 'Mira Sand', hue: 160 },
  { name: 'Small Rooms', artist: 'Odd Weather', hue: 300 },
  { name: 'Summer Static', artist: 'The Kites', hue: 45 },
]

// The app's main window, built in HTML: the sidebar of local files, Home
// with what played lately and the albums, and the player bar. It plays the
// page's song.
export function AppWindow() {
  const t = useT()
  // Rendered at build time, the date is fixed; the visitor's own fills in.
  const [now, setNow] = createSignal(new Date(2026, 9, 26, 21, 30))
  onMount(() => setNow(new Date()))
  const date = () =>
    t(
      `${weekdays[now().getDay()]}, ${months[now().getMonth()]} ${now().getDate()}`,
      `${now().getMonth() + 1}月${now().getDate()}日 ${weekdaysZh[now().getDay()]}`,
    )

  return (
    <div class="window" classList={{ playing: playing() }}>
      <aside class="win-side">
        <div class="win-strip">
          <div class="lights" aria-hidden="true">
            <i />
            <i />
            <i />
          </div>
          <span class="win-source">
            {t('Local Files', '本地文件')}
            <Icon name="down" size={12} stroke={2.2} />
          </span>
        </div>
        <div class="win-search">
          <Icon name="search" size={13} />
          {t('Search', '搜索')}
        </div>
        <For each={nav}>
          {(n, i) => (
            <div class="win-nav" classList={{ on: i() === 0 }}>
              <Icon name={n.icon} size={16} />
              {t(...n.label)}
            </div>
          )}
        </For>
        <div class="win-heading">{t('My Playlists', '我的歌单')}</div>
        <For each={playlists}>
          {(p, i) => (
            <div class="win-list">
              <Cover hue={(i() * 70 + 200) % 360} size={22} radius={5} />
              {t(...p)}
            </div>
          )}
        </For>
      </aside>

      <section class="win-main">
        <div class="win-page">
          <div class="win-greet">{t(...greeting(now().getHours()))}</div>
          <div class="win-date">{date()}</div>

          <div class="win-section">{t('Recently Played', '最近播放')}</div>
          <div class="win-rows">
            <For each={recent}>
              {(s, i) => (
                <div class="row" classList={{ current: i() === 0 }}>
                  <span class="row-n">
                    <Show when={i() === 0} fallback={i() + 1}>
                      <span class="bars" aria-hidden="true">
                        <i />
                        <i />
                        <i />
                      </span>
                    </Show>
                  </span>
                  <Cover hue={s.hue} size={36} radius={6} />
                  <span class="row-title">
                    <Show when={i() === 0} fallback={<b>{s.title}</b>}>
                      <button type="button" onClick={toggle} aria-label={playing() ? t('Pause', '暂停') : t(`Play ${s.title}`, `播放${s.title}`)}>
                        <b>{s.title}</b>
                      </button>
                    </Show>
                    <span>{s.artist}</span>
                  </span>
                  <span class="row-album">{s.album}</span>
                  <span class="row-time">{clock(s.duration)}</span>
                </div>
              )}
            </For>
          </div>

          <div class="win-section">{t('Albums', '专辑')}</div>
          <div class="win-albums">
            <For each={albums}>
              {(a) => (
                <div class="album">
                  <Cover hue={a.hue} size={120} radius={10} class="album-cover" />
                  <b>{a.name}</b>
                  <span>{a.artist}</span>
                </div>
              )}
            </For>
          </div>
        </div>

        <div class="win-bar">
          <div class="bar-song">
            <Cover hue={18} size={44} radius={8} />
            <div>
              <b>{track.title}</b>
              <span>{track.artist}</span>
            </div>
            <Like />
          </div>
          <div class="bar-center">
            <div class="bar-controls">
              <span class="ctl" aria-hidden="true">
                <Icon name="shuffle" size={14} />
              </span>
              <span class="ctl" aria-hidden="true">
                <Transport name="prev" size={16} />
              </span>
              <button type="button" class="ctl play" aria-label={playing() ? t('Pause', '暂停') : t('Play', '播放')} onClick={toggle}>
                <Transport name={playing() ? 'pause' : 'play'} size={20} />
              </button>
              <span class="ctl" aria-hidden="true">
                <Transport name="next" size={16} />
              </span>
              <span class="ctl" aria-hidden="true">
                <Icon name="repeat" size={14} />
              </span>
            </div>
            <div class="bar-progress">
              <span>{clock(position())}</span>
              <Progress />
              <span>{clock(track.duration)}</span>
            </div>
          </div>
          <div class="bar-end" aria-hidden="true">
            <Icon name="lyrics" size={16} />
            <Icon name="queue" size={16} />
            <Icon name="volume" size={16} />
          </div>
        </div>
      </section>
    </div>
  )
}

// The progress bar: click or drag to seek.
function Progress() {
  const t = useT()
  let el!: HTMLDivElement
  const at = (e: PointerEvent) => {
    const r = el.getBoundingClientRect()
    seek((e.clientX - r.left) / r.width)
  }
  return (
    <div
      ref={el}
      class="progress"
      role="slider"
      aria-label={t('Playback position', '播放进度')}
      aria-valuemin={0}
      aria-valuemax={Math.round(track.duration)}
      aria-valuenow={Math.round(position())}
      aria-valuetext={clock(position())}
      tabindex="0"
      onPointerDown={(e) => {
        el.setPointerCapture(e.pointerId)
        at(e)
      }}
      onPointerMove={(e) => el.hasPointerCapture(e.pointerId) && at(e)}
      onKeyDown={(e) => {
        const step = e.key === 'ArrowRight' ? 5 : e.key === 'ArrowLeft' ? -5 : 0
        if (step) {
          e.preventDefault()
          seek((position() + step) / track.duration)
        }
      }}
    >
      <div class="progress-fill" style={{ width: `${(position() / track.duration) * 100}%` }} />
    </div>
  )
}
