import { createFileRoute } from '@tanstack/solid-router'
import { Home } from '../pages/Home'
import { pageHead } from '../i18n'
import { site } from '../site'

export const Route = createFileRoute('/')({
  head: () => pageHead('/', 'en', site.title.en, site.description.en),
  component: Home,
})
