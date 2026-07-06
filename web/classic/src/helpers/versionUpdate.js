/*
Copyright (C) 2025 QuantumNous

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

const VERSION_PATTERN =
  /^v?(\d+(?:\.\d+){2,4})(?:-(alpha|beta|rc|patch)\.(\d+)(?:-fork\.(\d+))?)?$/;
const GIT_DESCRIBE_SUFFIX_PATTERN = /-\d+-g[0-9a-f]+(?:-dirty)?$/i;

const RELEASE_RANK = {
  alpha: 0,
  beta: 1,
  rc: 2,
  stable: 3,
  patch: 4,
};

export function parseUpdateVersion(version) {
  const normalized = version?.trim().replace(GIT_DESCRIBE_SUFFIX_PATTERN, '');
  if (!normalized) return null;

  const match = VERSION_PATTERN.exec(normalized);
  if (!match) return null;
  if (match[2] !== 'rc' && match[4]) return null;

  return {
    numbers: match[1].split('.').map(Number),
    release: match[2] ?? 'stable',
    releaseNumber: match[3] ? Number(match[3]) : 0,
    forkNumber: match[4] ? Number(match[4]) : -1,
  };
}

export function compareUpdateVersions(leftVersion, rightVersion) {
  const left = parseUpdateVersion(leftVersion);
  const right = parseUpdateVersion(rightVersion);
  if (!left || !right) return null;

  const numberCount = Math.max(left.numbers.length, right.numbers.length);
  for (let index = 0; index < numberCount; index += 1) {
    const diff = (left.numbers[index] ?? 0) - (right.numbers[index] ?? 0);
    if (diff !== 0) return diff;
  }

  const releaseDiff = RELEASE_RANK[left.release] - RELEASE_RANK[right.release];
  if (releaseDiff !== 0) return releaseDiff;

  const releaseNumberDiff = left.releaseNumber - right.releaseNumber;
  if (releaseNumberDiff !== 0) return releaseNumberDiff;

  return left.forkNumber - right.forkNumber;
}

export function isUpdateVersionNewer(latestVersion, currentVersion) {
  if (!currentVersion) return false;
  const comparison = compareUpdateVersions(latestVersion, currentVersion);
  return comparison !== null && comparison > 0;
}
