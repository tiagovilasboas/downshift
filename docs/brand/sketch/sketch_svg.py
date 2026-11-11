# Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0
"""Give a hand-written SVG diagram a symmetric pencil-sketch look.

Every colour, every text node and the layout stay exactly as written. Only the
way lines are drawn changes:

- each stroked shape gets a fine graphite jitter (small, even displacement, never a
  lopsided wobble), so boxes stay symmetric;
- each stroked shape is re-traced once by a faint, slightly offset copy in its
  own stroke colour, like a second pencil pass;
- each stroked rectangle gets regular diagonal hatching in its stroke colour;
- the full-size background gets a faint paper grain (light grain on a dark
  background, dark grain on a light one).

Fill-only shapes (logo marks, dots, arrowheads) and shapes inside <defs>,
markers, patterns, gradients and clip paths are left alone. A processed file
is marked with data-sketch="pencil" and is never processed twice.

Usage: python3 sketch_svg.py IN.svg [OUT.svg]   (OUT defaults to IN, in place)
"""
import re
import sys

MARK = 'data-sketch="pencil"'
SHAPE = re.compile(r'<(rect|path|line|circle|ellipse|polyline|polygon)\b([^>]*?)/>')
PROTECTED = re.compile(
    r'<(defs|marker|pattern|clipPath|mask|linearGradient|radialGradient|symbol)\b.*?</\1>', re.S)


def attr(attrs, name):
    m = re.search(r'(?<![\w-])' + name + r'="([^"]*)"', attrs)
    return m.group(1) if m else None


def css_rules(svg):
    """Map class name -> {property: value} from the file's <style> blocks."""
    rules = {}
    for block in re.findall(r'<style[^>]*>(.*?)</style>', svg, re.S):
        for names, body in re.findall(r'([^{}]+)\{([^}]*)\}', block):
            props = dict(
                (k.strip(), v.strip())
                for k, v in (p.split(':', 1) for p in body.split(';') if ':' in p))
            for name in re.findall(r'\.([\w-]+)', names):
                rules.setdefault(name, {}).update(props)
    return rules


def resolved(attrs, rules, prop):
    """A presentation value from the element's own style, attribute or class."""
    style = attr(attrs, 'style') or ''
    m = re.search(r'(?:^|;)\s*' + prop + r'\s*:\s*([^;]+)', style)
    if m:
        return m.group(1).strip()
    own = attr(attrs, prop)
    if own is not None:
        return own
    for cls in (attr(attrs, 'class') or '').split():
        if prop in rules.get(cls, {}):
            return rules[cls][prop]
    return None


def luminance(color):
    m = re.fullmatch(r'#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})', color or '')
    if not m:
        return 0.0  # gradients and unknowns: the diagrams here are dark
    h = m.group(1)
    if len(h) == 3:
        h = ''.join(c * 2 for c in h)
    r, g, b = (int(h[i:i + 2], 16) / 255 for i in (0, 2, 4))
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def without(attrs, *names):
    for n in names:
        attrs = re.sub(r'\s(?<![\w-])' + n + r'="[^"]*"', '', attrs)
    return attrs


