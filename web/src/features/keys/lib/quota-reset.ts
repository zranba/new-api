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
import type { TFunction } from 'i18next'

import { formatNumber } from '@/lib/format'

import type { ApiKey } from '../types'

export function quotaResetWeekdays(t: TFunction) {
  return [
    t('Monday'),
    t('Tuesday'),
    t('Wednesday'),
    t('Thursday'),
    t('Friday'),
    t('Saturday'),
    t('Sunday'),
  ]
}

export function formatQuotaResetSchedule(
  rule: Pick<
    ApiKey,
    'quota_reset_period' | 'quota_reset_day' | 'quota_reset_time'
  >,
  t: TFunction,
  locale?: string
): string {
  const time = rule.quota_reset_time || '00:00'
  if (rule.quota_reset_period === 'daily') {
    return t('Daily at {{time}}', { time })
  }
  if (rule.quota_reset_period === 'weekly') {
    return t('Every {{weekday}} at {{time}}', {
      weekday: quotaResetWeekdays(t)[(rule.quota_reset_day || 1) - 1],
      time,
    })
  }
  if (rule.quota_reset_period === 'monthly') {
    return t('Monthly on day {{day}} at {{time}}', {
      day: formatNumber(rule.quota_reset_day || 1, locale),
      time,
    })
  }
  return ''
}
