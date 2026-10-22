/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{html,js,svelte,ts}', './index.html'],
  theme: {
    extend: {
      colors: {
        // Downshift brand tokens from docs/brand/downshift-brand-tokens.json
        ds: {
          // Core surfaces
          bg: '#0A0A0A',
          surface: '#1A1A1A',
          line: '#1F2329',
          border: '#2A2D31',
          
          // Text hierarchy
          text: '#FFFFFF',
          body: '#D6DAE2',
          muted: '#868A90',
          inactive: '#5C6168',
          
          // Accent (green - small accent only)
          accent: '#22C55E',
          'accent-deep': '#16A34A',
          
          // Status
          success: '#22C55E',
          error: '#F87171',
          warning: '#E0B34A',
        },
      },
      fontFamily: {
        mono: ['JetBrains Mono', 'SF Mono', 'Monaco', 'Consolas', 'monospace'],
      },
      animation: {
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
      },
    },
  },
  plugins: [],
};
