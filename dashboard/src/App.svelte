<script lang="ts">
  // App Shell: Sessions-first layout
  // Sessions are the primary entity, agents appear inline when expanded
  import { onMount, onDestroy } from 'svelte';
  import { getSessionState } from './shared/state';
  import { fetchSessions } from './shared/data';
  import type { SessionSummary } from './shared/data';
  import { CostSaved } from './features/tokens';
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

  function setTab(tab: 'sessions' | 'tokens' | 'compactor') {
    activeTab = tab;
    const url = new URL(window.location.href);
    url.searchParams.set('tab', tab);
    history.pushState({ tab }, '', url);
    if (tab === 'sessions') loadSessions();
  }

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
    // Read initial tab from URL
    const params = new URLSearchParams(window.location.search);
    const tabParam = params.get('tab');
    if (tabParam === 'tokens' || tabParam === 'compactor' || tabParam === 'sessions') {
      activeTab = tabParam;
    }
    
    session.startAutoRefresh(3000);
    loadSessions();
    // Keep activeTab in sync with browser back/forward navigation
    const onPop = () => {
      const t = new URLSearchParams(window.location.search).get('tab');
      if (t === 'tokens' || t === 'compactor' || t === 'sessions') activeTab = t;
    };
    window.addEventListener('popstate', onPop);
    // Refresh sessions list every 10s
    const sessionsInterval = setInterval(loadSessions, 10000);
    return () => {
      clearInterval(sessionsInterval);
      window.removeEventListener('popstate', onPop);
    };
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
      type="button"
      data-testid="tab-sessions"
      onclick={() => setTab('sessions')}
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
      type="button"
      data-testid="tab-tokens"
      onclick={() => setTab('tokens')}
      class="px-3 py-1.5 text-xs rounded-md transition-colors"
      class:bg-ds-surface={activeTab === 'tokens'}
      class:text-ds-body={activeTab === 'tokens'}
      class:text-ds-muted={activeTab !== 'tokens'}
    >
      Tokens
    </button>
    <button
      type="button"
      data-testid="tab-compactor"
      onclick={() => setTab('compactor')}
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
      {#if sessions.length === 0}
        <div class="flex items-center justify-center py-12">
          <div class="w-6 h-6 rounded-full border-2 border-ds-muted border-t-transparent animate-spin"></div>
        </div>
      {:else}
        {@const totalSaved = sessions.reduce((sum, s) => sum + s.tokens_saved, 0)}
        {@const totalSwitched = sessions.reduce((sum, s) => sum + s.switched, 0)}
        {@const totalAgents = sessions.reduce((sum, s) => sum + s.spawn_count, 0)}
        <div class="space-y-3">
          <!-- Aggregated tokens from all sessions -->
          <div class="bg-ds-surface border border-ds-line rounded-lg p-4">
            <div class="flex items-center gap-4">
              <!-- Simple percentage visualization -->
              <div class="w-20 h-20 rounded-full border-4 border-ds-line flex items-center justify-center relative">
                <div class="absolute inset-0 rounded-full" 
                  style="background: conic-gradient(var(--ds-body) {Math.min(totalSwitched / Math.max(totalAgents, 1) * 100, 100)}%, var(--ds-line) 0)">
                </div>
                <div class="absolute inset-1 rounded-full bg-ds-surface flex items-center justify-center">
                  <span class="text-lg font-bold text-ds-body">{totalSwitched}</span>
                </div>
              </div>
              
              <div class="flex-1">
                <div class="text-2xl font-bold text-ds-body" data-testid="tokens-saved-total">{totalSaved >= 1_000_000 ? `${(totalSaved / 1_000_000).toFixed(1)}M` : totalSaved >= 1_000 ? `${Math.round(totalSaved / 1_000)}k` : totalSaved}</div>
                <div class="text-[10px] text-ds-muted">tokens saved across all sessions</div>
                <div class="mt-2 space-y-1 text-[9px]">
                  <div class="flex justify-between">
                    <span class="text-ds-inactive">Sessions</span>
                    <span class="text-ds-body">{sessions.length}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-ds-inactive">Agents spawned</span>
                    <span class="text-ds-body">{totalAgents}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-ds-inactive">Agents routed</span>
                    <span class="text-ds-body">{totalSwitched} of {totalAgents}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
          
          {#if session.panel?.cost_saved}
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
      <span data-testid="dashboard-footer">
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
