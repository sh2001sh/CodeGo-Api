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
import * as React from 'react'
import { ChevronDownIcon } from 'lucide-react'
import { enUS, fr, ja, ru, vi, zhCN } from 'react-day-picker/locale'
import { useTranslation } from 'react-i18next'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Input } from '@/components/ui/input'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'

const calendarLocales = {
  en: enUS,
  zh: zhCN,
  fr,
  ru,
  ja,
  vi,
} as const

interface DateTimePickerProps {
  id?: string
  value?: Date
  onChange?: (date: Date | undefined) => void
  placeholder?: string
  className?: string
  ariaInvalid?: boolean
  minDateTime?: Date
}

export function DateTimePicker({
  id,
  value,
  onChange,
  placeholder,
  className,
  ariaInvalid,
  minDateTime,
}: DateTimePickerProps) {
  const { t, i18n } = useTranslation()
  const placeholderText = placeholder ?? t('Select date')
  const calendarLocale =
    calendarLocales[i18n.language as keyof typeof calendarLocales] ?? enUS
  const [open, setOpen] = React.useState(false)
  const [date, setDate] = React.useState<Date | undefined>(value)
  const [month, setMonth] = React.useState<Date | undefined>(value)
  const [time, setTime] = React.useState<string>(() => {
    if (value) {
      const hours = value.getHours().toString().padStart(2, '0')
      const minutes = value.getMinutes().toString().padStart(2, '0')
      return `${hours}:${minutes}`
    }
    return '00:00'
  })
  const valueTime = value?.getTime()
  const dateTime = date?.getTime()

  React.useEffect(() => {
    if (valueTime !== dateTime) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setDate(value)
      setMonth(value)
      if (value) {
        const hours = value.getHours().toString().padStart(2, '0')
        const minutes = value.getMinutes().toString().padStart(2, '0')
        setTime(`${hours}:${minutes}`)
      } else {
        setTime('00:00')
      }
    }
  }, [dateTime, value, valueTime])

  const minimumTime = React.useMemo(() => {
    if (!minDateTime || !date || !isSameDay(date, minDateTime)) return undefined
    return formatTime(minDateTime)
  }, [date, minDateTime])
  const minimumDate = React.useMemo(
    () => (minDateTime ? startOfDay(minDateTime) : undefined),
    [minDateTime]
  )

  const handleDateSelect = (selectedDate: Date | undefined) => {
    if (selectedDate) {
      const [hours, minutes] = time.split(':').map(Number)
      const newDate = new Date(selectedDate)
      newDate.setHours(hours, minutes, 0, 0)
      if (minDateTime && newDate < minDateTime) {
        newDate.setTime(minDateTime.getTime())
        newDate.setSeconds(0, 0)
        setTime(formatTime(newDate))
      }
      setDate(newDate)
      setMonth(newDate)
      onChange?.(newDate)
      setOpen(false)
    } else {
      setDate(undefined)
      setMonth(undefined)
      onChange?.(undefined)
    }
  }

  const handleTimeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newTime = e.target.value
    setTime(newTime)

    if (date) {
      const [hours, minutes] = newTime.split(':').map(Number)
      const newDate = new Date(date)
      newDate.setHours(hours, minutes, 0, 0)
      if (minDateTime && newDate < minDateTime) {
        newDate.setTime(minDateTime.getTime())
        newDate.setSeconds(0, 0)
        setTime(formatTime(newDate))
      }
      setDate(newDate)
      onChange?.(newDate)
    }
  }

  const handleClear = () => {
    setDate(undefined)
    setMonth(undefined)
    setTime('00:00')
    onChange?.(undefined)
  }

  return (
    <div className={cn('flex gap-2', className)}>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger
          render={
            <Button
              variant='outline'
              id={id}
              type='button'
              aria-invalid={ariaInvalid}
              className={cn(
                'flex-1 justify-between font-normal',
                !date && 'text-muted-foreground'
              )}
            />
          }
        >
          {date ? dayjs(date).format('YYYY-MM-DD') : placeholderText}
          <ChevronDownIcon className='h-4 w-4 opacity-50' />
        </PopoverTrigger>
        <PopoverContent className='w-auto overflow-hidden p-0' align='start'>
          <Calendar
            mode='single'
            selected={date}
            month={month}
            onMonthChange={setMonth}
            startMonth={minimumDate ? startOfMonth(minimumDate) : undefined}
            captionLayout='dropdown'
            onSelect={handleDateSelect}
            locale={calendarLocale}
            disabled={(candidate) =>
              minimumDate ? startOfDay(candidate) < minimumDate : false
            }
          />
        </PopoverContent>
      </Popover>
      <Input
        type='time'
        value={time}
        min={minimumTime}
        onChange={handleTimeChange}
        className='w-32 appearance-none [&::-webkit-calendar-picker-indicator]:hidden [&::-webkit-calendar-picker-indicator]:appearance-none'
        disabled={!date}
      />
      {date && (
        <Button
          type='button'
          variant='outline'
          size='icon'
          onClick={handleClear}
          className='shrink-0'
          aria-label='Clear'
        >
          <span aria-hidden='true'>✕</span>
        </Button>
      )}
    </div>
  )
}

function formatTime(date: Date) {
  return `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`
}

function startOfDay(date: Date) {
  const value = new Date(date)
  value.setHours(0, 0, 0, 0)
  return value
}

function startOfMonth(date: Date) {
  const value = startOfDay(date)
  value.setDate(1)
  return value
}

function isSameDay(left: Date, right: Date) {
  return (
    left.getFullYear() === right.getFullYear() &&
    left.getMonth() === right.getMonth() &&
    left.getDate() === right.getDate()
  )
}
