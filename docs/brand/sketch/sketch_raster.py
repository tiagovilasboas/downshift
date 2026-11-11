#!/usr/bin/env python3
# Copyright (c) 2026 Tiago Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
"""Give a raster illustration a pencil-sketch finish.

Usage: sketch_raster.py INPUT.png OUTPUT.png [--svg-out FILE]

Run it from the ORIGINAL image: feeding an already sketched file compounds the
effect. The art is wrapped in an SVG and rendered with rsvg-convert, so the
only requirements are python3 and rsvg-convert. Colours, composition and
proportions are kept; the pencil is added on top of the art:

  1. graphite contour lines found with an edge filter (feConvolveMatrix),
     coloured from the source (darker on light areas, lighter on dark areas),
     with a fine symmetric jitter and one faint re-traced offset copy;
  2. regular 45 degree hatching in shaded areas, masked by the art's tones;
  3. a faint, mean-neutral paper grain.
"""
import base64
import struct
import subprocess
import sys
import zlib

# Tunable parameters (reported in the delivery notes).
P = {
    "pre_blur": 0.5,       # px, edge input smoothing (the art itself is not blurred)
    "edge_gain": 4.0,      # contour strength
    "edge_cut": 0.10,      # contour threshold (alpha offset)
    "dark_line": 0.75,     # contour colour on light areas = source * this
    "light_line": 1.7,     # contour colour on dark areas = source * this + lift
    "light_lift": 0.05,
    "line_opacity": 0.8,
    "jitter_freq": 0.045,  # feTurbulence base frequency of the jitter field
    "jitter_px": 1.6,      # displacement scale (symmetric around 0)
    "retrace_dx": 1.6,
    "retrace_dy": 1.2,
    "retrace_opacity": 0.32,
    "hatch_pitch": 8.0,    # px between hatch lines (along the axis)
    "hatch_width": 1.1,
    "hatch_dark_opacity": 0.22,   # multiply black, shaded mid-tones
    "hatch_light_opacity": 0.2,   # screen, very dark areas
    "grain_freq": 0.6,
    "grain_opacity": 0.025,
}


def png_size(path):
    with open(path, "rb") as f:
        head = f.read(24)
    if head[:8] != b"\x89PNG\r\n\x1a\n":
        sys.exit("input is not a PNG: " + path)
    return struct.unpack(">II", head[16:24])


def table(fn, n=17):
    return " ".join("%.3f" % max(0.0, min(1.0, fn(i / (n - 1)))) for i in range(n))


def ramp(x, a, b):
    return max(0.0, min(1.0, (x - a) / (b - a)))


