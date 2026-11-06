import { createFileRoute } from '@tanstack/solid-router'
import { Plugins } from '../../pages/Plugins'
import { pageHead } from '../../i18n'
import { site } from '../../site'

export const Route = createFileRoute('/zh/plugins')({
  head: () => pageHead('/plugins', 'zh', `插件 · ${site.name}`, `发布在 GitHub 上的 ${site.name} 插件。`),
  component: Plugins,
})
