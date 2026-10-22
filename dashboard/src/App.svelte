<script lang="ts">
  // App Shell: Layout, navigation, composition
  // Features handle their own domain logic
  import { onMount, onDestroy } from 'svelte';
  import { getSessionState } from './shared/state';
  import { fetchSessions } from './shared/data';
  import type { SessionSummary } from './shared/data';
  import { AgentsList, HarnessList } from './features/agents';
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

  // Compute harness stats from sessions
  const harnessStats = $derived(() => {
    const stats = new Map<string, { sessions: number; active: number; tokensSaved: number }>();
    for (const s of sessions) {
      const curr = stats.get(s.harness) || { sessions: 0, active: 0, tokensSaved: 0 };
      curr.sessions++;
      curr.tokensSaved += s.tokens_saved;
      // Consider active if last seen within 5 minutes
      const lastSeen = new Date(s.last_seen);
      const fiveMinutesAgo = new Date(Date.now() - 5 * 60 * 1000);
      if (lastSeen > fiveMinutesAgo) curr.active++;
      stats.set(s.harness, curr);
    }
    return Array.from(stats.entries()).map(([harness, data]) => ({
      harness,
      ...data
    }));
  });

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
    activeTab = 'sessions'; // Navigate to sessions tab with filter
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
        <!-- Logo: Downshift mark from docs/brand/downshift-mark-darkbg.svg -->
        <svg class="w-6 h-6" viewBox="0 0 200 220" fill="none">
          <path d="M28 22 H104 L168 86 V140 L137 171 V98 L94 55 H28 Z" fill="#F9FAFB"/>
          <path d="M28 80 H81 L120 119 V164 L94 190 V130 L74 110 H28 Z" fill="#F9FAFB"/>
          <path d="M28 138 H65 L103 176 L78 201 L55 178 H28 Z" fill="#22C55E"/>
        </svg>
        <span class="text-sm font-semibold text-ds-body tracking-tight">downshift</span>
      </div>
      
      <div class="flex items-center gap-2">
        {#if session.loading}
          <div class="w-3 h-3 rounded-full border-2 border-ds-muted border-t-transparent animate-spin"></div>
        {:else if session.isConnected}
          <div class="w-2 h-2 rounded-full bg-ds-body"></div>
        {:else}
          <div class="w-2 h-2 rounded-full bg-ds-warning"></div>
        {/if}
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
        <span class="px-1.5 py-0.5 text-[9px] rounded-full bg-ds-surface border border-ds-line text-ds-body font-medium">
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
        <span class="px-1.5 py-0.5 text-[9px] rounded-full bg-ds-surface text-ds-muted font-mono">
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
        <button onclick={() => session.refresh()} class="mt-2 text-[10px] text-ds-muted hover:underline">
          Retry
        </button>
      </div>
    {:else if !session.panel && activeTab !== 'sessions'}
      <div class="flex items-center justify-center py-12">
        <div class="w-6 h-6 rounded-full border-2 border-ds-muted border-t-transparent animate-spin"></div>
      </div>
    {:else}
      {#if activeTab === 'agents'}
        <div class="space-y-6">
          <!-- Live agents first -->
          <AgentsList spawns={session.panel?.spawns ?? []} />
          
          <!-- Harness overview (clickable) -->
          {#if harnessStats().length > 0}
            <HarnessList stats={harnessStats()} onSelectHarness={handleSelectHarness} />
          {/if}
        </div>
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
            <p class="text-[10px] text-ds-inactive mt-1 font-mono">
              downshift compactor -- cmd
            </p>
          </div>
        {/if}
      {:else if activeTab === 'sessions'}
        {#if loadingSessions}
          <div class="flex items-center justify-center py-12">
            <div class="w-6 h-6 rounded-full border-2 border-ds-muted border-t-transparent animate-spin"></div>
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
    <div class="flex items-center justify-between text-[9px] text-ds-inactive font-mono">
      <div class="flex items-center gap-2">
        <span>localhost:1337</span>
        {#if selectedHarness}
          <span class="px-1.5 py-0.5 rounded bg-ds-surface border border-ds-line text-ds-muted">{selectedHarness}</span>
        {/if}
      </div>
      {#if session.lastUpdate}
        <span>{session.lastUpdate.toLocaleTimeString()}</span>
      {/if}
    </div>
  </footer>
</div>
