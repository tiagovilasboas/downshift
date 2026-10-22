<script lang="ts">
  // Feature: Agents - Premium finished agent card
  import type { Spawn } from '../../shared';
  import { formatPercent } from '../../shared';
  import { cardHover } from '../../shared';

  interface Props {
    spawn: Spawn;
  }

  let { spawn }: Props = $props();

  const total = $derived(spawn.tokens.work_tokens + spawn.tokens.cache_tokens);
  const saved = $derived(spawn.tokens.work_saved + spawn.tokens.cache_saved);
  const pct = $derived(total > 0 ? (saved / total) * 100 : 0);
  
  const modelFrom = $derived(spawn.from_model.replace('claude-', '').replace(/-\d+$/, ''));
  const modelTo = $derived(spawn.to_model.replace('claude-', '').replace(/-\d+$/, ''));
  const didSwitch = $derived(spawn.state === 'switched' && spawn.from_model !== spawn.to_model);
  
  // State-based styling
  const stateConfig = $derived({
    switched: { color: 'text-ds-accent', bg: 'bg-ds-accent/10', border: 'border-ds-accent/20', icon: '↓' },
    held: { color: 'text-ds-muted', bg: 'bg-ds-muted/10', border: 'border-ds-line', icon: '—' },
    paused: { color: 'text-ds-warning', bg: 'bg-ds-warning/10', border: 'border-ds-warning/20', icon: '⏸' },
    control: { color: 'text-ds-muted', bg: 'bg-ds-muted/10', border: 'border-ds-line', icon: '◆' },
    blocked: { color: 'text-ds-warning', bg: 'bg-ds-warning/10', border: 'border-ds-warning/20', icon: '✕' },
  }[spawn.state] || { color: 'text-ds-muted', bg: 'bg-ds-muted/10', border: 'border-ds-line', icon: '?' });
</script>

<div 
  use:cardHover
  class="relative rounded-xl p-3 border {stateConfig.border} bg-ds-surface/80 backdrop-blur-sm"
>
  <div class="flex items-center gap-3">
    <!-- Status indicator -->
    <div class="relative">
      <div class="w-10 h-10 rounded-full {stateConfig.bg} flex items-center justify-center">
        {#if didSwitch}
          <!-- Downshift icon for switched -->
          <svg class="w-5 h-5 {stateConfig.color}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M7 7L12 12L7 17" stroke-linecap="round"/>
            <path d="M12 7L17 12L12 17" stroke-linecap="round"/>
          </svg>
        {:else}
          <span class="text-sm {stateConfig.color}">{stateConfig.icon}</span>
        {/if}
      </div>
      
      <!-- Completion checkmark -->
      <div class="absolute -bottom-0.5 -right-0.5 w-4 h-4 rounded-full bg-ds-bg flex items-center justify-center">
        <div class="w-3 h-3 rounded-full bg-ds-accent/20 flex items-center justify-center">
          <svg class="w-2 h-2 text-ds-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
            <path d="M5 12l5 5L20 7" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </div>
      </div>
    </div>
    
    <!-- Info -->
    <div class="flex-1 min-w-0">
      <div class="flex items-center gap-2">
        {#if didSwitch}
          <span class="text-[10px] text-ds-inactive">{modelFrom}</span>
          <svg class="w-3 h-3 text-ds-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M5 12h14M12 5l7 7-7 7"/>
          </svg>
          <span class="text-xs font-semibold text-ds-body">{modelTo}</span>
        {:else}
          <span class="text-xs font-semibold text-ds-body">{modelTo}</span>
          <span class="px-1.5 py-0.5 text-[8px] rounded-full {stateConfig.bg} {stateConfig.color} font-medium uppercase tracking-wider">
            {spawn.state}
          </span>
        {/if}
      </div>
      
      <p class="text-[10px] text-ds-inactive mt-0.5 truncate">{spawn.tokens_label}</p>
    </div>
    
    <!-- Savings badge -->
    {#if pct > 0}
      <div class="flex flex-col items-end">
        <span class="text-sm font-bold text-ds-accent">{formatPercent(pct, 0)}</span>
        <span class="text-[8px] text-ds-inactive uppercase tracking-wider">saved</span>
      </div>
    {/if}
  </div>
  
  <!-- Subtle progress bar at bottom -->
  {#if pct > 0}
    <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-ds-line/50 rounded-b-xl overflow-hidden">
      <div class="h-full bg-gradient-to-r from-ds-accent/50 to-ds-accent" style="width: {pct}%"></div>
    </div>
  {/if}
</div>
