<script lang="ts">
  // Feature: Compactor - Stats display
  import type { Compactor } from '../../shared/data/types';
  import { formatBytes, formatTokens, formatPercent } from '../../shared/data/transform';
  import { progressFill } from '../../shared/ui/animations';

  interface Props {
    compactor: Compactor;
  }

  let { compactor }: Props = $props();

  const rate = $derived(
    compactor.commands > 0 ? (compactor.compacted / compactor.commands) * 100 : 0
  );
  const bytesSavedPct = $derived(
    compactor.raw_bytes > 0 ? (compactor.saved_bytes / compactor.raw_bytes) * 100 : 0
  );
</script>

<div class="bg-ds-surface border border-ds-line rounded-lg p-3 space-y-3">
  <div class="flex items-center justify-between">
    <div>
      <div class="text-[10px] text-ds-muted uppercase tracking-wider">Compactor</div>
      <div class="text-lg font-bold text-ds-body">
        {compactor.compacted}<span class="text-ds-inactive text-sm">/{compactor.commands}</span>
      </div>
    </div>
    <div class="text-right">
      <div class="text-sm font-medium text-ds-body">{formatPercent(rate, 0)}</div>
      <div class="text-[9px] text-ds-inactive">compacted</div>
    </div>
  </div>

  <!-- Progress bar -->
  <div class="h-1.5 bg-ds-line rounded-full overflow-hidden">
    <div use:progressFill={rate} class="h-full bg-ds-body rounded-full"></div>
  </div>

  <!-- Details -->
  <div class="grid grid-cols-2 gap-2 text-[9px]">
    <div>
      <div class="text-ds-inactive">Bytes saved</div>
      <div class="text-ds-body font-medium">
        {formatBytes(compactor.saved_bytes)} 
        <span class="text-ds-muted">({formatPercent(bytesSavedPct, 0)})</span>
      </div>
    </div>
    <div>
      <div class="text-ds-inactive">Est. tokens saved</div>
      <div class="text-ds-body font-medium">{formatTokens(compactor.estimated_tokens_saved)}</div>
    </div>
  </div>
</div>
