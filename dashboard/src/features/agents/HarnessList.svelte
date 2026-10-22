<script lang="ts">
  // Feature: Harness cards - click to filter sessions
  import { Ghost, Bot, MousePointer, FileText, Rocket, Unlock, ChevronRight } from 'lucide-svelte';

  interface HarnessStats {
    harness: string;
    sessions: number;
    active: number;
    tokensSaved: number;
  }

  interface Props {
    stats: HarnessStats[];
    onSelectHarness: (harness: string) => void;
  }

  let { stats, onSelectHarness }: Props = $props();

  const icons: Record<string, typeof Ghost> = {
    'kirocrew': Ghost,
    'claude-code': Bot,
    'cursor': MousePointer,
    'codex': FileText,
    'antigravity': Rocket,
    'opencode': Unlock,
  };

  function formatTokens(n: number): string {
    if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
    if (n >= 1_000) return `${(n / 1_000).toFixed(0)}k`;
    return String(n);
  }
</script>

<div class="space-y-2">
  <div class="flex items-center justify-between">
    <span class="text-[10px] uppercase tracking-wider text-ds-muted">Harnesses</span>
    <span class="text-[9px] text-ds-inactive">{stats.length} detected</span>
  </div>
  
  <div class="space-y-1">
    {#each stats as item}
      {@const Icon = icons[item.harness] || Ghost}
      <button
        onclick={() => onSelectHarness(item.harness)}
        class="w-full flex items-center gap-3 p-2.5 rounded-lg bg-ds-surface border border-ds-line hover:border-ds-muted transition-colors text-left cursor-pointer"
      >
        <Icon class="w-4 h-4 text-ds-muted" strokeWidth={1.5} />
        
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <span class="text-sm text-ds-body">{item.harness}</span>
            {#if item.active > 0}
              <span class="w-1.5 h-1.5 rounded-full bg-ds-body"></span>
            {/if}
          </div>
          <span class="text-[10px] text-ds-muted">{item.sessions} session{item.sessions !== 1 ? 's' : ''}</span>
        </div>
        
        {#if item.tokensSaved > 0}
          <span class="text-xs text-ds-muted">{formatTokens(item.tokensSaved)}</span>
        {/if}
        
        <ChevronRight class="w-4 h-4 text-ds-inactive" strokeWidth={1.5} />
      </button>
    {/each}
  </div>
  
  {#if stats.length === 0}
    <p class="text-xs text-ds-muted text-center py-4">No harnesses detected</p>
  {/if}
</div>
