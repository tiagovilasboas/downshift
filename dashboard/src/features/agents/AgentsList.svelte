<script lang="ts">
  // Feature: Agents - Premium list with sections
  import type { Spawn } from '../../shared';
  import { fadeInStagger, breathe } from '../../shared';
  import LiveAgent from './LiveAgent.svelte';
  import FinishedAgent from './FinishedAgent.svelte';
  import EmptyState from './EmptyState.svelte';

  interface Props {
    spawns: Spawn[];
  }

  let { spawns }: Props = $props();

  const liveAgents = $derived(
    spawns.filter((s) => !s.tokens_label || s.tokens.work_tokens === 0)
  );
  const finishedAgents = $derived(
    spawns.filter((s) => s.tokens_label && s.tokens.work_tokens > 0)
  );
  
  // Stats
  const totalSwitched = $derived(spawns.filter(s => s.state === 'switched').length);
  const totalHeld = $derived(spawns.filter(s => s.state === 'held').length);
</script>

<section class="space-y-4">
  <!-- Live Agents Section -->
  {#if liveAgents.length > 0}
    <div>
      <div class="flex items-center gap-2 mb-3">
        <div use:breathe class="w-2 h-2 rounded-full bg-ds-accent"></div>
        <span class="text-[10px] font-semibold text-ds-accent uppercase tracking-wider">Live</span>
        <div class="flex-1 h-px bg-gradient-to-r from-ds-accent/30 to-transparent"></div>
        <span class="text-[9px] text-ds-inactive px-2 py-0.5 rounded-full bg-ds-surface">{liveAgents.length} active</span>
      </div>
      <div class="space-y-2" use:fadeInStagger={'.agent-item'}>
        {#each liveAgents as spawn (spawn.tool_use)}
          <div class="agent-item">
            <LiveAgent {spawn} />
          </div>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Finished Agents Section -->
  {#if finishedAgents.length > 0}
    <div>
      <div class="flex items-center gap-2 mb-3">
        <div class="w-2 h-2 rounded-full bg-ds-muted/50"></div>
        <span class="text-[10px] font-semibold text-ds-muted uppercase tracking-wider">Completed</span>
        <div class="flex-1 h-px bg-gradient-to-r from-ds-line to-transparent"></div>
        <div class="flex gap-1.5 text-[9px]">
          {#if totalSwitched > 0}
            <span class="px-2 py-0.5 rounded-full bg-ds-accent/10 text-ds-accent">{totalSwitched} switched</span>
          {/if}
          {#if totalHeld > 0}
            <span class="px-2 py-0.5 rounded-full bg-ds-surface text-ds-inactive">{totalHeld} held</span>
          {/if}
        </div>
      </div>
      <div class="space-y-2" use:fadeInStagger={'.agent-item'}>
        {#each finishedAgents as spawn (spawn.tool_use)}
          <div class="agent-item">
            <FinishedAgent {spawn} />
          </div>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Empty state -->
  {#if spawns.length === 0}
    <EmptyState />
  {/if}
</section>
