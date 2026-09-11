import { LanguageSwitcher } from '@/components/language-switcher'
import { NotificationPopover } from '@/components/notification-popover'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { useNotifications } from '@/hooks/use-notifications'

import { Header } from './header'
import { SystemBrand } from './system-brand'

export function AppHeader() {
  const notifications = useNotifications()

  return (
    <Header>
      <SystemBrand variant='inline' />
      <div className='ms-auto flex items-center gap-1 sm:gap-2'>
        <Search />
        <NotificationPopover
          open={notifications.popoverOpen}
          onOpenChange={notifications.setPopoverOpen}
          unreadCount={notifications.unreadCount}
          activeTab={notifications.activeTab}
          onTabChange={notifications.setActiveTab}
          notice={notifications.notice}
          announcements={notifications.announcements}
          loading={notifications.loading}
        />
        <LanguageSwitcher />
        <ProfileDropdown />
      </div>
    </Header>
  )
}
