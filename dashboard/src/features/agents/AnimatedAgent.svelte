<script lang="ts">
  import { Ghost, Robot, Cursor, Scroll, Rocket, Lock } from './characters';
  import { onMount } from 'svelte';

  export type AgentState = 'idle' | 'working' | 'done' | 'error';

  interface Props {
    harness: string;
    agentState?: AgentState;
    size?: number;
  }

  let { harness, agentState = 'idle', size = 48 }: Props = $props();

  const characters: Record<string, typeof Ghost> = {
    'kirocrew': Ghost,
    'claude-code': Robot,
    'cursor': Cursor,
    'codex': Scroll,
    'antigravity': Rocket,
    'opencode': Lock,
  };

  const colors: Record<string, string> = {
    'kirocrew': '#06B6D4',
    'claude-code': '#F97316',
    'cursor': '#8B5CF6',
    'codex': '#22C55E',
    'antigravity': '#EF4444',
    'opencode': '#FBBF24',
  };

  const CharacterComponent = $derived(characters[harness] || Ghost);
  const accentColor = $derived(colors[harness] || '#22C55E');

  let prefersReducedMotion = $state(false);
  
  onMount(() => {
    prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  });

  const stateClass = $derived(
    agentState === 'idle' ? 'agent-idle' :
    agentState === 'working' ? 'agent-working' :
    agentState === 'done' ? 'agent-done' :
    'agent-error'
  );
</script>

<div class="animated-agent {stateClass}" style="--accent: {accentColor}; --size: {size}px;">
  <svelte:component
    this={CharacterComponent}
    {agentState}
    {size}
    color={accentColor}
    reducedMotion={prefersReducedMotion}
  />
</div>

<style>
  .animated-agent {
    width: var(--size);
    height: var(--size);
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.3s ease;
  }

  .agent-idle { opacity: 0.5; filter: grayscale(0.5); }
  .agent-working { opacity: 1; filter: drop-shadow(0 0 8px var(--accent)); }
  .agent-done { opacity: 1; filter: drop-shadow(0 0 4px #22C55E); }
  .agent-error { opacity: 1; filter: drop-shadow(0 0 4px #EF4444); animation: shake 0.5s ease-in-out; }

  @keyframes shake {
    0%, 100% { transform: translateX(0); }
    25% { transform: translateX(-4px); }
    75% { transform: translateX(4px); }
  }

  @media (prefers-reduced-motion: reduce) {
    .animated-agent { transition: none; }
    .agent-error { animation: none; }
  }
</style>
