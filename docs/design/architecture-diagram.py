#!/usr/bin/env python3
"""
Downshift Architecture Diagram Generator
Generates a PNG diagram showing the full system with agent framework.
"""

import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
from matplotlib.patches import FancyBboxPatch, FancyArrowPatch
import numpy as np

# Set up the figure with dark theme
plt.style.use('dark_background')
fig, ax = plt.subplots(1, 1, figsize=(20, 14))
ax.set_xlim(0, 20)
ax.set_ylim(0, 14)
ax.set_aspect('equal')
ax.axis('off')

# Colors
COLORS = {
    'bg': '#0d1117',
    'orchestrator': '#58a6ff',
    'mcp': '#7ee787',
    'core': '#f0883e',
    'routing': '#a371f7',
    'compressor': '#79c0ff',
    'workers': '#ffa657',
    'agents': '#ff7b72',
    'memory': '#d2a8ff',
    'harness': '#8b949e',
    'text': '#c9d1d9',
    'border': '#30363d',
}

fig.patch.set_facecolor(COLORS['bg'])
ax.set_facecolor(COLORS['bg'])

def draw_box(ax, x, y, w, h, color, label, sublabel=None, alpha=0.8):
    """Draw a rounded box with label."""
    box = FancyBboxPatch((x, y), w, h, 
                          boxstyle="round,pad=0.02,rounding_size=0.15",
                          facecolor=color, edgecolor='white', 
                          linewidth=1.5, alpha=alpha)
    ax.add_patch(box)
    
    if sublabel:
        ax.text(x + w/2, y + h/2 + 0.15, label, ha='center', va='center',
                fontsize=10, fontweight='bold', color='white')
        ax.text(x + w/2, y + h/2 - 0.2, sublabel, ha='center', va='center',
                fontsize=7, color='white', alpha=0.8)
    else:
        ax.text(x + w/2, y + h/2, label, ha='center', va='center',
                fontsize=10, fontweight='bold', color='white')

def draw_section(ax, x, y, w, h, color, title):
    """Draw a section with border and title."""
    box = FancyBboxPatch((x, y), w, h,
                          boxstyle="round,pad=0.02,rounding_size=0.2",
                          facecolor=color, edgecolor='white',
                          linewidth=2, alpha=0.15)
    ax.add_patch(box)
    ax.text(x + 0.2, y + h - 0.3, title, fontsize=11, fontweight='bold',
            color=color, alpha=0.9)

def draw_arrow(ax, start, end, color='white', style='->', curved=False):
    """Draw an arrow between two points."""
    if curved:
        arrow = FancyArrowPatch(start, end, 
                                 connectionstyle="arc3,rad=0.2",
                                 arrowstyle=style, color=color, 
                                 linewidth=1.5, mutation_scale=15)
    else:
        arrow = FancyArrowPatch(start, end, 
                                 arrowstyle=style, color=color,
                                 linewidth=1.5, mutation_scale=15)
    ax.add_patch(arrow)

# Title
ax.text(10, 13.5, 'Downshift Architecture', fontsize=20, fontweight='bold',
        ha='center', color=COLORS['text'])
ax.text(10, 13.0, 'Token Optimization + Model Routing + Agent Orchestration', 
        fontsize=11, ha='center', color=COLORS['text'], alpha=0.7)

# ===== Layer 1: Harness (top) =====
draw_section(ax, 0.5, 11.0, 19, 1.7, COLORS['harness'], 'HARNESS LAYER')

harnesses = [
    ('Claude Code', 2.0), ('Cursor', 5.0), ('Codex', 8.0), 
    ('KiroCrew', 11.0), ('OpenCode', 14.0), ('Antigravity', 17.0)
]
for name, x in harnesses:
    draw_box(ax, x, 11.3, 2.2, 0.9, COLORS['harness'], name, alpha=0.6)

# ===== Layer 2: MCP Server =====
draw_section(ax, 0.5, 8.5, 19, 2.2, COLORS['mcp'], 'MCP SERVER (stdio/sse)')

# MCP Tools
mcp_tools = [
    ('delegate', 1.5, 'spawn worker'),
    ('collect', 4.0, 'read results'),
    ('cancel', 6.5, 'stop worker'),
    ('diff', 9.0, 'view changes'),
    ('steer', 11.5, 'mid-run msg'),
    ('send', 14.0, 'inject msg'),
    ('providers', 16.5, 'list models'),
]
for name, x, sub in mcp_tools:
    draw_box(ax, x, 8.85, 2.0, 0.8, COLORS['mcp'], name, sub)

# ===== Layer 3: Core Engine =====
draw_section(ax, 0.5, 5.5, 12.5, 2.7, COLORS['core'], 'CORE ENGINE')

# Core components
draw_box(ax, 1.0, 6.0, 2.5, 1.4, COLORS['routing'], 'Routing', 'model selection')
draw_box(ax, 4.0, 6.0, 2.5, 1.4, COLORS['compressor'], 'Compressor', 'output reduction')
draw_box(ax, 7.0, 6.0, 2.5, 1.4, COLORS['core'], 'Telemetry', 'token counting')
draw_box(ax, 10.0, 6.0, 2.5, 1.4, COLORS['core'], 'Policy', 'guardrails')

