<script lang="ts">
  // Feature: Tokens - Cost savings display
  import type { CostSaved } from '../../shared/data/types';
  import { formatUSD, formatPercent } from '../../shared/data/transform';

  interface Props {
    cost: CostSaved;
  }

  let { cost }: Props = $props();
</script>

<div class="bg-ds-surface border border-ds-line rounded-lg p-2.5">
  <div class="flex items-center justify-between">
    <div>
      <div class="text-[10px] text-ds-muted uppercase tracking-wider">Cost Saved</div>
      <div class="text-lg font-bold text-ds-accent">{formatUSD(cost.saved_usd)}</div>
    </div>
    <div class="text-right">
      <div class="text-xs text-ds-body">{formatPercent(cost.saved_percent, 0)}</div>
      <div class="text-[9px] text-ds-inactive">
        {formatUSD(cost.actual_cost_usd)} / {formatUSD(cost.baseline_cost_usd)}
      </div>
    </div>
  </div>
  {#if cost.quality !== 'exact'}
    <div class="mt-1 text-[8px] text-ds-warning">
      ⚠️ {cost.quality} pricing data
    </div>
  {/if}
</div>
