// Shared: Pure data transformations
import type { TokenSaving, PanelOut, SpawnState } from './types';

export function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}k`;
  return n.toString();
}

export function formatBytes(bytes: number): string {
  if (bytes >= 1_000_000) return `${(bytes / 1_000_000).toFixed(1)} MB`;
  if (bytes >= 1_000) return `${(bytes / 1_000).toFixed(0)} KB`;
  return `${bytes} B`;
}

export function formatPercent(value: number, decimals = 0): string {
  return `${value.toFixed(decimals)}%`;
}

export function formatUSD(value: number): string {
  if (value < 0.01) return `$${value.toFixed(4)}`;
  if (value < 1) return `$${value.toFixed(3)}`;
  return `$${value.toFixed(2)}`;
}

export function savedPercent(saving: TokenSaving): { work: number; cache: number } {
  return {
    work: saving.work_tokens > 0 ? (saving.work_saved / saving.work_tokens) * 100 : 0,
    cache: saving.cache_tokens > 0 ? (saving.cache_saved / saving.cache_tokens) * 100 : 0,
  };
}

export function stateClass(state: SpawnState): string {
  const classes: Record<SpawnState, string> = {
    switched: 'badge-switched',
    held: 'badge-held',
    paused: 'badge-held',
    control: 'badge-held',
    blocked: 'badge-blocked',
  };
  return classes[state] || 'badge-held';
}

export function stateLabel(state: SpawnState): string {
  const labels: Record<SpawnState, string> = {
    switched: 'Switched',
    held: 'Held',
    paused: 'Paused',
    control: 'Control',
    blocked: 'Blocked',
  };
  return labels[state] || state;
}

export function shortModel(model: string): string {
  if (model.includes('opus')) return 'opus';
  if (model.includes('sonnet')) return 'sonnet';
  if (model.includes('haiku')) return 'haiku';
  if (model.includes('gpt-4o')) return 'gpt-4o';
  if (model.includes('gpt-4')) return 'gpt-4';
  if (model.includes('gpt-3')) return 'gpt-3.5';
  return model.slice(0, 12);
}

export function summaryMetrics(panel: PanelOut) {
  const percents = savedPercent(panel.tokens);
  const totalTokens = panel.tokens.work_tokens + panel.tokens.cache_tokens;
  const totalSaved = panel.tokens.work_saved + panel.tokens.cache_saved;

  return {
    totalTokens,
    totalSaved,
    totalPercent: totalTokens > 0 ? (totalSaved / totalTokens) * 100 : 0,
    workPercent: percents.work,
    cachePercent: percents.cache,
    decisionsTotal: panel.switched + panel.held,
    switchRate: panel.switched + panel.held > 0
      ? (panel.switched / (panel.switched + panel.held)) * 100
      : 0,
    compactorRate: panel.compactor
      ? (panel.compactor.compacted / panel.compactor.commands) * 100
      : 0,
  };
}
