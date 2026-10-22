<script lang="ts">
  // Feature: Tokens - Summary card with donut visualization
  import type { TokenSaving } from '../../shared/data/types';
  import { formatTokens, formatPercent } from '../../shared/data/transform';
  import { scaleIn } from '../../shared/ui/animations';

  interface Props {
    tokens: TokenSaving;
    label?: string;
  }

  let { tokens, label }: Props = $props();

  const total = $derived(tokens.work_tokens + tokens.cache_tokens);
  const saved = $derived(tokens.work_saved + tokens.cache_saved);
  const pct = $derived(total > 0 ? (saved / total) * 100 : 0);
  
  // SVG donut calculations
  const radius = 36;
  const circumference = 2 * Math.PI * radius;
  const offset = $derived(circumference - (pct / 100) * circumference);
</script>

<div use:scaleIn class="bg-ds-surface border border-ds-line rounded-lg p-3">
  <div class="flex items-center gap-3">
    <!-- Donut chart -->
    <svg class="w-20 h-20 -rotate-90" viewBox="0 0 100 100">
      <!-- Background circle -->
      <circle cx="50" cy="50" r={radius} fill="none" stroke="currentColor" 
        class="text-ds-line" stroke-width="8" />
      <!-- Progress arc -->
      <circle cx="50" cy="50" r={radius} fill="none" stroke="currentColor"
        class="text-ds-accent" stroke-width="8" stroke-linecap="round"
        stroke-dasharray={circumference} stroke-dashoffset={offset} />
    </svg>
    
    <!-- Stats -->
    <div class="flex-1 min-w-0">
      <div class="text-2xl font-bold text-ds-body">{formatPercent(pct, 0)}</div>
      <div class="text-[10px] text-ds-muted">{label || 'tokens saved'}</div>
      <div class="mt-1.5 space-y-0.5 text-[9px]">
        <div class="flex justify-between">
          <span class="text-ds-inactive">Work</span>
          <span class="text-ds-body">{formatTokens(tokens.work_saved)} / {formatTokens(tokens.work_tokens)}</span>
        </div>
        <div class="flex justify-between">
          <span class="text-ds-inactive">Cache</span>
          <span class="text-ds-body">{formatTokens(tokens.cache_saved)} / {formatTokens(tokens.cache_tokens)}</span>
        </div>
      </div>
    </div>
  </div>
</div>
