<script lang="ts">
  // Feature: Agents - Main list container
  import type { Spawn } from '../../shared';
  import { fadeInStagger, pulse } from '../../shared';
  import LiveAgent from './LiveAgent.svelte';
  import FinishedAgent from './FinishedAgent.svelte';
  import EmptyState from './EmptyState.svelte';

  interface Props {
    spawns: Spawn[];
  }

  let { spawns }: Props = $props();

  // Separate running and finished agents
  const liveAgents = $derived(
    spawns.filter((s) => !s.tokens_label || s.tokens.work_tokens === 0)
  );
  const finishedAgents = $derived(
    spawns.filter((s) => s.tokens_label && s.tokens.work_tokens > 0)
  );
</script>

<section class="space-y-3">
  <!-- Live Agents -->
  {#if liveAgents.length > 0}
    <div>
      <div class="flex items-center gap-2 mb-2">
        <span use:pulse class="w-2 h-2 rounded-full bg-ds-accent"></span>
        <span class="text-[10px] font-medium text-ds-accent uppercase tracking-wider">Live</span>
        <span class="text-[9px] text-ds-inactive">({liveAgents.length})</span>
      </div>
      <div class="space-y-1.5" use:fadeInStagger={'.agent-item'}>
        {#each liveAgents as spawn (spawn.tool_use)}
          <div class="agent-item">
            <LiveAgent {spawn} />
          </div>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Finished Agents -->
  {#if finishedAgents.length > 0}
    <div>
      <div class="flex items-center gap-2 mb-2">
        <span class="w-2 h-2 rounded-full bg-ds-muted"></span>
        <span class="text-[10px] font-medium text-ds-muted uppercase tracking-wider">Finished</span>
        <span class="text-[9px] text-ds-inactive">({finishedAgents.length})</span>
      </div>
      <div class="space-y-1.5" use:fadeInStagger={'.agent-item'}>
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
