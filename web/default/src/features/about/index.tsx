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
import { Network, Route, ShieldCheck, SlidersHorizontal } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { PublicLayout } from '@/components/layout'

const capabilities = [
  {
    icon: Network,
    title: 'Provider connections',
    description:
      'Keep credentials and upstream model endpoints in one operational console.',
  },
  {
    icon: Route,
    title: 'Request routing',
    description:
      'Map models, groups, and fallback behavior to a consistent API surface.',
  },
  {
    icon: SlidersHorizontal,
    title: 'Usage controls',
    description:
      'Manage keys, quotas, rate limits, pricing, and access policies together.',
  },
  {
    icon: ShieldCheck,
    title: 'Auditable operations',
    description:
      'Inspect request usage and account activity without exposing provider secrets.',
  },
]

export function About() {
  return (
    <PublicLayout>
      <main className='mx-auto w-full max-w-6xl space-y-8 px-4 py-10 sm:py-14'>
        <header className='max-w-3xl space-y-4'>
          <div className='text-primary text-xs font-semibold tracking-[0.24em] uppercase'>
            API unified management
          </div>
          <h1 className='text-foreground text-4xl font-semibold tracking-tight md:text-5xl'>
            One control plane for every API connection.
          </h1>
          <p className='text-muted-foreground text-base leading-8 md:text-lg'>
            codego-api is an open-source management platform for connecting
            providers, routing model requests, issuing API keys, and tracking
            usage from one place.
          </p>
        </header>

        <section className='grid gap-4 sm:grid-cols-2'>
          {capabilities.map((item) => (
            <Card key={item.title} className='shadow-none'>
              <CardHeader>
                <item.icon className='text-primary size-5' />
                <CardTitle className='text-lg'>{item.title}</CardTitle>
              </CardHeader>
              <CardContent className='text-muted-foreground text-sm leading-7'>
                {item.description}
              </CardContent>
            </Card>
          ))}
        </section>
      </main>
    </PublicLayout>
  )
}
