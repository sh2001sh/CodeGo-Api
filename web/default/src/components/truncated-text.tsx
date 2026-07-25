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
import { cn } from '@/lib/utils'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

interface TruncatedTextProps {
  text: string
  className?: string
  maxWidth?: string
  side?: 'top' | 'bottom' | 'left' | 'right'
}

export function TruncatedText({
  text,
  className,
  maxWidth = 'max-w-[200px]',
  side = 'top',
}: TruncatedTextProps) {
  return (
    <TooltipProvider delay={300}>
      <Tooltip>
        <TooltipTrigger
          render={
            <span className={cn('block truncate', maxWidth, className)} />
          }
        >
          {text}
        </TooltipTrigger>
        <TooltipContent side={side} className='max-w-xs break-all'>
          {text}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
