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

import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  compareUpdateVersions,
  isUpdateVersionNewer,
  parseUpdateVersion,
} from './version-update'

describe('update version comparison', () => {
  test('parses rc fork versions', () => {
    assert.deepEqual(parseUpdateVersion('v1.2.3-rc.4-fork.5'), {
      numbers: [1, 2, 3],
      release: 'rc',
      releaseNumber: 4,
      forkNumber: 5,
    })
  })

  test('parses alpha and beta versions', () => {
    assert.deepEqual(parseUpdateVersion('v1.2.3-alpha.1'), {
      numbers: [1, 2, 3],
      release: 'alpha',
      releaseNumber: 1,
      forkNumber: -1,
    })
    assert.deepEqual(parseUpdateVersion('v1.2.3-beta.2'), {
      numbers: [1, 2, 3],
      release: 'beta',
      releaseNumber: 2,
      forkNumber: -1,
    })
  })

  test('compares prerelease numbers numerically', () => {
    assert.equal(isUpdateVersionNewer('v1.2.3-rc.10', 'v1.2.3-rc.9'), true)
  })

  test('orders stable releases above rc forks for the same base version', () => {
    assert.equal(isUpdateVersionNewer('v1.2.3', 'v1.2.3-rc.4-fork.5'), true)
  })

  test('orders fork revisions above the same official rc', () => {
    assert.equal(
      isUpdateVersionNewer('v1.2.3-rc.4-fork.5', 'v1.2.3-rc.4'),
      true
    )
  })

  test('orders higher stable versions above lower rc forks', () => {
    assert.equal(isUpdateVersionNewer('v1.2.4', 'v1.2.3-rc.9-fork.9'), true)
  })

  test('does not treat lower stable versions as updates for newer rc forks', () => {
    assert.equal(isUpdateVersionNewer('v1.2.2', 'v1.2.3-rc.1-fork.1'), false)
  })

  test('does not show an update when current version is newer than latest', () => {
    assert.equal(isUpdateVersionNewer('v1.2.3', 'v1.2.4-rc.1-fork.1'), false)
  })

  test('orders patch releases above stable versions with the same base', () => {
    assert.equal(isUpdateVersionNewer('v0.9.27-patch.1', 'v0.9.27'), true)
    assert.equal(isUpdateVersionNewer('v0.9.27', 'v0.9.27-patch.1'), false)
    assert.equal(
      isUpdateVersionNewer('v0.9.27-patch.2', 'v0.9.27-patch.1'),
      true
    )
  })

  test('compares four and five segment versions numerically', () => {
    assert.deepEqual(parseUpdateVersion('v0.9.0.9.1'), {
      numbers: [0, 9, 0, 9, 1],
      release: 'stable',
      releaseNumber: 0,
      forkNumber: -1,
    })
    assert.equal(isUpdateVersionNewer('v0.9.0.9.1', 'v0.9.0.9'), true)
  })

  test('strips git describe suffixes before comparison', () => {
    assert.equal(
      isUpdateVersionNewer('v1.0.0-rc.16', 'v1.0.0-rc.16-3-gbc768bbc'),
      false
    )
    assert.equal(
      isUpdateVersionNewer('v1.0.0-rc.17', 'v1.0.0-rc.16-3-gbc768bbc'),
      true
    )
    assert.equal(
      isUpdateVersionNewer('v1.0.0-rc.16', 'v1.0.0-rc.16-3-gbc768bbc-dirty'),
      false
    )
  })

  test('does not report updates when either version cannot be parsed', () => {
    assert.equal(isUpdateVersionNewer('v1.2.3', 'local-build'), false)
    assert.equal(isUpdateVersionNewer('v1.2.3', undefined), false)
    assert.equal(
      isUpdateVersionNewer('v1.2.3', 'nightly-20260317-44fc10b'),
      false
    )
    assert.equal(isUpdateVersionNewer('nightly-20260409', 'v1.2.3'), false)
    assert.equal(isUpdateVersionNewer('local-build', 'local-build'), false)
    assert.equal(compareUpdateVersions('v1.2.3', 'local-build'), null)
  })
})
