export interface StableVersion {
  readonly text: string;
  readonly numbers: readonly [bigint, bigint, bigint];
}

const stableVersionPattern = /^(?:aru\s+)?v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;

export function parseStableVersion(value: string): StableVersion | undefined {
  const match = stableVersionPattern.exec(value.trim());
  if (match === null || match[1] === undefined || match[2] === undefined || match[3] === undefined) {
    return undefined;
  }
  const numbers = [BigInt(match[1]), BigInt(match[2]), BigInt(match[3])] as const;
  return {
    text: `v${numbers[0]}.${numbers[1]}.${numbers[2]}`,
    numbers,
  };
}

export function isNewerStableVersion(candidate: string, installed: string): boolean {
  const have = parseStableVersion(installed);
  const want = parseStableVersion(candidate);
  if (have === undefined || want === undefined) {
    return false;
  }
  for (let index = 0; index < have.numbers.length; index += 1) {
    const current = have.numbers[index];
    const latest = want.numbers[index];
    if (current === undefined || latest === undefined) {
      return false;
    }
    if (latest > current) {
      return true;
    }
    if (latest < current) {
      return false;
    }
  }
  return false;
}
