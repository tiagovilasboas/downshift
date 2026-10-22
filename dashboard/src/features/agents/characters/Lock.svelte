<script lang="ts">
  import { onMount } from 'svelte';

  interface Props {
    agentState: 'idle' | 'working' | 'done' | 'error';
    size?: number;
    color?: string;
    reducedMotion?: boolean;
  }

  let { agentState, size = 48, color = '#FBBF24', reducedMotion = false }: Props = $props();

  let shackleRotation = $state(0);
  let keyRotation = $state(0);

  onMount(() => {
    if (reducedMotion) return;

    const animInterval = setInterval(() => {
      if (agentState === 'working') {
        keyRotation = Math.sin(Date.now() / 200) * 30;
        shackleRotation = Math.sin(Date.now() / 150) * 5;
      } else if (agentState === 'done') {
        shackleRotation = -45;
        keyRotation = 0;
      } else {
        shackleRotation = 0;
        keyRotation = 0;
      }
    }, 50);

    return () => clearInterval(animInterval);
  });
</script>

<svg width={size} height={size} viewBox="0 0 48 48" fill="none">
  <defs>
    <filter id="lock-glow" x="-50%" y="-50%" width="200%" height="200%">
      <feGaussianBlur stdDeviation="2" result="blur"/>
      <feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge>
    </filter>
  </defs>

  <g style="transform-origin: 24px 18px; transform: rotate({shackleRotation}deg);">
    <path
      d="M16 20 L16 12 C16 6 20 4 24 4 C28 4 32 6 32 12 L32 20"
      fill="none"
      stroke={agentState === 'done' ? '#22C55E' : color}
      stroke-width="4"
      stroke-linecap="round"
    />
  </g>

  <g filter={agentState === 'working' ? 'url(#lock-glow)' : ''}>
    <rect x="12" y="20" width="24" height="20" rx="3" fill="#1A1A1A" stroke={agentState === 'done' ? '#22C55E' : agentState === 'error' ? '#EF4444' : color} stroke-width="2"/>
    <circle cx="24" cy="28" r="3" fill={agentState === 'done' ? '#22C55E' : color}/>
    <rect x="22" y="28" width="4" height="6" fill={agentState === 'done' ? '#22C55E' : color}/>
  </g>

  {#if agentState === 'working' && !reducedMotion}
    <g style="transform-origin: 24px 28px; transform: rotate({keyRotation}deg);">
      <circle cx="24" cy="28" r="2" fill={color}/>
      <rect x="26" y="27" width="12" height="2" fill={color}/>
      <rect x="34" y="27" width="2" height="4" fill={color}/>
    </g>
    <circle cx="8" cy="24" r="1.5" fill={color}>
      <animate attributeName="opacity" values="0;1;0" dur="1s" repeatCount="indefinite"/>
    </circle>
  {/if}

  {#if agentState === 'done'}
    <circle cx="40" cy="40" r="5" fill="#22C55E"/>
    <path d="M37.5 40 L39 41.5 L42.5 38" stroke="white" stroke-width="1.5" fill="none" stroke-linecap="round"/>
  {:else if agentState === 'error'}
    <circle cx="40" cy="40" r="5" fill="#EF4444"/>
    <path d="M38 38 L42 42 M42 38 L38 42" stroke="white" stroke-width="1.5" fill="none" stroke-linecap="round"/>
  {/if}
</svg>
