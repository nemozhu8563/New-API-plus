/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { LanguageSwitcher } from '@/components/language-switcher'
import { Skeleton } from '@/components/ui/skeleton'
import { useSystemConfig } from '@/hooks/use-system-config'

type AuthLayoutProps = {
  children: React.ReactNode
}

export function AuthLayout(props: AuthLayoutProps) {
  const { t } = useTranslation()
  const { systemName, logo, loading } = useSystemConfig()

  const brand = (
    <Link
      to='/sign-in'
      className='focus-visible:ring-ring inline-flex items-center gap-2.5 rounded-lg transition-opacity outline-none hover:opacity-80 focus-visible:ring-2'
    >
      <div className='relative size-9 overflow-hidden rounded-xl'>
        {loading ? (
          <Skeleton className='absolute inset-0' />
        ) : (
          <img src={logo} alt={t('Logo')} className='size-full object-cover' />
        )}
      </div>
      {loading ? (
        <Skeleton className='h-6 w-28' />
      ) : (
        <span className='max-w-56 truncate text-base font-semibold tracking-tight'>
          {systemName}
        </span>
      )}
    </Link>
  )

  return (
    <div className='bg-background relative grid min-h-svh overflow-hidden lg:grid-cols-[minmax(0,0.9fr)_minmax(32rem,1.1fr)]'>
      <aside className='bg-muted/35 relative hidden overflow-hidden border-r lg:flex lg:flex-col lg:p-10 xl:p-14'>
        <div
          aria-hidden='true'
          className='bg-primary/12 absolute -top-32 -left-24 size-96 rounded-full blur-3xl'
        />
        <div
          aria-hidden='true'
          className='bg-primary/8 absolute right-0 bottom-0 size-80 translate-x-1/3 translate-y-1/3 rounded-full blur-3xl'
        />
        <div className='relative z-10'>{brand}</div>
        <div className='relative z-10 my-auto max-w-xl py-16'>
          <p className='text-primary mb-5 text-xs font-semibold tracking-[0.18em] uppercase'>
            {t('Console')}
          </p>
          {loading ? (
            <div className='space-y-4'>
              <Skeleton className='h-14 w-4/5' />
              <Skeleton className='h-14 w-3/5' />
            </div>
          ) : (
            <h1 className='text-foreground max-w-lg text-5xl leading-[1.08] font-semibold tracking-[-0.04em] text-pretty xl:text-6xl'>
              {systemName}
            </h1>
          )}
        </div>
      </aside>

      <main className='relative flex min-h-svh flex-col'>
        <header className='flex h-20 items-center justify-between px-5 sm:px-8 lg:justify-end'>
          <div className='lg:hidden'>{brand}</div>
          <LanguageSwitcher />
        </header>
        <div className='flex flex-1 items-center justify-center px-5 pt-2 pb-16 sm:px-8'>
          <div className='bg-card/80 border-border/70 w-full max-w-[30rem] rounded-2xl border p-6 shadow-[0_24px_80px_-48px_rgba(40,28,20,0.45)] backdrop-blur-sm sm:p-9'>
            {props.children}
          </div>
        </div>
      </main>
    </div>
  )
}