# ===== Layer 3b: Agent Framework (NEW) =====
draw_section(ax, 13.5, 5.5, 6, 2.7, COLORS['agents'], 'AGENT FRAMEWORK (NEW)')

draw_box(ax, 14.0, 6.6, 2.2, 1.0, COLORS['agents'], 'Coordinator', 'swarm logic')
draw_box(ax, 16.8, 6.6, 2.2, 1.0, COLORS['memory'], 'Memory', 'state.json')
draw_box(ax, 14.0, 5.8, 2.2, 0.6, COLORS['agents'], 'Topology')
draw_box(ax, 16.8, 5.8, 2.2, 0.6, COLORS['agents'], 'Registry')

# ===== Layer 4: Routing Detail =====
draw_section(ax, 0.5, 2.8, 7, 2.4, COLORS['routing'], 'ROUTING ENGINE')

routing_parts = [
    ('Policy\nParser', 1.0, 3.2),
    ('Tier\nHint', 2.8, 3.2),
    ('Model\nSelect', 4.6, 3.2),
]
for name, x, y in routing_parts:
    draw_box(ax, x, y, 1.5, 1.2, COLORS['routing'], name, alpha=0.7)

# ===== Layer 4b: Workers =====
draw_section(ax, 8.0, 2.8, 11.5, 2.4, COLORS['workers'], 'WORKER PROCESSES (isolated)')

# Workers with agents
workers = [
    ('Worker 1', 'coder', 8.5),
    ('Worker 2', 'tester', 11.0),
    ('Worker 3', 'reviewer', 13.5),
    ('Worker N', 'any agent', 16.5),
]
for name, agent, x in workers:
    draw_box(ax, x, 3.1, 2.0, 1.5, COLORS['workers'], name, f'agent: {agent}')

# ===== Layer 5: Agents Catalog =====
draw_section(ax, 0.5, 0.3, 19, 2.2, COLORS['agents'], 'AGENTS CATALOG (10 agents, 5 domains)')

# Agents by domain
domains = [
    ('CORE', ['coder', 'refactor', 'architect', 'researcher'], 1.0, COLORS['core']),
    ('QUALITY', ['tester', 'reviewer', 'debugger'], 6.5, COLORS['mcp']),
    ('SECURITY', ['security'], 11.0, COLORS['routing']),
    ('PERF', ['performance'], 13.5, COLORS['compressor']),
    ('SUPPORT', ['documenter'], 16.0, COLORS['memory']),
]

for domain, agents, x, color in domains:
    ax.text(x + 1.0, 2.1, domain, fontsize=8, fontweight='bold', color=color, ha='center')
    for i, agent in enumerate(agents):
        draw_box(ax, x + i * 1.3, 0.6, 1.2, 0.8, color, agent, alpha=0.6)

# ===== Arrows =====
# Harness to MCP
draw_arrow(ax, (10, 11.3), (10, 10.7), COLORS['orchestrator'])

# MCP to Core
draw_arrow(ax, (4, 8.85), (4, 8.2), COLORS['mcp'])
draw_arrow(ax, (8, 8.85), (8, 8.2), COLORS['mcp'])

# Core to Routing
draw_arrow(ax, (2.25, 6.0), (2.25, 5.2), COLORS['routing'])

# Core to Workers
draw_arrow(ax, (8.5, 6.0), (9.5, 5.2), COLORS['workers'])

# Agent Framework to Memory
draw_arrow(ax, (15.1, 6.6), (15.1, 5.2), COLORS['agents'], curved=True)

# Coordinator to Workers
draw_arrow(ax, (15.1, 6.0), (12.5, 4.6), COLORS['agents'], curved=True)

# Workers to Agents
draw_arrow(ax, (9.5, 3.1), (3, 2.5), COLORS['workers'], curved=True)
draw_arrow(ax, (12, 3.1), (8, 2.5), COLORS['workers'], curved=True)

# ===== Legend =====
legend_items = [
    ('Orchestrator Session', COLORS['orchestrator']),
    ('MCP Protocol', COLORS['mcp']),
    ('Core Engine', COLORS['core']),
    ('Routing', COLORS['routing']),
    ('Agent Framework', COLORS['agents']),
    ('Workers', COLORS['workers']),
]

for i, (label, color) in enumerate(legend_items):
    y_pos = 0.2 - i * 0.25
    ax.add_patch(plt.Rectangle((18.5, 12.8 + y_pos), 0.3, 0.15, 
                                facecolor=color, edgecolor='white'))
    ax.text(18.9, 12.85 + y_pos, label, fontsize=7, color=COLORS['text'], va='center')

# ===== Data Flow Labels =====
ax.text(10.5, 10.9, 'MCP calls', fontsize=7, color=COLORS['text'], alpha=0.7, style='italic')
ax.text(3.5, 8.0, 'delegate()', fontsize=7, color=COLORS['text'], alpha=0.7, style='italic')
ax.text(14.5, 5.0, 'state.json', fontsize=7, color=COLORS['text'], alpha=0.7, style='italic')

# ===== Save =====
output_path = '/Users/tiago.boas/Github/downshift/docs/design/downshift-architecture.png'
plt.savefig(output_path, dpi=150, bbox_inches='tight', 
            facecolor=COLORS['bg'], edgecolor='none')
plt.close()

print(f"Diagram saved to: {output_path}")
