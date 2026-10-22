<script lang="ts">
  // Feature: Agents - Premium live agent card with animated character
  import type { Spawn } from '../../shared';
  import { glowPulse } from '../../shared';
  import AnimatedAgent from './AnimatedAgent.svelte';

  interface Props {
    spawn: Spawn;
    harness?: string;
  }

  let { spawn, harness = 'kirocrew' }: Props = $props();
  
  const modelName = $derived(spawn.to_model.replace('claude-', '').replace(/-\d+$/, ''));
</script>

<div 
  use:glowPulse
  class="relative overflow-hidden rounded-xl p-3 border border-ds-accent/20 bg-gradient-to-br from-ds-surface via-ds-surface to-ds-accent/5"
>
  <!-- Animated gradient overlay -->
  <div class="absolute inset-0 bg-gradient-to-r from-transparent via-ds-accent/5 to-transparent -translate-x-full animate-shimmer"></div>
  
  <div class="relative flex items-center gap-3">
    <!-- Animated character -->
    <AnimatedAgent {harness} agentState="working" size={40} />
    
    <!-- Info -->
    <div class="flex-1 min-w-0">
      <div class="flex items-center gap-2">
        <span class="text-xs font-semibold text-ds-body">{modelName}</span>
        <span class="px-1.5 py-0.5 text-[8px] rounded-full bg-ds-accent/20 text-ds-accent font-medium uppercase tracking-wider">
          live
        </span>
      </div>
      
      {#if spawn.state === 'switched' && spawn.from_model !== spawn.to_model}
        <p class="text-[10px] text-ds-muted mt-0.5 flex items-center gap-1">
          <span class="text-ds-inactive">{spawn.from_model.replace('claude-', '').split('-')[0]}</span>
          <svg class="w-3 h-3 text-ds-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M5 12h14M12 5l7 7-7 7"/>
          </svg>
          <span class="text-ds-accent font-medium">{modelName}</span>
        </p>
      {:else}
        <p class="text-[10px] text-ds-muted mt-0.5">Processing request...</p>
      {/if}
    </div>
    
    <!-- Spinner -->
    <div class="relative w-8 h-8">
      <svg class="w-8 h-8 animate-spin" viewBox="0 0 24 24" fill="none">
        <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" class="text-ds-line" />
        <path d="M12 2a10 10 0 0 1 10 10" stroke="currentColor" stroke-width="2" stroke-linecap="round" class="text-ds-accent" />
      </svg>
    </div>
  </div>
</div>

<style>
  @keyframes shimmer {
    to { transform: translateX(200%); }
  }
  .animate-shimmer {
    animation: shimmer 2s infinite;
  }
</style>
