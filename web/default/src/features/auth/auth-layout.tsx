/*
Copyright (C) 2026 codego-api contributors

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

*/
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSystemConfig } from '@/hooks/use-system-config'
import { Skeleton } from '@/components/ui/skeleton'

type AuthLayoutProps = {
  children: React.ReactNode
}

export function AuthLayout({ children }: AuthLayoutProps) {
  const { t } = useTranslation()
  const { systemName, logo, loading } = useSystemConfig()

  return (
    <div className='bg-background text-foreground relative min-h-svh'>
      <Link
        to='/'
        className='app-subtle-panel absolute top-4 left-4 z-10 flex items-center gap-3 px-3 py-2 text-sm transition-opacity hover:opacity-80 sm:top-8 sm:left-8'
      >
        <div className='relative h-8 w-8'>
          {loading ? (
            <Skeleton className='absolute inset-0 rounded-full' />
          ) : (
            <img
              src={logo}
              alt={t('Logo')}
              className='h-8 w-8 rounded-full object-cover'
            />
          )}
        </div>
        {loading ? (
          <Skeleton className='h-6 w-24' />
        ) : (
          <h1 className='text-xl font-medium'>{systemName}</h1>
        )}
      </Link>
      <div className='relative container flex min-h-svh items-center justify-center px-4 py-20 sm:px-6 lg:px-8'>
        <div className='w-full max-w-[1040px]'>
          <div className='app-page-shell grid overflow-hidden lg:grid-cols-[minmax(0,1.05fr)_minmax(420px,0.95fr)]'>
            <div className='border-border bg-muted/40 hidden flex-col justify-between border-r p-10 lg:flex'>
              <div className='space-y-5'>
                <div className='border-border text-muted-foreground inline-flex items-center rounded-full border px-3 py-1 text-[11px] font-semibold tracking-[0.14em] uppercase'>
                  {t('Developer workspace')}
                </div>
                <div className='space-y-3'>
                  <h2 className='text-foreground max-w-[12ch] text-4xl font-semibold tracking-tight'>
                    {t('Manage your models, quota, and billing in one place')}
                  </h2>
                  <p className='text-muted-foreground max-w-[44ch] text-sm leading-7'>
                    {t(
                      'Sign in to continue with API keys, subscriptions, usage tracking, and operational controls.'
                    )}
                  </p>
                </div>
              </div>

              <div className='grid gap-3'>
                <div className='app-subtle-panel p-5'>
                  <div className='text-foreground text-sm font-semibold'>
                    {t('Reliable access')}
                  </div>
                  <p className='text-muted-foreground mt-2 text-sm leading-6'>
                    {t(
                      'Keep quota, subscription status, and workspace controls within immediate reach during active development.'
                    )}
                  </p>
                </div>
              </div>
            </div>

            <div className='flex min-h-[640px] items-center'>
              <div className='mx-auto flex w-full max-w-[480px] flex-col justify-center px-5 py-8 sm:px-8 md:px-10'>
                {children}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
