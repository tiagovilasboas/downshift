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
