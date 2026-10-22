<script lang="ts">
  import { onMount } from 'svelte';

  interface Props {
    agentState: 'idle' | 'working' | 'done' | 'error';
    size?: number;
    color?: string;
    reducedMotion?: boolean;
  }

  let { agentState, size = 48, color = '#EF4444', reducedMotion = false }: Props = $props();

  let shake = $state(0);
  let flameHeight = $state(8);

  onMount(() => {
    if (reducedMotion) return;

    const animInterval = setInterval(() => {
      if (agentState === 'working') {
        shake = (Math.random() - 0.5) * 2;
        flameHeight = 8 + Math.random() * 6;
      } else {
        shake = 0;
        flameHeight = agentState === 'done' ? 4 : 0;
      }
    }, 50);

    return () => clearInterval(animInterval);
  });
</script>

<svg width={size} height={size} viewBox="0 0 48 48" fill="none" style="transform: translateX({shake}px);">
  <defs>
    <linearGradient id="flame-gradient" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#FBBF24"/>
      <stop offset="50%" stop-color="#F97316"/>
      <stop offset="100%" stop-color="#EF4444"/>
    </linearGradient>
  </defs>

  {#if agentState === 'working' || agentState === 'done'}
    <path d="M20 40 Q24 {40 + flameHeight} 28 40" fill="url(#flame-gradient)">
      {#if agentState === 'working' && !reducedMotion}
        <animate attributeName="d" values="M20 40 Q24 48 28 40; M20 40 Q24 52 28 40; M20 40 Q24 48 28 40" dur="0.2s" repeatCount="indefinite"/>
      {/if}
    </path>
    <path d="M22 40 Q24 {40 + flameHeight * 0.6} 26 40" fill="#FBBF24"/>
  {/if}

  <path d="M24 4 C16 12 16 28 18 36 L30 36 C32 28 32 12 24 4 Z" fill="#E5E7EB" stroke={color} stroke-width="2"/>
  <path d="M24 4 C20 8 20 12 22 16 L26 16 C28 12 28 8 24 4 Z" fill={color}/>
  <circle cx="24" cy="22" r="4" fill="#0EA5E9" stroke="#1A1A1A" stroke-width="1"/>
  <circle cx="23" cy="21" r="1.5" fill="white" opacity="0.6"/>
  <path d="M18 32 L12 40 L18 38 Z" fill={color}/>
  <path d="M30 32 L36 40 L30 38 Z" fill={color}/>

  {#if agentState === 'working' && !reducedMotion}
    <circle cx="8" cy="12" r="1" fill="white">
      <animate attributeName="opacity" values="0;1;0" dur="1s" repeatCount="indefinite"/>
    </circle>
    <circle cx="40" cy="8" r="1.5" fill="white">
      <animate attributeName="opacity" values="0.5;1;0.5" dur="1.5s" repeatCount="indefinite"/>
    </circle>
  {/if}

  {#if agentState === 'done'}
    <circle cx="40" cy="8" r="5" fill="#22C55E"/>
    <path d="M37.5 8 L39 9.5 L42.5 6" stroke="white" stroke-width="1.5" fill="none" stroke-linecap="round"/>
  {:else if agentState === 'error'}
    <circle cx="40" cy="8" r="5" fill="#EF4444"/>
    <path d="M38 6 L42 10 M42 6 L38 10" stroke="white" stroke-width="1.5" fill="none" stroke-linecap="round"/>
  {/if}
</svg>
