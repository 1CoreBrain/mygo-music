import { createFileRoute } from '@tanstack/solid-router'
import { Download } from '../pages/Download'
import { pageHead } from '../i18n'
import { site } from '../site'

export const Route = createFileRoute('/download')({
  head: () => pageHead('/download', 'en', `Download · ${site.name}`, site.description.en),
  component: Download,
})