def build_svg(b64, w, h):
    p = dict(P)
    # Pencil geometry is tuned for a 1752 px wide image; narrower art gets a
    # finer pencil so small text stays legible (never above 1.25x).
    sc = max(0.55, min(1.25, w / 1752))
    for key in ("jitter_px", "retrace_dx", "retrace_dy", "hatch_pitch", "hatch_width"):
        p[key] = round(p[key] * sc, 3)
    # Tone gates (input: luminance, output: 0..1).
    dark_hatch = table(lambda l: min(ramp(l, 0.16, 0.28), 1 - ramp(l, 0.45, 0.70)))
    light_hatch = table(lambda l: min(ramp(l, 0.05, 0.09), 1 - ramp(l, 0.16, 0.26)))
    gate_dark_line = table(lambda l: ramp(l, 0.10, 0.30))      # lines on light areas
    gate_light_line = table(lambda l: 1 - ramp(l, 0.10, 0.30))  # lines on dark areas
    lum = "0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  .2126 .7152 .0722 0 0"
    gray = ".2126 .7152 .0722 0 0  .2126 .7152 .0722 0 0  .2126 .7152 .0722 0 0  0 0 0 0 1"
    k = "-1 -1 -1 -1 8 -1 -1 -1 -1"
    kn = "1 1 1 1 -8 1 1 1 1"
    dk, lk, lift = p["dark_line"], p["light_line"], p["light_lift"]
    j = p["jitter_px"]
    pitch = p["hatch_pitch"]
    img = '<image width="%d" height="%d" href="data:image/png;base64,%s"/>' % (w, h, b64)
    return f'''<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}">
<defs>
<filter id="pencil" x="0" y="0" width="{w}" height="{h}" filterUnits="userSpaceOnUse" color-interpolation-filters="sRGB">
  <feColorMatrix in="SourceGraphic" type="matrix" values="{lum}" result="lumA"/>
  <feGaussianBlur in="lumA" stdDeviation="{p['pre_blur']}" result="lumB"/>
  <feConvolveMatrix in="lumB" order="3" kernelMatrix="{k}" divisor="1" preserveAlpha="false" edgeMode="duplicate" result="ep"/>
  <feConvolveMatrix in="lumB" order="3" kernelMatrix="{kn}" divisor="1" preserveAlpha="false" edgeMode="duplicate" result="en"/>
  <feComposite in="ep" in2="en" operator="arithmetic" k1="0" k2="1" k3="1" k4="0" result="eabs"/>
  <feComponentTransfer in="eabs" result="edge"><feFuncA type="linear" slope="{p['edge_gain']}" intercept="-{p['edge_cut']}"/></feComponentTransfer>

  <feColorMatrix in="SourceGraphic" type="matrix" values="0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  .2126 .7152 .0722 0 0" result="lumG"/>
  <feComponentTransfer in="lumG" result="gDark"><feFuncA type="table" tableValues="{gate_dark_line}"/></feComponentTransfer>
  <feComponentTransfer in="lumG" result="gLight"><feFuncA type="table" tableValues="{gate_light_line}"/></feComponentTransfer>
  <feComposite in="edge" in2="gDark" operator="in" result="aDark"/>
  <feComposite in="edge" in2="gLight" operator="in" result="aLight"/>
  <feColorMatrix in="SourceGraphic" type="matrix" values="{dk} 0 0 0 0  0 {dk} 0 0 0  0 0 {dk} 0 0  0 0 0 0 1" result="cDark"/>
  <feColorMatrix in="SourceGraphic" type="matrix" values="{lk} 0 0 0 {lift}  0 {lk} 0 0 {lift}  0 0 {lk} 0 {lift}  0 0 0 0 1" result="cLight"/>
  <feComposite in="cDark" in2="aDark" operator="in" result="lDark"/>
  <feComposite in="cLight" in2="aLight" operator="in" result="lLight"/>
  <feMerge result="lines"><feMergeNode in="lDark"/><feMergeNode in="lLight"/></feMerge>

  <feTurbulence type="fractalNoise" baseFrequency="{p['jitter_freq']}" numOctaves="2" seed="7" result="n1"/>
  <feDisplacementMap in="lines" in2="n1" scale="{j}" xChannelSelector="R" yChannelSelector="G" result="main"/>
  <feOffset in="lines" dx="{p['retrace_dx']}" dy="{p['retrace_dy']}" result="off"/>
  <feTurbulence type="fractalNoise" baseFrequency="{p['jitter_freq']}" numOctaves="2" seed="19" result="n2"/>
  <feDisplacementMap in="off" in2="n2" scale="{j}" xChannelSelector="R" yChannelSelector="G" result="retr0"/>
  <feComponentTransfer in="retr0" result="retr"><feFuncA type="linear" slope="{p['retrace_opacity']}"/></feComponentTransfer>
  <feMerge result="pen"><feMergeNode in="retr"/><feMergeNode in="main"/></feMerge>
  <feComponentTransfer in="pen" result="penA"><feFuncA type="linear" slope="{p['line_opacity']}"/></feComponentTransfer>
  <feMerge><feMergeNode in="SourceGraphic"/><feMergeNode in="penA"/></feMerge>
</filter>

<filter id="toneDark" x="0" y="0" width="{w}" height="{h}" filterUnits="userSpaceOnUse" color-interpolation-filters="sRGB">
  <feColorMatrix in="SourceGraphic" type="matrix" values="{gray}" result="g"/>
  <feComponentTransfer in="g"><feFuncR type="table" tableValues="{dark_hatch}"/><feFuncG type="table" tableValues="{dark_hatch}"/><feFuncB type="table" tableValues="{dark_hatch}"/></feComponentTransfer>
</filter>
<filter id="toneLight" x="0" y="0" width="{w}" height="{h}" filterUnits="userSpaceOnUse" color-interpolation-filters="sRGB">
  <feColorMatrix in="SourceGraphic" type="matrix" values="{gray}" result="g"/>
  <feComponentTransfer in="g"><feFuncR type="table" tableValues="{light_hatch}"/><feFuncG type="table" tableValues="{light_hatch}"/><feFuncB type="table" tableValues="{light_hatch}"/></feComponentTransfer>
</filter>

<pattern id="hatch" width="{pitch}" height="{pitch}" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
  <rect width="{p['hatch_width']}" height="{pitch}" fill="#fff"/>
</pattern>

<mask id="mDark" maskUnits="userSpaceOnUse" x="0" y="0" width="{w}" height="{h}">
  <g filter="url(#toneDark)">{img}</g>
</mask>
<mask id="mLight" maskUnits="userSpaceOnUse" x="0" y="0" width="{w}" height="{h}">
  <g filter="url(#toneLight)">{img}</g>
</mask>

<filter id="grain" x="0" y="0" width="{w}" height="{h}" filterUnits="userSpaceOnUse" color-interpolation-filters="sRGB">
  <feTurbulence type="fractalNoise" baseFrequency="{p['grain_freq']}" numOctaves="2" seed="3" result="n"/>
  <feColorMatrix in="n" type="matrix" values="0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  3 0 0 0 -1.5" result="wh"/>
  <feColorMatrix in="n" type="matrix" values="0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  -3 0 0 0 1.5" result="bk"/>
  <feMerge><feMergeNode in="wh"/><feMergeNode in="bk"/></feMerge>
</filter>
</defs>
<g filter="url(#pencil)">{img}</g>
<rect width="{w}" height="{h}" fill="url(#hatch)" mask="url(#mDark)" style="mix-blend-mode:multiply" opacity="{p['hatch_dark_opacity']}"/>
<rect width="{w}" height="{h}" fill="url(#hatch)" mask="url(#mLight)" style="mix-blend-mode:screen" opacity="{p['hatch_light_opacity']}"/>
<rect width="{w}" height="{h}" filter="url(#grain)" opacity="{p['grain_opacity']}"/>
</svg>'''


