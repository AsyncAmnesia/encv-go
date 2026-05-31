let MOCK_SUFFIX = '.ae'

export function setMockSuffix(suffix: string): void {
  MOCK_SUFFIX = suffix
}

export function getMockSuffix(): string {
  return MOCK_SUFFIX
}
