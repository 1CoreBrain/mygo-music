import { For } from 'solid-js'
import { Icon } from './Icon'
import { liked, setLiked } from '../player'
import { useT } from '../i18n'

// The heart beside the song; liking it bursts, as in the app.
export function Like() {
  const t = useT()
  return (
    <button
      type="button"
      class="like"
      classList={{ on: liked() }}
      aria-pressed={liked()}
      aria-label={liked() ? t('Remove from Liked Songs', '取消喜欢') : t('Add to Liked Songs', '喜欢')}
      onClick={() => setLiked(!liked())}
    >
      <Icon name="heart" size={16} />
      <span class="burst" aria-hidden="true">
        <For each={[0, 1, 2, 3, 4, 5]}>{(i) => <i style={{ '--a': `${i * 60}deg` }} />}</For>
      </span>
    </button>
  )
}
