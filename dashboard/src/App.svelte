<script lang="ts">
  // App Shell: Layout, navigation, composition
  // Features handle their own domain logic
  import { onMount, onDestroy } from 'svelte';
  import { getSessionState } from './shared/state';
  import { fetchSessions } from './shared/data';
  import type { SessionSummary } from './shared/data';
  import { AgentsList } from './features/agents';
  import { TokensSummary, CostSaved } from './features/tokens';
  import { CompactorStats } from './features/compactor';
  import { SessionSelector } from './features/sessions';

  const session = getSessionState();
  let activeTab = $state<'agents' | 'tokens' | 'compactor' | 'sessions'>('agents');
  
  // Sessions state
  let sessions = $state<SessionSummary[]>([]);
  let harnesses = $state<string[]>([]);
  let selectedSession = $state<string | null>(null);
  let selectedHarness = $state<string | null>(null);
  let loadingSessions = $state(false);

  // Count live agents for badge
  const liveCount = $derived(
    session.panel?.spawns.filter((s) => !s.tokens_label || s.tokens.work_tokens === 0).length ?? 0
  );

  async function loadSessions() {
    loadingSessions = true;
    try {
      const data = await fetchSessions();
      sessions = data.sessions;
      harnesses = data.harnesses;
    } catch (e) {
      console.error('Failed to load sessions:', e);
    } finally {
      loadingSessions = false;
    }
  }

  function handleSelectSession(id: string | null) {
    selectedSession = id;
    if (id) {
      session.setSessionId(id);
      activeTab = 'agents'; // Switch to agents tab to show selected session
    } else {
      session.setSessionId(null); // Back to current session
    }
  }

  function handleSelectHarness(harness: string | null) {
    selectedHarness = harness;
  }

  onMount(() => {
    session.startAutoRefresh(3000);
    loadSessions();
  });
  onDestroy(() => session.stopAutoRefresh());
</script>

<div class="min-h-screen bg-ds-bg text-ds-body pb-10">
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
    <button
      onclick={() => { activeTab = 'sessions'; loadSessions(); }}
      class="px-3 py-1.5 text-xs rounded-md transition-colors flex items-center gap-1.5"
      class:bg-ds-surface={activeTab === 'sessions'}
      class:text-ds-body={activeTab === 'sessions'}
      class:text-ds-muted={activeTab !== 'sessions'}
    >
      Sessions
      {#if harnesses.length > 1}
        <span class="px-1.5 py-0.5 text-[9px] rounded-full bg-ds-surface text-ds-muted">
          {harnesses.length}
        </span>
      {/if}
    </button>
  </nav>

  <!-- Content -->
  <main class="p-3">
    {#if session.error && activeTab !== 'sessions'}
      <div class="bg-ds-surface border border-ds-warning/30 rounded-lg p-3 text-center">
        <p class="text-xs text-ds-warning">⚠️ {session.error}</p>
        <button onclick={() => session.refresh()} class="mt-2 text-[10px] text-ds-accent hover:underline">
          Retry
        </button>
      </div>
    {:else if !session.panel && activeTab !== 'sessions'}
      <div class="flex items-center justify-center py-12">
        <div class="w-6 h-6 rounded-full border-2 border-ds-accent border-t-transparent animate-spin"></div>
      </div>
    {:else}
      {#if activeTab === 'agents'}
        <AgentsList spawns={session.panel?.spawns ?? []} />
      {:else if activeTab === 'tokens'}
        <div class="space-y-3">
          {#if session.panel}
            <TokensSummary tokens={session.panel.tokens} />
            {#if session.panel.cost_saved}
              <CostSaved cost={session.panel.cost_saved} />
            {/if}
          {/if}
        </div>
      {:else if activeTab === 'compactor'}
        {#if session.panel?.compactor}
          <CompactorStats compactor={session.panel.compactor} />
        {:else}
          <div class="text-center py-8">
            <p class="text-xs text-ds-muted">No compactor data</p>
            <p class="text-[10px] text-ds-inactive mt-1">
              Run <code class="px-1 py-0.5 bg-ds-surface rounded">downshift compactor -- cmd</code>
            </p>
          </div>
        {/if}
      {:else if activeTab === 'sessions'}
        {#if loadingSessions}
          <div class="flex items-center justify-center py-12">
            <div class="w-6 h-6 rounded-full border-2 border-ds-accent border-t-transparent animate-spin"></div>
          </div>
        {:else}
          <SessionSelector
            {sessions}
            {harnesses}
            {selectedSession}
            {selectedHarness}
            onSelectSession={handleSelectSession}
            onSelectHarness={handleSelectHarness}
          />
        {/if}
      {/if}
    {/if}
  </main>

  <!-- Footer -->
  <footer class="fixed bottom-0 left-0 right-0 px-3 py-1.5 bg-ds-bg/95 backdrop-blur border-t border-ds-line">
    <div class="flex items-center justify-between text-[9px] text-ds-inactive">
      <div class="flex items-center gap-2">
        <span>Session: {session.panel?.session ?? '—'}</span>
        {#if selectedHarness}
          <span class="px-1.5 py-0.5 rounded bg-ds-accent/10 text-ds-accent capitalize">{selectedHarness}</span>
        {/if}
      </div>
      {#if session.lastUpdate}
        <span>Updated {session.lastUpdate.toLocaleTimeString()}</span>
      {/if}
    </div>
  </footer>
</div>
