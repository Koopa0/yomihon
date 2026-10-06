# Architecture diagrams

The architecture documents embed the PNG files. SVG files provide scalable copies.
Rebuilding requires Python 3, CairoSVG, and its Cairo runtime. Edit `render.py`, then regenerate both formats:

```sh
python -m pip install cairosvg
python docs/architecture/render.py
```

PNG output is 3200 pixels wide; `--width` changes the export size.
Keep diagram labels in English. Descriptions and source links belong in the architecture documents.
