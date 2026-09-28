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
import { useFormContext, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { sideDrawerSwitchItemClassName } from '@/components/drawer-layout'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { toIntlLocale } from '@/i18n/languages'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'

import type { ApiKeyFormValues } from '../lib/api-key-form'
import {
  formatQuotaResetSchedule,
  quotaResetWeekdays,
} from '../lib/quota-reset'

interface ApiKeyQuotaResetFieldsProps {
  serverTimezone?: string
  nextResetDisplay?: string
}

export function ApiKeyQuotaResetFields(props: ApiKeyQuotaResetFieldsProps) {
  const { t, i18n } = useTranslation()
  const form = useFormContext<ApiKeyFormValues>()
  const [period, day, time] = useWatch({
    control: form.control,
    name: ['quota_reset_period', 'quota_reset_day', 'quota_reset_time'],
  })
  const enabled = !!period && period !== 'never'
  const weekdays = quotaResetWeekdays(t).map((label, index) => ({
    value: String(index + 1),
    label,
  }))
  const periods = [
    { value: 'daily', label: t('Daily') },
    { value: 'weekly', label: t('Weekly') },
    { value: 'monthly', label: t('Monthly') },
  ]
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const currency = getCurrencyDisplay()
  const dirty = form.formState.dirtyFields
  const scheduleChanged =
    dirty.quota_reset_period ||
    dirty.quota_reset_day ||
    dirty.quota_reset_time ||
    dirty.quota_reset_amount_dollars

  return (
    <>
      <FormField
        control={form.control}
        name='quota_reset_period'
        render={({ field }) => (
          <FormItem className={sideDrawerSwitchItemClassName()}>
            <div className='flex flex-col gap-0.5'>
              <FormLabel>{t('Automatic quota reset')}</FormLabel>
              <FormDescription>
                {t(
                  'Restore this key to a fixed amount each period. Unused quota does not carry over.'
                )}
              </FormDescription>
            </div>
            <FormControl>
              <Switch
                checked={enabled}
                onCheckedChange={(checked) => {
                  field.onChange(checked ? 'monthly' : 'never')
                  if (checked) {
                    form.setValue('quota_reset_day', 1, { shouldDirty: true })
                  }
                }}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
      {enabled && (
        <div className='grid gap-4 sm:grid-cols-2'>
          <FormField
            control={form.control}
            name='quota_reset_period'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Reset period')}</FormLabel>
                <Select
                  items={periods}
                  value={field.value}
                  onValueChange={(value) => {
                    if (!value) return
                    field.onChange(value)
                    form.setValue(
                      'quota_reset_day',
                      value === 'daily' ? 0 : 1,
                      { shouldDirty: true }
                    )
                  }}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    {periods.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='quota_reset_amount_dollars'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t('Reset amount ({{currency}})', {
                    currency: getCurrencyLabel(),
                  })}
                </FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    value={field.value ?? ''}
                    type='number'
                    min='0'
                    step={currency.meta.kind === 'tokens' ? 1 : 0.01}
                    onChange={(event) =>
                      field.onChange(
                        event.target.value === ''
                          ? undefined
                          : Number(event.target.value)
                      )
                    }
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          {period === 'weekly' && (
            <FormField
              control={form.control}
              name='quota_reset_day'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Reset weekday')}</FormLabel>
                  <Select
                    items={weekdays}
                    value={String(field.value ?? 1)}
                    onValueChange={(value) =>
                      value !== null && field.onChange(Number(value))
                    }
                  >
                    <FormControl>
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {weekdays.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
          )}
          {period === 'monthly' && (
            <FormField
              control={form.control}
              name='quota_reset_day'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Day of month')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      value={field.value ?? ''}
                      type='number'
                      min='1'
                      max='31'
                      step='1'
                      onChange={(event) =>
                        field.onChange(
                          event.target.value === ''
                            ? undefined
                            : Number(event.target.value)
                        )
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'If the day does not exist, reset on the last day of that month.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          )}
          <FormField
            control={form.control}
            name='quota_reset_time'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Reset time')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    value={field.value ?? '00:00'}
                    type='time'
                    step='60'
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <div className='text-muted-foreground space-y-1 text-xs sm:col-span-2'>
            <p>
              {formatQuotaResetSchedule(
                {
                  quota_reset_period: period,
                  quota_reset_day: day,
                  quota_reset_time: time,
                },
                t,
                locale
              )}
            </p>
            <p>
              {props.serverTimezone
                ? t('Server timezone: {{timezone}}', {
                    timezone: props.serverTimezone,
                  })
                : t('Times use the server timezone.')}
            </p>
            <p>
              {t(
                'Saving changes keeps the current balance. The new rule starts at the next scheduled time.'
              )}
            </p>
            {!scheduleChanged && props.nextResetDisplay && (
              <p>
                {t('Next reset')}: {props.nextResetDisplay}
              </p>
            )}
          </div>
        </div>
      )}
    </>
  )
}
