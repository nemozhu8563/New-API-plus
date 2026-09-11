import { ChevronRight } from 'lucide-react'
import { memo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ScrollArea } from '@/components/ui/scroll-area'
import { useAnnouncements } from '@/features/dashboard/hooks/use-status-data'
import { getPreviewText } from '@/features/dashboard/lib'
import type { AnnouncementItem } from '@/features/dashboard/types'
import { getAnnouncementColorClass } from '@/lib/colors'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'

import { PanelWrapper } from '../ui/panel-wrapper'
import { AnnouncementDetailModal } from './announcement-detail-dialog'

const AnnouncementStatusDot = memo(function AnnouncementStatusDot(props: {
  type?: string
}) {
  return (
    <span
      className={cn(
        'inline-block size-2 shrink-0 rounded-full',
        getAnnouncementColorClass(props.type)
      )}
    />
  )
})

export function AnnouncementsPanel() {
  const { t } = useTranslation()
  const { items: list, loading } = useAnnouncements()
  const [selectedAnnouncement, setSelectedAnnouncement] =
    useState<AnnouncementItem | null>(null)
  const [isDialogOpen, setIsDialogOpen] = useState(false)

  const handleAnnouncementClick = (item: AnnouncementItem) => {
    setSelectedAnnouncement(item)
    setIsDialogOpen(true)
  }

  return (
    <PanelWrapper
      title={t('System Announcements')}
      loading={loading}
      empty={!list.length}
      emptyMessage={t('No announcements at this time')}
      height='h-[20rem]'
      className='h-full min-w-0'
      contentClassName='h-[20rem] p-0'
    >
      <ScrollArea className='h-[20rem]'>
        <div>
          {list.map((item: AnnouncementItem, idx: number) => {
            const key = item.id ?? `announcement-${idx}`
            return (
              <button
                key={key}
                type='button'
                onClick={() => handleAnnouncementClick(item)}
                className={cn(
                  'group hover:bg-muted/40 focus-visible:ring-ring flex w-full items-center gap-3 px-4 py-4 text-left outline-none transition-colors focus-visible:ring-2 focus-visible:ring-inset sm:px-5',
                  idx < list.length - 1 && 'border-border/60 border-b'
                )}
              >
                <AnnouncementStatusDot type={item.type} />
                <p className='min-w-0 flex-1 truncate text-sm font-medium'>
                  {getPreviewText(item.content)}
                </p>
                {item.publishDate && (
                  <time className='text-muted-foreground shrink-0 font-mono text-xs tabular-nums'>
                    {dayjs(item.publishDate).format('MM/DD')}
                  </time>
                )}
                <ChevronRight
                  aria-hidden='true'
                  className='text-muted-foreground size-4 shrink-0 transition-transform group-hover:translate-x-0.5'
                />
              </button>
            )
          })}
        </div>
      </ScrollArea>

      <AnnouncementDetailModal
        open={isDialogOpen}
        onOpenChange={setIsDialogOpen}
        announcement={selectedAnnouncement}
      />
    </PanelWrapper>
  )
}
