import { createFileRoute } from '@tanstack/solid-router'
import { Download } from '../../pages/Download'
import { pageHead } from '../../i18n'
import { site } from '../../site'

export const Route = createFileRoute('/zh/download')({
  head: () => pageHead('/download', 'zh', `下载 · ${site.name}`, site.description.zh),
  component: Download,
})
