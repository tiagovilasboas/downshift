<script lang="ts">
  // Feature: Agents - Clean list
  import type { Spawn } from '../../shared';
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
  
  const totalSwitched = $derived(spawns.filter(s => s.state === 'switched').length);
  const totalHeld = $derived(spawns.filter(s => s.state === 'held').length);
</script>

<section class="space-y-4">
  <!-- Live Agents -->
  {#if liveAgents.length > 0}
    <div>
      <div class="flex items-center justify-between mb-2">
        <span class="text-[10px] uppercase tracking-wider text-ds-muted">Live</span>
        <span class="text-[9px] text-ds-inactive">{liveAgents.length} active</span>
      </div>
      <div class="space-y-2">
        {#each liveAgents as spawn (spawn.tool_use)}
          <LiveAgent {spawn} />
        {/each}
      </div>
    </div>
  {/if}

  <!-- Finished Agents -->
  {#if finishedAgents.length > 0}
    <div>
      <div class="flex items-center justify-between mb-2">
        <span class="text-[10px] uppercase tracking-wider text-ds-muted">Completed</span>
        <div class="flex gap-2 text-[9px]">
          {#if totalSwitched > 0}
            <span class="text-ds-muted">{totalSwitched} switched</span>
          {/if}
          {#if totalHeld > 0}
            <span class="text-ds-inactive">{totalHeld} held</span>
          {/if}
        </div>
      </div>
      <div class="space-y-2">
        {#each finishedAgents as spawn (spawn.tool_use)}
          <FinishedAgent {spawn} />
        {/each}
      </div>
    </div>
  {/if}

  {#if spawns.length === 0}
    <EmptyState />
  {/if}
</section>
