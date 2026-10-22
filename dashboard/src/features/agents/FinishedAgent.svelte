<script lang="ts">
  // Feature: Agents - Finished agent row
  import type { Spawn } from '../../shared';
  import { formatPercent } from '../../shared';

  interface Props {
    spawn: Spawn;
  }

  let { spawn }: Props = $props();

  const total = $derived(spawn.tokens.work_tokens + spawn.tokens.cache_tokens);
  const saved = $derived(spawn.tokens.work_saved + spawn.tokens.cache_saved);
  const pct = $derived(total > 0 ? (saved / total) * 100 : 0);
</script>

<div class="bg-ds-surface border border-ds-line rounded-lg p-2 flex items-center gap-2">
  <div 
    class="w-0.5 h-8 rounded-full"
    class:bg-ds-body={spawn.state === 'switched'}
    class:bg-ds-inactive={spawn.state === 'held'}
    class:bg-ds-warning={spawn.state !== 'switched' && spawn.state !== 'held'}
  ></div>
  
  <div class="flex-1 min-w-0">
    <div class="flex items-center gap-1.5">
      <span class="text-[9px] px-1.5 py-0.5 rounded bg-ds-line text-ds-muted">{spawn.state}</span>
      <span class="text-[10px] font-mono text-ds-body truncate">
        {spawn.from_model.replace('claude-', '').split('-')[0]}
        {#if spawn.state === 'switched' && spawn.from_model !== spawn.to_model}
          <span class="text-ds-inactive">→</span>
          <span class="text-ds-accent">{spawn.to_model.replace('claude-', '').split('-')[0]}</span>
        {/if}
      </span>
    </div>
    <p class="text-[9px] text-ds-inactive truncate">{spawn.tokens_label}</p>
  </div>
  
  {#if pct > 0}
    <span class="text-[10px] font-mono text-ds-accent">{formatPercent(pct, 0)}</span>
  {/if}
</div>
