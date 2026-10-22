<script lang="ts">
  // App Shell: Layout, navigation, composition
  // Features handle their own domain logic
  import { onMount, onDestroy } from 'svelte';
  import { getSessionState } from './shared/state';
  import { AgentsList } from './features/agents';
  import { TokensSummary, CostSaved } from './features/tokens';
  import { CompactorStats } from './features/compactor';

  const session = getSessionState();
  let activeTab = $state<'agents' | 'tokens' | 'compactor'>('agents');

  // Count live agents for badge
  const liveCount = $derived(
    session.panel?.spawns.filter((s) => !s.tokens_label || s.tokens.work_tokens === 0).length ?? 0
  );

  onMount(() => session.startAutoRefresh(3000));
  onDestroy(() => session.stopAutoRefresh());
</script>

<div class="min-h-screen bg-ds-bg text-ds-body">
  <!-- Header -->
  <header class="sticky top-0 z-10 bg-ds-bg/95 backdrop-blur border-b border-ds-line px-3 py-2">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-2">
        <!-- Logo: Downshift mark -->
        <svg class="w-6 h-6" viewBox="0 0 24 24" fill="none">
          <path d="M7 7L12 12L7 17" stroke="currentColor" stroke-width="2" stroke-linecap="round" class="text-ds-body"/>
          <path d="M12 7L17 12L12 17" stroke="currentColor" stroke-width="2" stroke-linecap="round" class="text-ds-body"/>
          <path d="M17 7L22 12L17 17" stroke="currentColor" stroke-width="2" stroke-linecap="round" class="text-ds-accent"/>
        </svg>
        <span class="text-sm font-semibold text-ds-body">Downshift</span>
      </div>
      
      <div class="flex items-center gap-2">
        {#if session.loading}
          <div class="w-3 h-3 rounded-full border-2 border-ds-accent border-t-transparent animate-spin"></div>
        {:else if session.isConnected}
          <div class="w-2 h-2 rounded-full bg-ds-accent"></div>
        {:else}
          <div class="w-2 h-2 rounded-full bg-ds-warning"></div>
        {/if}
        <button onclick={() => session.toggleMock()} class="text-[9px] text-ds-inactive hover:text-ds-muted">
          {session.useMock ? 'mock' : 'live'}
        </button>
      </div>
    </div>
  </header>

  <!-- Navigation tabs -->
  <nav class="px-3 pt-2 pb-1 flex gap-1 border-b border-ds-line bg-ds-bg">
    <button
      onclick={() => activeTab = 'agents'}
      class="px-3 py-1.5 text-xs rounded-md transition-colors flex items-center gap-1.5"
      class:bg-ds-surface={activeTab === 'agents'}
      class:text-ds-body={activeTab === 'agents'}
      class:text-ds-muted={activeTab !== 'agents'}
    >
      Agents
      {#if liveCount > 0}
        <span class="px-1.5 py-0.5 text-[9px] rounded-full bg-ds-accent text-ds-bg font-medium">
          {liveCount}
        </span>
      {/if}
    </button>
    <button
      onclick={() => activeTab = 'tokens'}
      class="px-3 py-1.5 text-xs rounded-md transition-colors"
      class:bg-ds-surface={activeTab === 'tokens'}
      class:text-ds-body={activeTab === 'tokens'}
      class:text-ds-muted={activeTab !== 'tokens'}
    >
      Tokens
    </button>
    <button
      onclick={() => activeTab = 'compactor'}
      class="px-3 py-1.5 text-xs rounded-md transition-colors"
      class:bg-ds-surface={activeTab === 'compactor'}
      class:text-ds-body={activeTab === 'compactor'}
      class:text-ds-muted={activeTab !== 'compactor'}
    >
      Compactor
    </button>
  </nav>

  <!-- Content -->
  <main class="p-3">
    {#if session.error}
      <div class="bg-ds-surface border border-ds-warning/30 rounded-lg p-3 text-center">
        <p class="text-xs text-ds-warning">⚠️ {session.error}</p>
        <button onclick={() => session.refresh()} class="mt-2 text-[10px] text-ds-accent hover:underline">
          Retry
        </button>
      </div>
    {:else if !session.panel}
      <div class="flex items-center justify-center py-12">
        <div class="w-6 h-6 rounded-full border-2 border-ds-accent border-t-transparent animate-spin"></div>
      </div>
    {:else}
      {#if activeTab === 'agents'}
        <AgentsList spawns={session.panel.spawns} />
      {:else if activeTab === 'tokens'}
        <div class="space-y-3">
          <TokensSummary tokens={session.panel.tokens} />
          {#if session.panel.cost_saved}
            <CostSaved cost={session.panel.cost_saved} />
          {/if}
        </div>
      {:else if activeTab === 'compactor'}
        {#if session.panel.compactor}
          <CompactorStats compactor={session.panel.compactor} />
        {:else}
          <div class="text-center py-8">
            <p class="text-xs text-ds-muted">No compactor data</p>
            <p class="text-[10px] text-ds-inactive mt-1">
              Run <code class="px-1 py-0.5 bg-ds-surface rounded">downshift compactor -- cmd</code>
            </p>
          </div>
        {/if}
      {/if}
    {/if}
  </main>

  <!-- Footer -->
  <footer class="fixed bottom-0 left-0 right-0 px-3 py-1.5 bg-ds-bg/95 backdrop-blur border-t border-ds-line">
    <div class="flex items-center justify-between text-[9px] text-ds-inactive">
      <span>Session: {session.panel?.session ?? '—'}</span>
      {#if session.lastUpdate}
        <span>Updated {session.lastUpdate.toLocaleTimeString()}</span>
      {/if}
    </div>
  </footer>
</div>
