<script lang="ts">
  // Feature: Sessions-first UI
  // Sessions are the primary entity, agents appear inline when expanded
  import type { SessionSummary, Spawn } from '../../shared';
  import { fetchPanel } from '../../shared/data';
  import LiveAgent from '../agents/LiveAgent.svelte';
  import FinishedAgent from '../agents/FinishedAgent.svelte';
  import { ChevronDown, ChevronRight, Cpu, Zap, Clock } from 'lucide-svelte';

  interface Props {
    sessions: SessionSummary[];
    harnesses: string[];
    currentSessionId: string | null;
  }

  let { sessions, harnesses, currentSessionId }: Props = $props();

  // Expanded sessions state
  let expandedSessions = $state<Set<string>>(new Set());
  let sessionSpawns = $state<Map<string, Spawn[]>>(new Map());
  let loadingSession = $state<string | null>(null);
  let selectedHarness = $state<string | null>(null);

  // Filter sessions by harness
  const filteredSessions = $derived(
    selectedHarness 
      ? sessions.filter(s => s.harness === selectedHarness)
      : sessions
  );

  // Stats by harness
  const harnessStats = $derived(() => {
    const stats = new Map<string, { count: number; saved: number; active: number }>();
    for (const s of sessions) {
      const curr = stats.get(s.harness) || { count: 0, saved: 0, active: 0 };
      curr.count++;
      curr.saved += s.tokens_saved;
      // Consider active if last seen within 5 minutes
      const lastSeen = new Date(s.last_seen);
      const fiveMinutesAgo = new Date(Date.now() - 5 * 60 * 1000);
      if (lastSeen > fiveMinutesAgo) curr.active++;
      stats.set(s.harness, curr);
    }
    return stats;
  });

  async function toggleSession(sessionId: string) {
    if (expandedSessions.has(sessionId)) {
      expandedSessions.delete(sessionId);
      expandedSessions = new Set(expandedSessions);
    } else {
      // Load spawns if not already loaded
      if (!sessionSpawns.has(sessionId)) {
        loadingSession = sessionId;
        try {
          const panel = await fetchPanel(sessionId);
          sessionSpawns.set(sessionId, panel.spawns || []);
          sessionSpawns = new Map(sessionSpawns);
        } catch (e) {
          console.error('Failed to load session:', e);
        }
        loadingSession = null;
      }
      expandedSessions.add(sessionId);
      expandedSessions = new Set(expandedSessions);
    }
  }

  function formatTokens(n: number): string {
    if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
    if (n >= 1_000) return `${(n / 1_000).toFixed(0)}k`;
    return String(n);
  }

  function formatTime(iso: string): string {
    const d = new Date(iso);
    const now = new Date();
    const diffMs = now.getTime() - d.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    if (diffMins < 1) return 'now';
    if (diffMins < 60) return `${diffMins}m ago`;
    const diffHours = Math.floor(diffMins / 60);
    if (diffHours < 24) return `${diffHours}h ago`;
    return d.toLocaleDateString();
  }

  function isActive(session: SessionSummary): boolean {
    const lastSeen = new Date(session.last_seen);
    const fiveMinutesAgo = new Date(Date.now() - 5 * 60 * 1000);
    return lastSeen > fiveMinutesAgo;
  }

  function getSpawnsForSession(sessionId: string): { live: Spawn[]; finished: Spawn[] } {
    const spawns = sessionSpawns.get(sessionId) || [];
    const live = spawns.filter((s) => !s.tokens_label || s.tokens.work_tokens === 0);
    const finished = spawns.filter((s) => s.tokens_label && s.tokens.work_tokens > 0);
    return { live, finished };
  }
</script>

