// Shared: Animation utilities using Motion library
// Svelte actions for declarative animations

import { animate, stagger } from 'motion';

/**
 * Svelte action: Staggered fade-in from bottom
 */
export function fadeInStagger(node: HTMLElement, selector = '.animate-item') {
  const items = node.querySelectorAll(selector);
  if (items.length > 0) {
    animate(items, { opacity: [0, 1], y: [16, 0] }, { delay: stagger(0.04), duration: 0.25 });
  }
  return { destroy() {} };
}

/**
 * Svelte action: Pulsing indicator for live states
 */
export function pulse(node: HTMLElement): { destroy: () => void } {
  const controls = animate(
    node,
    { scale: [1, 1.15, 1], opacity: [1, 0.7, 1] },
    { duration: 1.2, repeat: Infinity }
  );
  return { destroy() { controls.stop(); } };
}

/**
 * Svelte action: Animate width from 0 to percentage
 */
export function progressFill(node: HTMLElement, percent: number) {
  animate(node, { width: ['0%', `${Math.min(100, Math.max(0, percent))}%`] }, { duration: 0.6 });
  return { destroy() {} };
}

/**
 * Svelte action: Scale-in on mount
 */
export function scaleIn(node: HTMLElement) {
  animate(node, { scale: [0.95, 1], opacity: [0, 1] }, { duration: 0.2 });
  return { destroy() {} };
}

/**
 * Svelte action: Slide-in from right
 */
export function slideInRight(node: HTMLElement) {
  animate(node, { x: [20, 0], opacity: [0, 1] }, { duration: 0.3 });
  return { destroy() {} };
}

// ============================================
// Premium animations for elevated UI
// ============================================

/**
 * Svelte action: Subtle float animation for cards
 */
export function floatSubtle(node: HTMLElement): { destroy: () => void } {
  const controls = animate(
    node,
    { y: [0, -2, 0] },
    { duration: 3, repeat: Infinity }
  );
  return { destroy() { controls.stop(); } };
}

/**
 * Svelte action: Glow pulse for accent elements
 */
export function glowPulse(node: HTMLElement): { destroy: () => void } {
  const controls = animate(
    node,
    { boxShadow: [
      '0 0 0 0 rgba(34, 197, 94, 0)',
      '0 0 20px 2px rgba(34, 197, 94, 0.3)',
      '0 0 0 0 rgba(34, 197, 94, 0)'
    ]},
    { duration: 2, repeat: Infinity }
  );
  return { destroy() { controls.stop(); } };
}

/**
 * Svelte action: Count up animation for numbers
 */
export function countUp(node: HTMLElement, target: number) {
  const duration = 800;
  const start = performance.now();
  const initial = 0;
  
  function update() {
    const elapsed = performance.now() - start;
    const progress = Math.min(elapsed / duration, 1);
    // Ease out cubic
    const eased = 1 - Math.pow(1 - progress, 3);
    const current = Math.round(initial + (target - initial) * eased);
    node.textContent = current.toLocaleString();
    
    if (progress < 1) {
      requestAnimationFrame(update);
    }
  }
  
  requestAnimationFrame(update);
  return { destroy() {} };
}

/**
 * Svelte action: Shimmer loading effect
 */
export function shimmer(node: HTMLElement): { destroy: () => void } {
  node.style.background = 'linear-gradient(90deg, transparent, rgba(255,255,255,0.05), transparent)';
  node.style.backgroundSize = '200% 100%';
  
  const controls = animate(
    node,
    { backgroundPosition: ['200% 0', '-200% 0'] },
    { duration: 1.5, repeat: Infinity }
  );
  return { destroy() { controls.stop(); } };
}

/**
 * Svelte action: Smooth tab content transition
 */
export function tabTransition(node: HTMLElement) {
  animate(node, { opacity: [0, 1], y: [8, 0] }, { duration: 0.2 });
  return { destroy() {} };
}

/**
 * Svelte action: Card hover lift effect (via CSS, animation on mount)
 */
export function cardHover(node: HTMLElement) {
  node.style.transition = 'transform 0.2s ease, box-shadow 0.2s ease';
  
  const onEnter = () => {
    node.style.transform = 'translateY(-2px)';
    node.style.boxShadow = '0 8px 25px -5px rgba(0, 0, 0, 0.3)';
  };
  
  const onLeave = () => {
    node.style.transform = 'translateY(0)';
    node.style.boxShadow = 'none';
  };
  
  node.addEventListener('mouseenter', onEnter);
  node.addEventListener('mouseleave', onLeave);
  
  // Initial fade in
  animate(node, { opacity: [0, 1], y: [10, 0] }, { duration: 0.3 });
  
  return {
    destroy() {
      node.removeEventListener('mouseenter', onEnter);
      node.removeEventListener('mouseleave', onLeave);
    }
  };
}

/**
 * Svelte action: Breathing glow for live indicators
 */
export function breathe(node: HTMLElement): { destroy: () => void } {
  const controls = animate(
    node,
    { 
      opacity: [0.6, 1, 0.6],
      scale: [0.95, 1.05, 0.95]
    },
    { duration: 2, repeat: Infinity }
  );
  return { destroy() { controls.stop(); } };
}

/**
 * Svelte action: Donut chart arc animation
 */
export function animateArc(node: SVGCircleElement, params: { offset: number; circumference: number }) {
  animate(
    node,
    { strokeDashoffset: [params.circumference, params.offset] },
    { duration: 1, delay: 0.2 }
  );
  return { destroy() {} };
}
