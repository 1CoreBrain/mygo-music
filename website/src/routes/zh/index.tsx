import { createFileRoute } from '@tanstack/solid-router'
import { Home } from '../../pages/Home'
import { pageHead } from '../../i18n'
import { site } from '../../site'

export const Route = createFileRoute('/zh/')({
  head: () => pageHead('/', 'zh', site.title.zh, site.description.zh),
  component: Home,
})
