<script lang="ts">
  // Feature: Sessions - Session and harness selector
  import type { SessionSummary } from '../../shared';
  import { fadeInStagger } from '../../shared';

  interface Props {
    sessions: SessionSummary[];
    harnesses: string[];
    selectedSession: string | null;
    selectedHarness: string | null;
    onSelectSession: (id: string | null) => void;
    onSelectHarness: (harness: string | null) => void;
  }

  let { sessions, harnesses, selectedSession, selectedHarness, onSelectSession, onSelectHarness }: Props = $props();

  // Filter sessions by harness
  const filteredSessions = $derived(
    selectedHarness 
      ? sessions.filter(s => s.harness === selectedHarness)
      : sessions
  );

  // Stats by harness
  const harnessStats = $derived(() => {
    const stats = new Map<string, { count: number; saved: number }>();
    for (const s of sessions) {
      const curr = stats.get(s.harness) || { count: 0, saved: 0 };
      curr.count++;
      curr.saved += s.tokens_saved;
      stats.set(s.harness, curr);
    }
    return stats;
  });

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
</script>

<div class="space-y-4">
  <!-- Harness filter chips -->
  <div class="flex flex-wrap gap-2">
    <button
      class="px-3 py-1.5 text-xs rounded-full transition-all {selectedHarness === null ? 'bg-ds-body text-ds-bg font-semibold' : 'bg-ds-surface text-ds-muted hover:bg-ds-surface/80'}"
      onclick={() => onSelectHarness(null)}
    >
      All harnesses
    </button>
    {#each harnesses as harness}
      {@const stats = harnessStats().get(harness)}
      <button
        class="px-3 py-1.5 text-xs rounded-full transition-all flex items-center gap-1.5 {selectedHarness === harness ? 'bg-ds-body text-ds-bg font-semibold' : 'bg-ds-surface text-ds-muted hover:bg-ds-surface/80'}"
        onclick={() => onSelectHarness(harness === selectedHarness ? null : harness)}
      >
        <span class="capitalize">{harness.replace('-', ' ')}</span>
        {#if stats}
          <span class="opacity-60">({stats.count})</span>
        {/if}
      </button>
    {/each}
  </div>

  <!-- Session list -->
  <div class="space-y-2 max-h-60 overflow-y-auto" use:fadeInStagger={'.session-item'}>
    {#each filteredSessions as session (session.session_id)}
      {@const isSelected = selectedSession === session.session_id}
      <button
        class="session-item w-full p-3 rounded-lg text-left transition-all border {isSelected ? 'bg-ds-surface border-ds-muted' : 'bg-ds-surface/50 border-transparent hover:bg-ds-surface'}"
        onclick={() => onSelectSession(isSelected ? null : session.session_id)}
      >
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <span class="text-xs font-mono text-ds-inactive">{session.session_id.slice(0, 8)}</span>
            <span class="px-1.5 py-0.5 text-[8px] rounded bg-ds-surface text-ds-muted uppercase tracking-wider">{session.harness}</span>
          </div>
          <span class="text-[10px] text-ds-inactive">{formatTime(session.last_seen)}</span>
        </div>
        <div class="flex items-center justify-between mt-1.5">
          <div class="flex items-center gap-3 text-[10px]">
            <span class="text-ds-body">{session.switched} switched</span>
            <span class="text-ds-muted">{session.held} held</span>
          </div>
          {#if session.tokens_saved > 0}
            <span class="text-xs font-semibold text-ds-body">{formatTokens(session.tokens_saved)} saved</span>
          {/if}
        </div>
      </button>
    {/each}
  </div>

  <!-- Summary for selected harness -->
  {#if selectedHarness}
    {@const stats = harnessStats().get(selectedHarness)}
    {#if stats}
      <div class="p-3 rounded-lg bg-ds-surface border border-ds-line">
        <div class="flex items-center justify-between">
          <span class="text-xs text-ds-muted capitalize">{selectedHarness.replace('-', ' ')} total</span>
          <span class="text-sm font-bold text-ds-body">{formatTokens(stats.saved)} saved</span>
        </div>
      </div>
    {/if}
  {/if}
</div>
