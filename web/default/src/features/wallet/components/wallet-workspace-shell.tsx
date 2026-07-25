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
import type { ReactNode } from 'react'
import { SectionPageLayout } from '@/components/layout'

interface WalletWorkspaceShellProps {
  title: string
  description?: string
  main: ReactNode
  sidebar?: ReactNode
  framedMain?: boolean
}

export function WalletWorkspaceShell(props: WalletWorkspaceShellProps) {
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{props.title}</SectionPageLayout.Title>
      {props.description ? (
        <SectionPageLayout.Description>
          {props.description}
        </SectionPageLayout.Description>
      ) : null}
      <SectionPageLayout.Content>
        <div
          className={
            props.sidebar
              ? 'mx-auto grid w-full max-w-[1600px] items-start gap-5 min-[1200px]:grid-cols-[minmax(0,1fr)_288px] 2xl:grid-cols-[minmax(0,1fr)_320px]'
              : 'mx-auto w-full max-w-[1360px]'
          }
        >
          {props.framedMain === false ? (
            <div className='min-w-0'>{props.main}</div>
          ) : (
            <div className='app-page-shell min-w-0 p-4 sm:p-5'>
              {props.main}
            </div>
          )}
          {props.sidebar ? (
            <aside className='min-[1200px]:sticky min-[1200px]:top-5'>
              {props.sidebar}
            </aside>
          ) : null}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
