<script lang="ts">
  // Feature: Agents - Live agent row with pulse animation
  import type { Spawn } from '../../shared';
  import { pulse } from '../../shared';

  interface Props {
    spawn: Spawn;
  }

  let { spawn }: Props = $props();
</script>

<div class="bg-ds-surface border border-ds-accent/30 rounded-lg p-2.5 flex items-center gap-2">
  <!-- Animated pulse bar -->
  <div class="relative w-1 h-10 rounded-full bg-ds-line overflow-hidden">
    <div use:pulse class="absolute inset-0 bg-ds-accent rounded-full"></div>
  </div>
  
  <div class="flex-1 min-w-0">
    <div class="flex items-center gap-1.5">
      <span class="text-[9px] px-1.5 py-0.5 rounded bg-ds-accent/20 text-ds-accent font-medium">
        {spawn.state}
      </span>
      <span class="text-[10px] font-mono text-ds-body truncate">
        {spawn.from_model.replace('claude-', '').split('-')[0]}
        {#if spawn.state === 'switched' && spawn.from_model !== spawn.to_model}
          <span class="text-ds-inactive">→</span>
          <span class="text-ds-accent">{spawn.to_model.replace('claude-', '').split('-')[0]}</span>
        {/if}
      </span>
    </div>
    <p class="text-[9px] text-ds-accent truncate mt-0.5">Running...</p>
  </div>
  
  <!-- Spinning indicator -->
  <div class="w-4 h-4 rounded-full border-2 border-ds-accent border-t-transparent animate-spin"></div>
</div>
