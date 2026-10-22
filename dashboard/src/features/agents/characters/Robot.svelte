<script lang="ts">
  import { onMount } from 'svelte';

  interface Props {
    agentState: 'idle' | 'working' | 'done' | 'error';
    size?: number;
    color?: string;
    reducedMotion?: boolean;
  }

  let { agentState, size = 48, color = '#F97316', reducedMotion = false }: Props = $props();

  let antennaRotation = $state(0);
  let eyeGlow = $state(false);

  onMount(() => {
    if (reducedMotion) return;

    const antennaInterval = setInterval(() => {
      if (agentState === 'working') {
        antennaRotation = Math.sin(Date.now() / 200) * 15;
      } else {
        antennaRotation = 0;
      }
    }, 50);

    const eyeInterval = setInterval(() => {
      if (agentState === 'working') {
        eyeGlow = !eyeGlow;
      } else {
        eyeGlow = agentState === 'done';
      }
    }, 500);

    return () => {
      clearInterval(antennaInterval);
      clearInterval(eyeInterval);
    };
  });
</script>

<svg width={size} height={size} viewBox="0 0 48 48" fill="none">
  <defs>
    <filter id="robot-glow" x="-50%" y="-50%" width="200%" height="200%">
      <feGaussianBlur stdDeviation="1.5" result="blur"/>
      <feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge>
    </filter>
  </defs>

  <g style="transform-origin: 24px 8px; transform: rotate({antennaRotation}deg);">
    <line x1="24" y1="8" x2="24" y2="2" stroke={color} stroke-width="2"/>
    <circle cx="24" cy="2" r="2" fill={color} filter={agentState === 'working' ? 'url(#robot-glow)' : ''}>
      {#if agentState === 'working' && !reducedMotion}
        <animate attributeName="r" values="2;3;2" dur="0.5s" repeatCount="indefinite"/>
      {/if}
    </circle>
  </g>

  <rect x="10" y="8" width="28" height="24" rx="4" fill="#1A1A1A" stroke={color} stroke-width="2"/>
  <rect x="14" y="12" width="20" height="16" rx="2" fill="#2A2A2A"/>

  <g filter={eyeGlow ? 'url(#robot-glow)' : ''}>
    <rect x="16" y="16" width="6" height="4" rx="1" fill={agentState === 'error' ? '#EF4444' : color} opacity={eyeGlow ? 1 : 0.6}/>
    <rect x="26" y="16" width="6" height="4" rx="1" fill={agentState === 'error' ? '#EF4444' : color} opacity={eyeGlow ? 1 : 0.6}/>
  </g>

  {#if agentState === 'done'}
    <path d="M18 24 Q24 28 30 24" stroke={color} stroke-width="2" fill="none"/>
  {:else if agentState === 'error'}
    <path d="M16 24 L20 26 L24 22 L28 26 L32 24" stroke="#EF4444" stroke-width="2" fill="none"/>
  {:else}
    <g>
      <circle cx="20" cy="24" r="1.5" fill={color} opacity={eyeGlow ? 1 : 0.4}/>
      <circle cx="24" cy="24" r="1.5" fill={color} opacity={eyeGlow ? 0.4 : 1}/>
      <circle cx="28" cy="24" r="1.5" fill={color} opacity={eyeGlow ? 1 : 0.4}/>
    </g>
  {/if}

  <rect x="14" y="34" width="20" height="10" rx="2" fill="#1A1A1A" stroke={color} stroke-width="2"/>
  <rect x="22" y="37" width="4" height="4" rx="1" fill={agentState === 'working' ? color : '#333'}>
    {#if agentState === 'working' && !reducedMotion}
      <animate attributeName="opacity" values="0.5;1;0.5" dur="1s" repeatCount="indefinite"/>
    {/if}
  </rect>

  {#if agentState === 'done'}
    <circle cx="38" cy="10" r="5" fill="#22C55E"/>
    <path d="M35.5 10 L37 11.5 L40.5 8" stroke="white" stroke-width="1.5" fill="none" stroke-linecap="round"/>
  {:else if agentState === 'error'}
    <circle cx="38" cy="10" r="5" fill="#EF4444"/>
    <path d="M36 8 L40 12 M40 8 L36 12" stroke="white" stroke-width="1.5" fill="none" stroke-linecap="round"/>
  {/if}
</svg>
