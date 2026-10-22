<script lang="ts">
  // App Shell: Sessions-first layout
  // Sessions are the primary entity, agents appear inline when expanded
  import { onMount, onDestroy } from 'svelte';
  import { getSessionState } from './shared/state';
  import { fetchSessions } from './shared/data';
  import type { SessionSummary } from './shared/data';
  import { TokensSummary, CostSaved } from './features/tokens';
  import { CompactorStats } from './features/compactor';
  import { SessionsList } from './features/sessions';

  const session = getSessionState();
  let activeTab = $state<'sessions' | 'tokens' | 'compactor'>('sessions');
  
  // Sessions state
  let sessions = $state<SessionSummary[]>([]);
  let harnesses = $state<string[]>([]);
  let loadingSessions = $state(false);

  // Derive active harness count
  const activeHarnessCount = $derived(() => {
    const fiveMinutesAgo = new Date(Date.now() - 5 * 60 * 1000);
    const activeHarnesses = new Set<string>();
    for (const s of sessions) {
      if (new Date(s.last_seen) > fiveMinutesAgo) {
        activeHarnesses.add(s.harness);
      }
    }
    return activeHarnesses.size;
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

  onMount(() => {
    session.startAutoRefresh(3000);
    loadSessions();
    // Refresh sessions list every 10s
    const sessionsInterval = setInterval(loadSessions, 10000);
    return () => clearInterval(sessionsInterval);
  });
  onDestroy(() => session.stopAutoRefresh());
</script>

<div class="min-h-screen bg-ds-bg text-ds-body pb-10">
  <!-- Header -->
  <header class="sticky top-0 z-10 bg-ds-bg/95 backdrop-blur border-b border-ds-line px-3 py-2">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-2">
        <!-- Logo: Downshift mark -->
        <svg class="w-6 h-6" viewBox="0 0 200 220" fill="none">
          <path d="M28 22 H104 L168 86 V140 L137 171 V98 L94 55 H28 Z" fill="#F9FAFB"/>
          <path d="M28 80 H81 L120 119 V164 L94 190 V130 L74 110 H28 Z" fill="#F9FAFB"/>
          <path d="M28 138 H65 L103 176 L78 201 L55 178 H28 Z" fill="#22C55E"/>
        </svg>
        <span class="text-sm font-semibold text-ds-body tracking-tight">downshift</span>
      </div>
      
      <div class="flex items-center gap-2">
        {#if session.loading || loadingSessions}
          <div class="w-3 h-3 rounded-full border-2 border-ds-muted border-t-transparent animate-spin"></div>
        {:else if session.isConnected}
          <div class="w-2 h-2 rounded-full bg-ds-accent"></div>
        {:else}
          <div class="w-2 h-2 rounded-full bg-ds-warning"></div>
        {/if}
      </div>
    </div>
  </header>

  <!-- Navigation tabs -->
  <nav class="px-3 pt-2 pb-1 flex gap-1 border-b border-ds-line bg-ds-bg">
    <button
      onclick={() => { activeTab = 'sessions'; loadSessions(); }}
      class="px-3 py-1.5 text-xs rounded-md transition-colors flex items-center gap-1.5"
      class:bg-ds-surface={activeTab === 'sessions'}
      class:text-ds-body={activeTab === 'sessions'}
      class:text-ds-muted={activeTab !== 'sessions'}
    >
      Sessions
      {#if activeHarnessCount() > 0}
        <span class="px-1.5 py-0.5 text-[9px] rounded-full bg-ds-accent/20 text-ds-accent font-medium">
          {activeHarnessCount()} active
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
    {#if activeTab === 'sessions'}
      {#if loadingSessions && sessions.length === 0}
        <div class="flex items-center justify-center py-12">
          <div class="w-6 h-6 rounded-full border-2 border-ds-muted border-t-transparent animate-spin"></div>
        </div>
      {:else if sessions.length === 0}
        <div class="text-center py-12">
          <p class="text-sm text-ds-muted">No sessions yet</p>
          <p class="text-xs text-ds-inactive mt-1">Spawn a subagent to see it here</p>
        </div>
      {:else}
        <SessionsList 
          {sessions} 
          {harnesses} 
          currentSessionId={session.currentSessionId}
        />
      {/if}
    {:else if activeTab === 'tokens'}
      {#if session.error}
        <div class="bg-ds-surface border border-ds-warning/30 rounded-lg p-3 text-center">
          <p class="text-xs text-ds-warning">⚠️ {session.error}</p>
          <button onclick={() => session.refresh()} class="mt-2 text-[10px] text-ds-muted hover:underline">
            Retry
          </button>
        </div>
      {:else if !session.panel}
        <div class="flex items-center justify-center py-12">
          <div class="w-6 h-6 rounded-full border-2 border-ds-muted border-t-transparent animate-spin"></div>
        </div>
      {:else}
        <div class="space-y-3">
          <TokensSummary tokens={session.panel.tokens} />
          {#if session.panel.cost_saved}
            <CostSaved cost={session.panel.cost_saved} />
          {/if}
        </div>
      {/if}
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
    {/if}
  </main>

  <!-- Footer -->
  <footer class="fixed bottom-0 left-0 right-0 bg-ds-bg/95 backdrop-blur border-t border-ds-line px-3 py-2">
    <div class="flex items-center justify-between text-[9px] text-ds-inactive">
      <span>
        {sessions.length} sessions · {harnesses.length} harnesses
      </span>
      {#if session.lastUpdate}
        <span>
          Updated {session.lastUpdate.toLocaleTimeString()}
        </span>
      {/if}
    </div>
  </footer>
</div>
