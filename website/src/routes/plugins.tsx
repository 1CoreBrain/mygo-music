import { createFileRoute } from '@tanstack/solid-router'
import { Plugins } from '../pages/Plugins'
import { pageHead } from '../i18n'
import { site } from '../site'

export const Route = createFileRoute('/plugins')({
  head: () =>
    pageHead('/plugins', 'en', `Plugins · ${site.name}`, `Plugins for ${site.name}, published on GitHub.`),
  component: Plugins,
})
