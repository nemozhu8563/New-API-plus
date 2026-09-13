import { createFileRoute } from '@tanstack/react-router'
import { ArrowUpRight, BookOpen, Code2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

export const Route = createFileRoute('/_authenticated/integration-guides')({
  component: IntegrationGuidesPage,
})

const guides = [
  {
    title: 'Claude API Integration Guide',
    description:
      'Learn how to connect Claude applications to this API gateway.',
    url: 'https://my.feishu.cn/docx/H21fdfviUoVU8GxBcrHcr4jqnOc',
    icon: BookOpen,
  },
  {
    title: 'Codex API Integration Guide',
    description: 'Learn how to configure Codex to use this API gateway.',
    url: 'https://my.feishu.cn/docx/G6OIdPeE9oFbX0xlwZPci8QRnig',
    icon: Code2,
  },
] as const

function IntegrationGuidesPage() {
  const { t } = useTranslation()

  return (
    <main className='mx-auto w-full max-w-5xl p-4 md:p-6'>
      <div className='mb-6 space-y-2'>
        <h1 className='text-2xl font-semibold tracking-tight'>
          {t('API Integration Guides')}
        </h1>
        <p className='text-muted-foreground'>
          {t('Choose a guide to learn how to connect your AI client.')}
        </p>
      </div>
      <div className='grid gap-4 md:grid-cols-2'>
        {guides.map((guide) => {
          const Icon = guide.icon
          return (
            <a
              key={guide.url}
              href={guide.url}
              target='_blank'
              rel='noreferrer'
              className='group focus-visible:ring-ring rounded-xl focus-visible:ring-2 focus-visible:outline-none'
            >
              <Card className='group-hover:bg-accent/50 h-full transition-colors'>
                <CardHeader>
                  <div className='bg-primary/10 text-primary mb-2 flex h-10 w-10 items-center justify-center rounded-lg'>
                    <Icon className='h-5 w-5' aria-hidden='true' />
                  </div>
                  <CardTitle className='flex items-center justify-between gap-2'>
                    <span>{t(guide.title)}</span>
                    <ArrowUpRight
                      className='text-muted-foreground h-4 w-4 shrink-0 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5'
                      aria-hidden='true'
                    />
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <CardDescription>{t(guide.description)}</CardDescription>
                </CardContent>
              </Card>
            </a>
          )
        })}
      </div>
    </main>
  )
}
