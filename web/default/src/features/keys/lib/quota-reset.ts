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
import { API_KEY_STATUS } from '../constants'
import type { ApiKey } from '../types'

export type ApiKeyQuotaResetBlockReason =
  | 'unlimited'
  | 'missing-reset-amount'
  | 'expired'

type ResettableApiKey = Pick<
  ApiKey,
  'unlimited_quota' | 'quota_reset_amount' | 'status' | 'expired_time'
>

export function getApiKeyQuotaResetBlockReason(
  apiKey: ResettableApiKey,
  nowSeconds = Math.floor(Date.now() / 1000)
): ApiKeyQuotaResetBlockReason | null {
  if (apiKey.unlimited_quota) {
    return 'unlimited'
  }
  if (apiKey.quota_reset_amount <= 0) {
    return 'missing-reset-amount'
  }
  if (
    apiKey.status === API_KEY_STATUS.EXPIRED ||
    (apiKey.expired_time > 0 && apiKey.expired_time < nowSeconds)
  ) {
    return 'expired'
  }
  return null
}
