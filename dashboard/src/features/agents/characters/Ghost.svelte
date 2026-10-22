<script lang="ts">
  import { onMount } from 'svelte';

  interface Props {
    agentState: 'idle' | 'working' | 'done' | 'error';
    size?: number;
    color?: string;
    reducedMotion?: boolean;
  }

  let { agentState, size = 48, color = '#06B6D4', reducedMotion = false }: Props = $props();

  let floatY = $state(0);
  let glowOpacity = $state(0.3);
  let eyeBlink = $state(false);

  onMount(() => {
    if (reducedMotion) return;

    const floatInterval = setInterval(() => {
      if (agentState === 'working') {
        floatY = Math.sin(Date.now() / 300) * 3;
        glowOpacity = 0.3 + Math.sin(Date.now() / 500) * 0.2;
      } else {
        floatY = 0;
        glowOpacity = 0.3;
      }
    }, 50);

    const blinkInterval = setInterval(() => {
      if (agentState === 'working') {
        eyeBlink = true;
        setTimeout(() => eyeBlink = false, 150);
      }
    }, 3000);

    return () => {
      clearInterval(floatInterval);
      clearInterval(blinkInterval);
    };
  });

  const eyeHeight = $derived(eyeBlink ? 1 : 4);
</script>

<svg 
  width={size} 
  height={size} 
  viewBox="0 0 48 48" 
  fill="none"
  style="transform: translateY({floatY}px);"
>
  {#if agentState === 'working'}
    <defs>
      <filter id="ghost-glow" x="-50%" y="-50%" width="200%" height="200%">
        <feGaussianBlur stdDeviation="2" result="blur"/>
        <feMerge>
          <feMergeNode in="blur"/>
          <feMergeNode in="SourceGraphic"/>
        </feMerge>
      </filter>
    </defs>
  {/if}

  <g filter={agentState === 'working' ? 'url(#ghost-glow)' : ''}>
    <path
      d="M24 6C14 6 8 14 8 24C8 32 8 42 8 42L12 38L16 42L20 38L24 42L28 38L32 42L36 38L40 42C40 42 40 32 40 24C40 14 34 6 24 6Z"
      fill={color}
      fill-opacity={agentState === 'done' ? 0.9 : agentState === 'error' ? 0.7 : 0.85}
    />

    <g>
      <ellipse cx="18" cy="22" rx="3" ry={eyeHeight} fill="#0A0A0A"/>
      <ellipse cx="30" cy="22" rx="3" ry={eyeHeight} fill="#0A0A0A"/>

      {#if agentState === 'done'}
        <path d="M18 30 Q24 36 30 30" stroke="#0A0A0A" stroke-width="2" fill="none" stroke-linecap="round"/>
      {:else if agentState === 'error'}
        <path d="M18 34 Q24 28 30 34" stroke="#0A0A0A" stroke-width="2" fill="none" stroke-linecap="round"/>
      {:else}
        <ellipse cx="24" cy="32" rx="4" ry="2" fill="#0A0A0A"/>
      {/if}
    </g>

    {#if agentState === 'working' && !reducedMotion}
      <circle cx="6" cy="12" r="1.5" fill={color} opacity={glowOpacity}>
        <animate attributeName="opacity" values="0.2;0.8;0.2" dur="1.5s" repeatCount="indefinite"/>
      </circle>
      <circle cx="42" cy="18" r="1" fill={color} opacity={glowOpacity}>
        <animate attributeName="opacity" values="0.8;0.2;0.8" dur="1.2s" repeatCount="indefinite"/>
      </circle>
    {/if}

    {#if agentState === 'done'}
      <circle cx="38" cy="10" r="6" fill="#22C55E"/>
      <path d="M35 10 L37 12 L41 8" stroke="white" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
    {/if}

    {#if agentState === 'error'}
      <circle cx="38" cy="10" r="6" fill="#EF4444"/>
      <path d="M35 7 L41 13 M41 7 L35 13" stroke="white" stroke-width="2" fill="none" stroke-linecap="round"/>
    {/if}
  </g>
</svg>
