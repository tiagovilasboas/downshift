<script lang="ts">
  // Feature: Agents - Simple live agent card
  import type { Spawn } from '../../shared';
  import { Activity } from 'lucide-svelte';

  interface Props {
    spawn: Spawn;
  }

  let { spawn }: Props = $props();
  
  const modelName = $derived(spawn.to_model.replace('claude-', '').replace(/-\d+$/, ''));
</script>

<div class="flex items-center gap-3 p-3 rounded-lg bg-ds-surface border border-ds-line">
  <!-- Simple icon -->
  <div class="w-8 h-8 rounded-md bg-ds-bg flex items-center justify-center">
    <Activity class="w-4 h-4 text-ds-muted" strokeWidth={1.5} />
  </div>
  
  <!-- Info -->
  <div class="flex-1 min-w-0">
    <div class="flex items-center gap-2">
      <span class="text-xs font-medium text-ds-body">{modelName}</span>
      <span class="w-1.5 h-1.5 rounded-full bg-ds-body"></span>
    </div>
    
    {#if spawn.state === 'switched' && spawn.from_model !== spawn.to_model}
      <p class="text-[10px] text-ds-muted mt-0.5">
        {spawn.from_model.replace('claude-', '').split('-')[0]} → {modelName}
      </p>
    {:else}
      <p class="text-[10px] text-ds-muted mt-0.5">Processing...</p>
    {/if}
  </div>
  
  <!-- Simple spinner -->
  <div class="w-4 h-4 rounded-full border-2 border-ds-line border-t-ds-muted animate-spin"></div>
</div>