class Sketcher:
    def __init__(self, svg):
        self.svg = svg
        self.rules = css_rules(svg)
        box = re.search(r'viewBox="\s*[-\d.]+[\s,]+[-\d.]+[\s,]+([\d.]+)[\s,]+([\d.]+)', svg)
        self.w, self.h = (float(box.group(1)), float(box.group(2))) if box else (0, 0)
        self.hatches = {}
        self.background_seen = False
        self.grain_ink = None

    def is_background(self, tag, attrs):
        if self.background_seen or tag != 'rect':
            return False
        if resolved(attrs, self.rules, 'stroke') not in (None, 'none'):
            return False
        width, height = attr(attrs, 'width'), attr(attrs, 'height')
        full = lambda v, full_size: v == '100%' or (v and _num(v) >= full_size - 1)
        return bool(full(width, self.w) and full(height, self.h))

    def hatch(self, color):
        key = 'dsk-h' + re.sub(r'[^\w]', '', color)
        self.hatches[key] = (
            f'<pattern id="{key}" width="9" height="9" patternUnits="userSpaceOnUse" '
            f'patternTransform="rotate(-38)"><line x1="0" y1="0" x2="0" y2="9" '
            f'stroke="{color}" stroke-width="1.1" opacity="0.22"/></pattern>')
        return f'url(#{key})'

    def shape(self, m):
        tag, attrs = m.group(1), m.group(2)
        el = m.group(0)
        if self.is_background(tag, attrs):
            self.background_seen = True
            fill = resolved(attrs, self.rules, 'fill')
            self.grain_ink = '0' if luminance(fill) > 0.5 else '1'
            body = without(attrs, 'fill', 'class', 'style', 'filter')
            return el + f'<{tag}{body} fill="#000" filter="url(#dsk-grain)"/>'
        stroke = resolved(attrs, self.rules, 'stroke')
        if not stroke or stroke == 'none' or stroke.startswith('url('):
            return el  # fill-only shapes (logo marks, dots, arrowheads) stay exact
        out = f'<g filter="url(#dsk-p1)">{el}</g>'
        ghost = without(attrs, 'fill', 'stroke-width', 'marker-end', 'marker-start', 'id')
        out += (f'<g filter="url(#dsk-p2)" opacity="0.55" transform="translate(1.4 -1.1)">'
                f'<{tag}{ghost} style="fill:none;stroke:{stroke};stroke-width:1.1;'
                f'marker-start:none;marker-end:none"/></g>')
        area = _num(attr(attrs, 'width') or '0') * _num(attr(attrs, 'height') or '0')
        if tag == 'rect' and area < 0.4 * self.w * self.h:  # no hatch over a whole card
            body = without(attrs, 'fill', 'stroke', 'stroke-width', 'stroke-dasharray',
                           'class', 'style', 'filter', 'id')
            out += f'<{tag}{body} fill="{self.hatch(stroke)}"/>'
        return out

    def run(self):
        svg = self.svg
        if MARK in svg:
            return svg
        # Shapes inside protected blocks are swapped out, then restored.
        kept = []

        def keep(m):
            kept.append(m.group(0))
            return f'\x00{len(kept) - 1}\x00'
        body = PROTECTED.sub(keep, svg)
        body = SHAPE.sub(self.shape, body)
        body = re.sub(r'\x00(\d+)\x00', lambda m: kept[int(m.group(1))], body)
        return re.sub(r'<svg\b([^>]*)>', lambda m: f'<svg{m.group(1)} {MARK}>' + self.defs(),
                      body, count=1)

    def defs(self):
        pad = 40
        region = (f'filterUnits="userSpaceOnUse" x="-{pad}" y="-{pad}" '
                  f'width="{self.w + 2 * pad:g}" height="{self.h + 2 * pad:g}"')
        ink = self.grain_ink or '1'
        jitter = ('<filter id="dsk-p{n}" {r}><feTurbulence type="fractalNoise" '
                  'baseFrequency="{f}" numOctaves="1" seed="{s}" result="n"/>'
                  '<feDisplacementMap in="SourceGraphic" in2="n" scale="{k}" '
                  'xChannelSelector="{x}" yChannelSelector="{y}"/></filter>')
        return ('<defs>'
                + jitter.format(n=1, r=region, f='0.09', s=3, k='2.2', x='R', y='G')
                + jitter.format(n=2, r=region, f='0.07', s=11, k='2.6', x='G', y='R')
                + '<filter id="dsk-grain"><feTurbulence type="fractalNoise" baseFrequency="0.8" '
                  'numOctaves="2" seed="5"/><feColorMatrix values="0 0 0 0 {i}  0 0 0 0 {i}  '
                  '0 0 0 0 {i}  0 0 0 0.06 0"/><feComposite in2="SourceGraphic" operator="in"/>'
                  '</filter>'.format(i=ink)
                + ''.join(self.hatches.values()) + '</defs>')


def _num(v):
    try:
        return float(re.sub(r'[a-z%]+$', '', v))
    except ValueError:
        return 0.0


def main(argv):
    if len(argv) not in (2, 3):
        sys.exit(__doc__)
    src = argv[1]
    dst = argv[2] if len(argv) == 3 else src
    with open(src, encoding='utf-8') as f:
        svg = f.read()
    with open(dst, 'w', encoding='utf-8') as f:
        f.write(Sketcher(svg).run())


if __name__ == '__main__':
    main(sys.argv)
