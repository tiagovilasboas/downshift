<script lang="ts">
  import { onMount } from 'svelte';

  interface Props {
    agentState: 'idle' | 'working' | 'done' | 'error';
    size?: number;
    color?: string;
    reducedMotion?: boolean;
  }

  let { agentState, size = 48, color = '#8B5CF6', reducedMotion = false }: Props = $props();

  let clickScale = $state(1);
  let sparkleOpacity = $state(0);

  onMount(() => {
    if (reducedMotion) return;

    const clickInterval = setInterval(() => {
      if (agentState === 'working') {
        clickScale = 0.9;
        sparkleOpacity = 1;
        setTimeout(() => { clickScale = 1; sparkleOpacity = 0; }, 150);
      }
    }, 800);

    return () => clearInterval(clickInterval);
  });
</script>

<svg width={size} height={size} viewBox="0 0 48 48" fill="none">
  <defs>
    <filter id="cursor-glow" x="-50%" y="-50%" width="200%" height="200%">
      <feGaussianBlur stdDeviation="2" result="blur"/>
      <feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge>
    </filter>
  </defs>

  <g style="transform-origin: 16px 8px; transform: scale({clickScale});" filter={agentState === 'working' ? 'url(#cursor-glow)' : ''}>
    <path
      d="M8 6 L8 36 L14 30 L20 42 L26 40 L20 28 L30 28 Z"
      fill={agentState === 'error' ? '#EF4444' : color}
      stroke="#1A1A1A"
      stroke-width="2"
      stroke-linejoin="round"
    />
  </g>

  {#if agentState === 'working' && !reducedMotion}
    <g opacity={sparkleOpacity}>
      <circle cx="32" cy="12" r="2" fill={color}/>
      <circle cx="36" cy="20" r="1.5" fill={color}/>
    </g>
    <circle cx="38" cy="16" r="1.5" fill={color}>
      <animate attributeName="opacity" values="0;1;0" dur="1.2s" repeatCount="indefinite"/>
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
