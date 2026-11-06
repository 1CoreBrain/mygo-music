import { For, Show, createSignal, onMount } from 'solid-js'
import { Layout } from '../components/Layout'
import { GitHubMark, Icon } from '../components/Icon'
import { builtPlugins, repo } from '../site'
import { fetchPlugins, installLink, pluginTopic, shortName, type Plugin } from '../plugins'
import { LangLink, Tr, useT } from '../i18n'

function count(n: number) {
  return n >= 1000 ? `${(n / 1000).toFixed(n >= 10000 ? 0 : 1)}k` : `${n}`
}

export function Plugins() {
  const t = useT()
  // The page is rendered with the plugins there were as the site was built,
  // and asks GitHub for the ones published since once it runs.
  const [plugins, setPlugins] = createSignal<Plugin[]>(builtPlugins)
  onMount(() => {
    fetchPlugins()
      .then(setPlugins)
      .catch(() => {})
  })

  return (
    <Layout>
      <section class="download">
        <div class="section-head">
          <p class="eyebrow">{t('Plugins', '插件')}</p>
          <h1 class="display">
            <Tr
              en={
                <>
                  <span>More music,</span> <span>from plugins.</span>
                </>
              }
              zh={
                <>
                  <span>更多的音乐，</span>
                  <span>来自插件。</span>
                </>
              }
            />
          </h1>
          <p class="lede">
            {t(
              'Plugins published on GitHub by the people who make them. Install one here, or with the + in Settings > Plugins by its repository. A plugin asks before it reaches anything, and you can remove it from its ⋯ menu.',
              '由作者发布在 GitHub 上的插件。在这里安装，或在「设置 > 插件」中点 +，填入它的仓库。插件访问任何东西前都会先征得你的同意，也可以从它的 ⋯ 菜单中移除。',
            )}
          </p>
        </div>

        <Show
          when={plugins().length > 0}
          fallback={<p class="plugin-empty">{t('No plugins have been published yet.', '还没有人发布插件。')}</p>}
        >
          <div class="plugin-list">
            <For each={plugins()}>{(p) => <PluginCard plugin={p} />}</For>
          </div>
        </Show>

        <div class="install publish">
          <h2>{t('Publish your plugin', '发布你的插件')}</h2>
          <p>
            <Tr
              en={
                <>
                  Put it in a public GitHub repository and add the topic <code>{pluginTopic}</code> to it: it shows here
                  once GitHub has it. <a href={`${repo}#publishing-a-plugin`}>How to publish a plugin</a>
                </>
              }
              zh={
                <>
                  把它放进一个公开的 GitHub 仓库，并给仓库加上话题 <code>{pluginTopic}</code>，GitHub 收录后它就会出现在这里。
                  <a href={`${repo}#publishing-a-plugin`}>如何发布插件</a>
                </>
              }
            />
          </p>
        </div>

        <p class="fine download-more">
          <Icon name="download" size={14} />
          <LangLink to="/download">{t('Install MyGo Music first', '先安装 MyGo Music')}</LangLink>
        </p>
      </section>
    </Layout>
  )
}

function PluginCard(props: { plugin: Plugin }) {
  const t = useT()
  const p = () => props.plugin
  return (
    <article class="platform plugin-card">
      <div class="plugin-card-head">
        <img class="plugin-avatar" src={`${p().avatar}&s=80`} alt="" width="40" height="40" loading="lazy" />
        <div class="plugin-title">
          <h2>{shortName(p().name)}</h2>
          <a class="plugin-repo" href={p().url}>
            {p().repo}
          </a>
        </div>
        <span class="plugin-stars" aria-label={t(`${p().stars} stars`, `${p().stars} 个星标`)}>
          <svg width="13" height="13" viewBox="0 0 24 24" aria-hidden="true">
            <path
              d="m12 2.8 2.8 5.8 6.3.9-4.6 4.4 1.1 6.3L12 17.2l-5.6 3 1.1-6.3-4.6-4.4 6.3-.9z"
              fill="currentColor"
            />
          </svg>
          {count(p().stars)}
        </span>
      </div>
      <p class="platform-note">{p().description || t('No description.', '没有简介。')}</p>
      <div class="plugin-actions">
        <a
          class="build primary"
          href={installLink(p().repo)}
          aria-label={t(`Install ${shortName(p().name)}`, `安装 ${shortName(p().name)}`)}
        >
          <Icon name="download" size={16} />
          <span class="build-label">{t('Install', '安装')}</span>
        </a>
        <a class="build" href={p().url} aria-label={t(`${p().repo} on GitHub`, `GitHub 上的 ${p().repo}`)}>
          <GitHubMark size={15} />
          <span class="build-label">GitHub</span>
        </a>
      </div>
    </article>
  )
}