def recompress(png):
    """Re-deflate the IDAT stream at level 9 (lossless, a few percent smaller)."""
    pos, idat, chunks = 8, b"", []
    while pos < len(png):
        n, = struct.unpack(">I", png[pos:pos + 4])
        typ, body = png[pos + 4:pos + 8], png[pos + 8:pos + 8 + n]
        pos += 12 + n
        if typ == b"IDAT":
            idat += body
        elif typ != b"IEND":
            chunks.append((typ, body))
    chunks.append((b"IDAT", zlib.compress(zlib.decompress(idat), 9)))
    chunks.append((b"IEND", b""))
    out = png[:8]
    for typ, body in chunks:
        out += struct.pack(">I", len(body)) + typ + body
        out += struct.pack(">I", zlib.crc32(typ + body))
    return out


def main(argv):
    if len(argv) < 3:
        sys.exit(__doc__)
    src, dst = argv[1], argv[2]
    w, h = png_size(src)
    with open(src, "rb") as f:
        b64 = base64.b64encode(f.read()).decode()
    svg = build_svg(b64, w, h)
    if "--svg-out" in argv:
        with open(argv[argv.index("--svg-out") + 1], "w") as f:
            f.write(svg)
    done = subprocess.run(["rsvg-convert", "-w", str(w), "-h", str(h), "-f", "png"],
                          input=svg.encode(), check=True, stdout=subprocess.PIPE)
    with open(dst, "wb") as f:
        f.write(recompress(done.stdout))


if __name__ == "__main__":
    main(sys.argv)
