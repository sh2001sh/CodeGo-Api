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
import { memo } from 'react'
import { type UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

type GroupFormValues = {
  GroupRatio: string
  TopupGroupRatio: string
  UserUsableGroups: string
  GroupGroupRatio: string
  AutoGroups: string
  DefaultUseAutoGroup: boolean
  GroupSpecialUsableGroup: string
}

type GroupRatioFormProps = {
  form: UseFormReturn<GroupFormValues>
  onSave: (values: GroupFormValues) => Promise<void>
  isSaving: boolean
}

const JSON_FIELDS: Array<{
  name: Exclude<keyof GroupFormValues, 'DefaultUseAutoGroup'>
  label: string
  description: string
  rows: number
}> = [
  {
    name: 'GroupRatio',
    label: 'Group ratios',
    description: 'JSON map of token group names to their billing ratios.',
    rows: 8,
  },
  {
    name: 'TopupGroupRatio',
    label: 'Top-up group ratios',
    description:
      'Optional JSON map used when calculating wallet top-up pricing.',
    rows: 6,
  },
  {
    name: 'UserUsableGroups',
    label: 'Selectable groups',
    description:
      'JSON map of groups that users may select when creating API keys.',
    rows: 6,
  },
  {
    name: 'GroupGroupRatio',
    label: 'Inter-group overrides',
    description:
      'Nested JSON for user-group and token-group billing overrides.',
    rows: 8,
  },
  {
    name: 'AutoGroups',
    label: 'Auto assignment order',
    description: 'JSON array of groups tried by automatic routing.',
    rows: 6,
  },
  {
    name: 'GroupSpecialUsableGroup',
    label: 'Special usable group rules',
    description:
      'JSON rules that add or remove selectable groups for user groups.',
    rows: 8,
  },
]

export const GroupRatioForm = memo(function GroupRatioForm(
  props: GroupRatioFormProps
) {
  const { t } = useTranslation()

  return (
    <Form {...props.form}>
      <form
        onSubmit={props.form.handleSubmit(props.onSave)}
        className='space-y-6'
      >
        {JSON_FIELDS.map((fieldConfig) => (
          <FormField
            key={fieldConfig.name}
            control={props.form.control}
            name={fieldConfig.name}
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t(fieldConfig.label)}</FormLabel>
                <FormControl>
                  <Textarea rows={fieldConfig.rows} {...field} />
                </FormControl>
                <FormDescription>{t(fieldConfig.description)}</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        ))}

        <FormField
          control={props.form.control}
          name='DefaultUseAutoGroup'
          render={({ field }) => (
            <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
              <div className='space-y-0.5'>
                <FormLabel className='text-base'>
                  {t('Default to auto groups')}
                </FormLabel>
                <FormDescription>
                  {t(
                    'codego-api keys start with automatic routing when enabled.'
                  )}
                </FormDescription>
              </div>
              <FormControl>
                <Switch
                  checked={field.value}
                  onCheckedChange={field.onChange}
                />
              </FormControl>
            </FormItem>
          )}
        />

        <Button type='submit' disabled={props.isSaving}>
          {props.isSaving ? t('Saving...') : t('Save group ratios')}
        </Button>
      </form>
    </Form>
  )
})
