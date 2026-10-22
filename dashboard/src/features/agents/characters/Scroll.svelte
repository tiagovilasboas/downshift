<script lang="ts">
  import { onMount } from 'svelte';

  interface Props {
    agentState: 'idle' | 'working' | 'done' | 'error';
    size?: number;
    color?: string;
    reducedMotion?: boolean;
  }

  let { agentState, size = 48, color = '#22C55E', reducedMotion = false }: Props = $props();

  let unrollAmount = $state(0);
  let penPosition = $state(0);

  onMount(() => {
    if (reducedMotion) return;

    const animInterval = setInterval(() => {
      if (agentState === 'working') {
        unrollAmount = 8 + Math.sin(Date.now() / 1000) * 4;
        penPosition = (Date.now() / 50) % 20;
      } else if (agentState === 'done') {
        unrollAmount = 12;
      } else {
        unrollAmount = 0;
      }
    }, 50);

    return () => clearInterval(animInterval);
  });
</script>

<svg width={size} height={size} viewBox="0 0 48 48" fill="none">
  <defs>
    <linearGradient id="scroll-gradient" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#D4A574"/>
      <stop offset="50%" stop-color="#E8D4B8"/>
      <stop offset="100%" stop-color="#D4A574"/>
    </linearGradient>
  </defs>

  {#if agentState !== 'idle'}
    <rect x="12" y={36 - unrollAmount} width="24" height={unrollAmount + 6} fill="url(#scroll-gradient)" rx="1"/>
    {#if agentState === 'working'}
      <g opacity="0.6">
        <line x1="16" y1={40 - unrollAmount + 4} x2={16 + penPosition} y2={40 - unrollAmount + 4} stroke={color} stroke-width="1"/>
      </g>
    {:else if agentState === 'done'}
      <g opacity="0.6">
        <line x1="16" y1="32" x2="32" y2="32" stroke={color} stroke-width="1"/>
        <line x1="16" y1="36" x2="32" y2="36" stroke={color} stroke-width="1"/>
      </g>
    {/if}
  {/if}

  <ellipse cx="24" cy="10" rx="16" ry="6" fill="url(#scroll-gradient)" stroke="#B8956E" stroke-width="1"/>
  <ellipse cx="24" cy="10" rx="14" ry="4" fill="#C4A882"/>
  <circle cx="8" cy="10" r="3" fill="#8B7355" stroke="#6B5344" stroke-width="1"/>
  <circle cx="40" cy="10" r="3" fill="#8B7355" stroke="#6B5344" stroke-width="1"/>

  {#if agentState === 'working' && !reducedMotion}
    <circle cx="36" cy="20" r="1.5" fill={color}>
      <animate attributeName="opacity" values="0;1;0" dur="1.5s" repeatCount="indefinite"/>
    </circle>
  {/if}

  {#if agentState === 'done'}
    <circle cx="40" cy="36" r="5" fill="#22C55E"/>
    <path d="M37.5 36 L39 37.5 L42.5 34" stroke="white" stroke-width="1.5" fill="none" stroke-linecap="round"/>
  {:else if agentState === 'error'}
    <circle cx="40" cy="36" r="5" fill="#EF4444"/>
    <path d="M38 34 L42 38 M42 34 L38 38" stroke="white" stroke-width="1.5" fill="none" stroke-linecap="round"/>
  {/if}
</svg>
