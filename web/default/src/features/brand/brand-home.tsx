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
import {
  Activity,
  ArrowUpRight,
  BarChart3,
  KeyRound,
  Network,
  Route,
  Settings2,
  ShieldCheck,
} from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { Button } from '@/components/ui/button'
import { PublicLayout } from '@/components/layout'
import { Footer } from '@/components/layout/components/footer'

const capabilities = [
  {
    icon: Network,
    title: 'Provider registry',
    text: 'Connect OpenAI-compatible and native provider channels from one place.',
    href: '/channels',
  },
  {
    icon: Route,
    title: 'Routing policies',
    text: 'Organize models, groups, fallbacks, and traffic rules for each API surface.',
    href: '/route-pools',
  },
  {
    icon: KeyRound,
    title: 'API key control',
    text: 'Issue keys with clear scopes, quotas, and lifecycle controls for every consumer.',
    href: '/keys',
  },
  {
    icon: BarChart3,
    title: 'Usage visibility',
    text: 'Review requests, cost signals, and provider performance in one operational view.',
    href: '/usage-logs',
  },
]

const workflow = [
  { label: 'Connect', value: 'Providers and model channels' },
  { label: 'Govern', value: 'Keys, quotas, and routing policies' },
  { label: 'Observe', value: 'Requests, usage, and health signals' },
]

export function BrandHome() {
  const isAuthenticated = Boolean(useAuthStore((state) => state.auth.user))

  return (
    <PublicLayout showMainContainer={false}>
      <main className='bg-background min-h-screen overflow-hidden'>
        <section className='border-border/70 relative border-b px-5 pt-24 pb-12 sm:px-8 sm:pt-28 lg:px-12'>
          <div className='mx-auto grid w-full max-w-7xl gap-12 lg:grid-cols-[minmax(0,1.08fr)_minmax(360px,0.92fr)] lg:items-center'>
            <div className='max-w-3xl space-y-7'>
              <div className='text-primary inline-flex items-center gap-2 text-xs font-semibold tracking-[0.22em] uppercase'>
                <Settings2 className='size-4' />
                API unified management platform
              </div>
              <div className='space-y-5'>
                <h1 className='text-foreground text-4xl leading-[1.04] font-semibold tracking-tight sm:text-6xl'>
                  codego-api
                  <br />
                  <span className='text-muted-foreground'>
                    one control plane for every API.
                  </span>
                </h1>
                <p className='text-muted-foreground max-w-2xl text-base leading-8 sm:text-lg'>
                  Connect providers, expose a consistent API, manage keys and
                  quotas, and understand usage without maintaining separate
                  control panels.
                </p>
              </div>
              <div className='flex flex-wrap gap-3'>
                <Button
                  render={
                    <Link to={isAuthenticated ? '/dashboard' : '/sign-up'} />
                  }
                >
                  {isAuthenticated ? 'Open console' : 'Create account'}
                  <ArrowUpRight className='size-4' />
                </Button>
                <Button variant='outline' render={<Link to='/about' />}>
                  Explore the platform
                </Button>
              </div>
              <div className='text-muted-foreground flex flex-wrap gap-x-5 gap-y-2 text-xs'>
                <span className='inline-flex items-center gap-1.5'>
                  <ShieldCheck className='text-success size-3.5' />
                  Open-source control plane
                </span>
                <span className='inline-flex items-center gap-1.5'>
                  <Activity className='text-primary size-3.5' />
                  Provider-neutral routing
                </span>
              </div>
            </div>

            <div className='border-border bg-card overflow-hidden rounded-xl border shadow-sm'>
              <div className='border-border flex items-center justify-between border-b px-4 py-3'>
                <div className='flex items-center gap-2 text-sm font-semibold'>
                  <Activity className='text-primary size-4' />
                  Control plane
                </div>
                <span className='text-success text-xs font-medium'>
                  Operational
                </span>
              </div>
              <div className='space-y-3 p-4'>
                <div className='border-border bg-muted/30 rounded-lg border p-3'>
                  <div className='text-muted-foreground text-[11px] font-medium tracking-wide uppercase'>
                    Unified endpoint
                  </div>
                  <code className='text-foreground mt-2 block text-sm'>
                    https://your-host/v1
                  </code>
                </div>
                {workflow.map((item, index) => (
                  <div key={item.label} className='flex items-center gap-3'>
                    <span className='bg-primary/10 text-primary flex size-7 shrink-0 items-center justify-center rounded-md text-xs font-semibold'>
                      0{index + 1}
                    </span>
                    <div className='min-w-0'>
                      <div className='text-foreground text-sm font-medium'>
                        {item.label}
                      </div>
                      <div className='text-muted-foreground truncate text-xs'>
                        {item.value}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
              <div className='border-border bg-muted/20 border-t px-4 py-3'>
                <div className='text-muted-foreground flex items-center gap-2 text-xs'>
                  <BarChart3 className='size-3.5' />
                  Centralized usage and audit signals
                </div>
              </div>
            </div>
          </div>
        </section>

        <section className='px-5 py-14 sm:px-8 lg:px-12'>
          <div className='mx-auto w-full max-w-7xl'>
            <div className='flex flex-col justify-between gap-4 border-b pb-5 sm:flex-row sm:items-end'>
              <div>
                <p className='text-primary text-xs font-semibold tracking-[0.22em] uppercase'>
                  Platform surface
                </p>
                <h2 className='text-foreground mt-2 text-2xl font-semibold tracking-tight sm:text-3xl'>
                  Built for repeated API operations
                </h2>
              </div>
              <p className='text-muted-foreground max-w-md text-sm leading-6'>
                Keep the daily path compact: configure once, route deliberately,
                and inspect the result.
              </p>
            </div>
            <div className='mt-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
              {capabilities.map((item) => (
                <Link
                  key={item.title}
                  to={item.href}
                  className='border-border bg-card hover:border-primary/50 group rounded-lg border p-4 transition-colors'
                >
                  <item.icon className='text-primary size-5' />
                  <h3 className='text-foreground mt-4 text-sm font-semibold'>
                    {item.title}
                  </h3>
                  <p className='text-muted-foreground mt-2 text-sm leading-6'>
                    {item.text}
                  </p>
                  <span className='text-primary mt-4 inline-flex items-center gap-1 text-xs font-medium'>
                    Open module
                    <ArrowUpRight className='size-3.5 transition-transform group-hover:translate-x-0.5' />
                  </span>
                </Link>
              ))}
            </div>
          </div>
        </section>
      </main>
      <Footer />
    </PublicLayout>
  )
}
