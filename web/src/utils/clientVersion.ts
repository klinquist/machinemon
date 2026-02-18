export function clientVersionLabel(version: string): string {
  const trimmed = (version || '').trim();
  if (!trimmed) return 'client unknown';

  const withoutDirty = trimmed.replace(/-dirty$/i, '');
  const withoutGitHash = withoutDirty.replace(/-\d+-g[0-9a-f]+$/i, '');
  if (!withoutGitHash) return 'client unknown';

  const haIntegrationMatch = withoutGitHash.match(/^(?:client-)?ha-integration-(.+)$/i);
  if (haIntegrationMatch) {
    return `client-ha-${haIntegrationMatch[1]}`;
  }

  const haShortMatch = withoutGitHash.match(/^client-ha-(.+)$/i);
  if (haShortMatch) {
    return `client-ha-${haShortMatch[1]}`;
  }

  if (withoutGitHash.startsWith('v')) return `client ${withoutGitHash}`;
  if (/^\d/.test(withoutGitHash)) return `client v${withoutGitHash}`;
  return `client ${withoutGitHash}`;
}