<div class="space-y-4">
  <!-- Harness filter chips -->
  <div class="flex flex-wrap gap-2">
    <button
      class="px-3 py-1.5 text-xs rounded-full transition-all {selectedHarness === null ? 'bg-ds-body text-ds-bg font-semibold' : 'bg-ds-surface text-ds-muted hover:bg-ds-surface/80'}"
      onclick={() => selectedHarness = null}
    >
      All
    </button>
    {#each harnesses as harness}
      {@const stats = harnessStats().get(harness)}
      <button
        class="px-3 py-1.5 text-xs rounded-full transition-all flex items-center gap-1.5 {selectedHarness === harness ? 'bg-ds-body text-ds-bg font-semibold' : 'bg-ds-surface text-ds-muted hover:bg-ds-surface/80'}"
        onclick={() => selectedHarness = (harness === selectedHarness ? null : harness)}
      >
        <span class="capitalize">{harness.replace('-', ' ')}</span>
        {#if stats && stats.active > 0}
          <span class="w-1.5 h-1.5 rounded-full bg-ds-accent"></span>
        {/if}
      </button>
    {/each}
  </div>

  <!-- Sessions list (expandable) -->
  <div class="space-y-2">
    {#each filteredSessions as session (session.session_id)}
      {@const isExpanded = expandedSessions.has(session.session_id)}
      {@const isLoading = loadingSession === session.session_id}
      {@const isCurrent = currentSessionId === session.session_id}
      {@const active = isActive(session)}
      {@const { live, finished } = getSpawnsForSession(session.session_id)}
      
      <div class="rounded-lg border transition-all {isCurrent ? 'border-ds-accent/50 bg-ds-surface' : 'border-ds-line bg-ds-surface/50'}">
        <!-- Session header (clickable) -->
        <button
          class="w-full p-3 text-left flex items-start gap-2"
          onclick={() => toggleSession(session.session_id)}
        >
          <!-- Expand/collapse icon -->
          <div class="mt-0.5 text-ds-muted">
            {#if isExpanded}
              <ChevronDown size={14} />
            {:else}
              <ChevronRight size={14} />
            {/if}
          </div>
          
          <!-- Session info -->
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2">
              <!-- Active indicator -->
              {#if active}
                <span class="w-2 h-2 rounded-full bg-ds-accent animate-pulse"></span>
              {:else}
                <span class="w-2 h-2 rounded-full bg-ds-inactive"></span>
              {/if}
              
              <!-- Session ID -->
              <span class="text-xs font-mono text-ds-muted">{session.session_id.slice(0, 8)}</span>
              
              <!-- Harness badge -->
              <span class="px-1.5 py-0.5 text-[8px] rounded bg-ds-bg text-ds-inactive uppercase tracking-wider">
                {session.harness}
              </span>
              
              <!-- Current badge -->
              {#if isCurrent}
                <span class="px-1.5 py-0.5 text-[8px] rounded bg-ds-accent/20 text-ds-accent uppercase tracking-wider">
                  current
                </span>
              {/if}
            </div>
            
            <!-- Stats row -->
            <div class="flex items-center gap-4 mt-1.5 text-[10px]">
              <span class="flex items-center gap-1 text-ds-muted">
                <Cpu size={10} />
                {session.spawn_count} agents
              </span>
              {#if session.switched > 0}
                <span class="flex items-center gap-1 text-ds-body">
                  <Zap size={10} />
                  {session.switched} switched
                </span>
              {/if}
              <span class="flex items-center gap-1 text-ds-inactive">
                <Clock size={10} />
                {formatTime(session.last_seen)}
              </span>
            </div>
          </div>
          
          <!-- Tokens saved (right aligned) -->
          {#if session.tokens_saved > 0}
            <div class="text-right">
              <span class="text-sm font-semibold text-ds-body">{formatTokens(session.tokens_saved)}</span>
              <span class="text-[9px] text-ds-muted block">saved</span>
            </div>
          {/if}
        </button>
        
        <!-- Expanded: agents list -->
        {#if isExpanded}
          <div class="border-t border-ds-line px-3 pb-3">
            {#if isLoading}
              <div class="flex items-center justify-center py-4">
                <div class="w-4 h-4 rounded-full border-2 border-ds-muted border-t-transparent animate-spin"></div>
              </div>
            {:else if live.length === 0 && finished.length === 0}
              <div class="py-4 text-center">
                <p class="text-[10px] text-ds-inactive">No active agents</p>
                <p class="text-[9px] text-ds-inactive/60 mt-0.5">Finished agents fade after ~1 minute</p>
              </div>
            {:else}
              <div class="space-y-3 pt-3">
                <!-- Live agents -->
                {#if live.length > 0}
                  <div>
                    <div class="flex items-center gap-2 mb-2">
                      <span class="w-1.5 h-1.5 rounded-full bg-ds-accent animate-pulse"></span>
                      <span class="text-[9px] uppercase tracking-wider text-ds-muted">Live ({live.length})</span>
                    </div>
                    <div class="space-y-1.5 pl-3">
                      {#each live as spawn (spawn.tool_use)}
                        <LiveAgent {spawn} />
                      {/each}
                    </div>
                  </div>
                {/if}
                
                <!-- Finished agents -->
                {#if finished.length > 0}
                  <div>
                    <div class="flex items-center gap-2 mb-2">
                      <span class="w-1.5 h-1.5 rounded-full bg-ds-inactive"></span>
                      <span class="text-[9px] uppercase tracking-wider text-ds-inactive">Completed ({finished.length})</span>
                    </div>
                    <div class="space-y-1.5 pl-3">
                      {#each finished as spawn (spawn.tool_use)}
                        <FinishedAgent {spawn} />
                      {/each}
                    </div>
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        {/if}
      </div>
    {/each}
  </div>

  <!-- Total stats -->
  {#if sessions.length > 0}
    {@const totalSaved = sessions.reduce((sum, s) => sum + s.tokens_saved, 0)}
    {@const totalSwitched = sessions.reduce((sum, s) => sum + s.switched, 0)}
    <div class="p-3 rounded-lg bg-ds-surface border border-ds-line">
      <div class="flex items-center justify-between">
        <div class="text-[10px] text-ds-muted">
          {sessions.length} sessions · {totalSwitched} total switched
        </div>
        <span class="text-sm font-bold text-ds-body">{formatTokens(totalSaved)} saved</span>
      </div>
    </div>
  {/if}
</div>
