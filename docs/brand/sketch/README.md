<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Pencil-sketch media

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift-ai

The README and docs diagrams are drawn in a symmetric pencil style. Colours, text and
layout stay as written; only the lines change. Logos, marks and the favicon are never
processed.

- `sketch_svg.py IN.svg [OUT.svg]`: fine graphite jitter, a faint second pencil pass and
  diagonal hatching for every stroked shape, plus paper grain on the background.
  Fill-only shapes (logo marks, dots, arrowheads) stay exact. It marks the file with
  `data-sketch="pencil"`, so running it twice changes nothing.
- `sketch_raster.py IN.png OUT.png`: contours, hatching and grain for a raster
  illustration, rendered at the original size.

After editing a diagram that has a PNG copy, run the script on the source SVG (or
edit an already sketched one) and re-render the copy at its current size:

```bash
python3 docs/brand/sketch/sketch_svg.py docs/brand/readme-media/routing.svg
rsvg-convert -w 1400 -h 480 docs/brand/readme-media/routing.svg -o docs/brand/readme-media/routing.png
```
