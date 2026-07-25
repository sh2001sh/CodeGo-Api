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
import { type SVGProps } from 'react'
import { cn } from '@/lib/utils'

export function Logo({ className, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg
      id='code-go-logo'
      viewBox='0 0 24 24'
      xmlns='http://www.w3.org/2000/svg'
      height='24'
      width='24'
      fill='none'
      className={cn('size-6', className)}
      {...props}
    >
      <title>codego-api</title>
      <rect x='1' y='1' width='22' height='22' rx='6' fill='#f8fbff' />
      <rect
        x='1'
        y='1'
        width='22'
        height='22'
        rx='6'
        stroke='#dbeafe'
        strokeWidth='0.8'
      />
      <path
        d='M8.1 5.5 3.7 12l4.4 6.5 1.6-1.05L6.1 12l3.6-5.45Z'
        fill='url(#code-go-core)'
      />
      <path
        d='m15.9 5.5-1.6 1.05 3.6 5.45-3.6 5.45 1.6 1.05 4.4-6.5Z'
        fill='url(#code-go-core)'
      />
      <path
        d='M13.1 4.9 10.1 19.1'
        stroke='#0f172a'
        strokeWidth='1.7'
        strokeLinecap='round'
      />
      <circle cx='12' cy='12' r='1.25' fill='#38bdf8' opacity='.16' />
      <defs>
        <linearGradient
          id='code-go-core'
          x1='4'
          y1='3.5'
          x2='20'
          y2='20.5'
          gradientUnits='userSpaceOnUse'
        >
          <stop offset='0%' stopColor='#38bdf8' />
          <stop offset='100%' stopColor='#2563eb' />
        </linearGradient>
      </defs>
    </svg>
  )
}
