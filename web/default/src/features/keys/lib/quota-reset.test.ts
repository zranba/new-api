import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { API_KEY_STATUS } from '../constants'
import { getApiKeyQuotaResetBlockReason } from './quota-reset'

const baseApiKey = {
  unlimited_quota: false,
  quota_reset_amount: 500,
  status: API_KEY_STATUS.ENABLED,
  expired_time: -1,
}

describe('API key quota reset guard', () => {
  test('blocks unlimited keys', () => {
    assert.equal(
      getApiKeyQuotaResetBlockReason({
        ...baseApiKey,
        unlimited_quota: true,
      }),
      'unlimited'
    )
  })

  test('blocks keys without a configured reset amount', () => {
    assert.equal(
      getApiKeyQuotaResetBlockReason({
        ...baseApiKey,
        quota_reset_amount: 0,
      }),
      'missing-reset-amount'
    )
  })

  test('blocks expired keys by status or expiration time', () => {
    assert.equal(
      getApiKeyQuotaResetBlockReason({
        ...baseApiKey,
        status: API_KEY_STATUS.EXPIRED,
      }),
      'expired'
    )

    assert.equal(
      getApiKeyQuotaResetBlockReason(
        {
          ...baseApiKey,
          expired_time: 90,
        },
        100
      ),
      'expired'
    )
  })

  test('allows resettable limited keys', () => {
    assert.equal(getApiKeyQuotaResetBlockReason(baseApiKey), null)
  })
})
