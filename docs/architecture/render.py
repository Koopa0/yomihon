#!/usr/bin/env python3
"""Rebuild the architecture SVGs and PNGs. Requires CairoSVG."""
from __future__ import annotations

import argparse
import html
from pathlib import Path
from typing import Iterable

W = 1600
INK = '#152B3C'
MUTED = '#506574'
LINE = '#5B7081'
BORDER = '#C9D6DE'
BLUE = '#23699A'
TEAL = '#237869'
AMBER = '#986626'


def esc(value: str) -> str:
    return html.escape(value, quote=True)


class Canvas:
    def __init__(self, height: int, title: str, description: str):
        self.height = height
        self.parts = [f'''<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{height}" viewBox="0 0 {W} {height}" role="img" aria-labelledby="title description">
<title id="title">{esc(title)}</title><desc id="description">{esc(description)}</desc>
<defs>
<marker id="arrow" markerWidth="10" markerHeight="10" refX="8" refY="5" orient="auto-start-reverse" markerUnits="userSpaceOnUse"><path d="M1,1 L8,5 L1,9" fill="none" stroke="{LINE}" stroke-width="1.8"/></marker>
<marker id="write-arrow" markerWidth="10" markerHeight="10" refX="8" refY="5" orient="auto-start-reverse" markerUnits="userSpaceOnUse"><path d="M1,1 L8,5 L1,9" fill="none" stroke="{AMBER}" stroke-width="1.8"/></marker>
</defs>
<rect width="{W}" height="{height}" fill="#FFFFFF"/>
<g font-family="DejaVu Sans, Arial, sans-serif" fill="{INK}">''']

    def text(self, x: float, y: float, value: str, size: float = 22,
             weight: int = 400, color: str = INK, anchor: str = 'start') -> None:
        self.parts.append(f'<text x="{x}" y="{y}" font-size="{size}" font-weight="{weight}" fill="{color}" text-anchor="{anchor}">{esc(value)}</text>')

    def group(self, x: int, y: int, w: int, h: int, label: str) -> None:
        self.parts.append(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="14" fill="#F5F8FA" stroke="{BORDER}" stroke-width="1.5"/>')
        self.text(x+28, y+41, label, 25, 600)

    def box(self, x: int, y: int, w: int, h: int, title: str,
            lines: Iterable[str] = (), accent: str = BLUE, fill: str = '#FFFFFF',
            title_size: int = 24, text_size: int = 20) -> None:
        self.parts.append(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="9" fill="{fill}" stroke="{BORDER}" stroke-width="1.6"/>')
        self.parts.append(f'<path d="M{x+1},{y+14} V{y+h-14}" stroke="{accent}" stroke-width="3.5"/>')
        lines = list(lines)
        total = 29 + (len(lines)*27 + (8 if lines else 0))
        start = y + (h-total)/2 + 23
        self.text(x+23, start, title, title_size, 600)
        for i, line in enumerate(lines):
            self.text(x+23, start+35+i*27, line, text_size, color=MUTED)

    def edge(self, points: Iterable[tuple[int,int]], label: str = '',
             at: tuple[int,int] | None = None, dashed: bool = False,
             write: bool = False, both: bool = False) -> None:
        pts = ' '.join(f'{x},{y}' for x,y in points)
        marker = 'write-arrow' if write else 'arrow'
        color = AMBER if write else LINE
        dash = ' stroke-dasharray="7 6"' if dashed else ''
        start = f' marker-start="url(#{marker})"' if both else ''
        self.parts.append(f'<polyline points="{pts}" fill="none" stroke="{color}" stroke-width="1.9" stroke-linejoin="round" marker-end="url(#{marker})"{dash}{start}/>')
        if label and at:
            self.text(*at, label, 18, color=MUTED, anchor='middle')

    def line(self, x: int, y1: int, y2: int) -> None:
        self.parts.append(f'<path d="M{x},{y1} V{y2}" stroke="{BORDER}" stroke-width="1.5" stroke-dasharray="7 7"/>')

    def svg(self) -> str:
        return '\n'.join(self.parts) + '\n</g></svg>\n'


def system_context() -> Canvas:
    c = Canvas(930, 'System context', 'The editor maintains the vault. Yomihon reads captured generations, serves a loopback browser, changes status values, and stores reader marks outside the vault.')
    c.group(430, 60, 755, 775, 'Yomihon')
    c.box(55, 155, 295, 115, 'Editor', ['Markdown + sidecars'], accent=TEAL)
    c.box(55, 385, 295, 255, 'Vault', ['Notes', 'Attachments', 'Contract'], accent=TEAL)
    c.box(480, 185, 220, 125, 'Vault reader', ['Rooted file access'], title_size=23, text_size=18)
    c.box(800, 185, 330, 125, 'Reading generation', ['Graph · search · navigation'], title_size=23, text_size=18)
    c.box(800, 395, 330, 125, 'HTTP + rendering', ['templ · Markdown pipeline'], text_size=18)
    c.box(480, 665, 260, 110, 'Status writer', [], accent=AMBER)
    c.box(1260, 395, 285, 125, 'Browser', ['Reading + preferences'], text_size=18)
    c.box(1260, 665, 285, 125, 'Reader marks', ['User configuration'], accent=TEAL)
    c.edge([(202,270),(202,385)], 'edit', (233,334), write=True)
    c.edge([(350,455),(387,455),(387,247),(480,247)], 'read', (414,229))
    c.edge([(700,247),(800,247)])
    c.edge([(965,310),(965,395)])
    c.edge([(1130,457),(1260,457)], '127.0.0.1', (1195,438), both=True)
    c.edge([(965,520),(965,590),(610,590),(610,665)], 'status update', (788,575), write=True)
    c.edge([(480,720),(390,720),(390,610),(350,610)], 'status only', (378,754), write=True)
    c.edge([(1130,495),(1218,495),(1218,727),(1260,727)], 'read / write', (1276,617), write=True)
    return c


def reading_generations() -> Canvas:
    c = Canvas(970, 'Reading generations', 'Scans capture files and build complete derived generations. Incomplete reads retry before degraded publication. Each request captures a published generation.')
    c.group(50, 55, 1500, 575, 'Rebuild')
    c.group(50, 685, 1500, 225, 'Read')
    c.box(95, 155, 270, 120, 'Scan', ['Identity + metadata'])
    c.box(435, 155, 300, 120, 'Capture sources', ['Bodies + parsed products'], text_size=18)
    c.box(805, 155, 320, 120, 'Derive views', ['Links · search · navigation'], text_size=18)
    c.box(1205, 155, 300, 120, 'Candidate', ['Reading generation'])
    c.edge([(365,215),(435,215)])
    c.edge([(735,215),(805,215)])
    c.edge([(1125,215),(1205,215)])
    c.box(95, 385, 355, 160, 'Scan refused', ['Keep published generation', 'Collision or scan error'], accent=AMBER, text_size=18)
    c.box(510, 385, 450, 160, 'Incomplete build', ['Retry; initially retain current', 'After 3 attempts: publish degraded'], accent=AMBER, text_size=19)
    c.box(1035, 385, 470, 160, 'Complete build', ['Publish candidate'], accent=TEAL)
    c.edge([(230,275),(230,385)])
    c.edge([(1355,275),(1355,331),(735,331),(735,385)])
    c.edge([(1355,275),(1355,385)])
    c.box(1175, 755, 330, 110, 'Published generation', ['Atomic pointer swap'], title_size=22, accent=TEAL)
    c.box(675, 755, 350, 110, 'Capture once', ['Request-local handle'])
    c.box(95, 755, 440, 110, 'Reading view', ['Content + derived projections'])
    c.edge([(1355,545),(1355,755)])
    c.edge([(735,545),(735,612),(1210,612),(1210,755)], 'degraded candidate', (971,596), write=True)
    c.edge([(1175,810),(1025,810)])
    c.edge([(675,810),(535,810)])
    return c


def reading_pipeline() -> Canvas:
    c = Canvas(1030, 'Reading pipeline', 'Captured note bodies and the generation resolver feed a bounded Markdown pipeline. The output combines HTML, headings and diagnostics with the reading interface.')
    c.group(480, 60, 650, 850, 'Markdown pipeline')
    c.box(60, 185, 340, 120, 'Captured body', ['Authored Markdown'])
    c.box(60, 410, 340, 235, 'Generation', ['Link resolver', 'Embed bodies', 'Resource identities'])
    c.box(535, 165, 540, 100, 'Protect code + comments', [], title_size=24)
    c.box(535, 325, 540, 105, 'Resolve links + excerpts', ['One-level transclusion'])
    c.box(535, 490, 540, 115, 'Render HTML', ['Goldmark · Chroma · inert markup'])
    c.box(535, 670, 540, 115, 'Build anchors + TOC', ['Local asset URLs'])
    c.edge([(400,245),(447,245),(447,215),(535,215)])
    c.edge([(400,527),(447,527),(447,377),(535,377)])
    c.edge([(805,265),(805,325)])
    c.edge([(805,430),(805,490)])
    c.edge([(805,605),(805,670)])
    c.box(1215, 585, 315, 200, 'Reading view', ['templ shell + HTML', 'TOC + diagnostics'], text_size=19, accent=TEAL)
    c.edge([(1075,725),(1215,725)])
    c.box(1215, 865, 315, 120, 'Browser controls', ['Mermaid · speech', 'Preview · reading aids'], text_size=18)
    c.edge([(1372,785),(1372,865)])
    return c


def status_update() -> Canvas:
    c = Canvas(1180, 'Status update', 'The successful status path validates a live file, prepares a surgical replacement, checks authority again, installs with filesystem-specific protection, and confirms after directory synchronization.')
    c.box(80, 50, 320, 100, 'Browser', [], accent=BLUE)
    c.box(620, 50, 360, 100, 'Status writer', [], accent=AMBER)
    c.box(1190, 50, 345, 100, 'Vault filesystem', [], accent=TEAL)
    for x in (240,800,1362):
        c.line(x,150,1135)
    c.edge([(240,250),(800,250)], 'POST /status', (520,199), write=True)
    c.text(520,229,'path · from · to · content identity',18,color=MUTED,anchor='middle')
    c.edge([(800,345),(1362,345)], 'Read source + validate target', (1081,326))
    c.edge([(1362,410),(800,410)], 'Bytes + file identity', (1081,392), dashed=True)
    c.box(563, 460, 474, 95, 'Compare state and content', ['Check contract transition'], accent=AMBER, text_size=19)
    c.edge([(800,620),(1362,620)], 'Prepare replacement + sync file', (1081,601), write=True)
    c.edge([(800,715),(1362,715)], 'Recheck source + authority', (1081,696))
    c.edge([(800,810),(1362,810)], 'Install replacement', (1081,791), write=True)
    c.text(1120, 844, 'Exchange / retained hardlink / rename', 18, color=MUTED, anchor='middle')
    c.edge([(800,935),(1362,935)], 'Sync parent directory', (1081,916), write=True)
    c.edge([(1362,1005),(800,1005)], 'Durable success', (1081,986), dashed=True)
    c.edge([(800,1090),(240,1090)], '303 redirect', (520,1071))
    return c


def contract_commands() -> Canvas:
    c = Canvas(1090, 'Contract and commands', 'The contract supplies typed reading and lifecycle declarations. CLI output has an additional privacy gate and source revalidation; local reading is a separate boundary.')
    c.box(560, 55, 480, 135, 'Vault contract', ['Types · lifecycle · policies', 'vault-schema.toml'], accent=TEAL)
    c.box(580, 280, 440, 110, 'Schema capabilities', ['Loaded declarations'])
    c.edge([(800,190),(800,280)])
    for end in (250,800,1350):
        c.edge([(800,390),(800,447),(end,447),(end,505)])
    c.box(55, 505, 390, 140, 'Reading projections', ['Course roles · metadata'])
    c.box(605, 505, 390, 140, 'Status writer', ['Artifact scope · transitions'], accent=AMBER, text_size=19)
    c.box(1155, 505, 390, 140, 'CLI judge', ['check · coverage · exists'])
    c.box(55, 825, 390, 115, 'Reading interface', ['Local content'])
    c.box(605, 825, 390, 115, 'Status value', ['Governed note'], accent=AMBER)
    c.box(1155, 720, 390, 115, 'Output gate', ['Privacy + source revalidation'], accent=AMBER, text_size=18)
    c.box(1155, 940, 390, 105, 'Command output', ['JSON / human-readable'], accent=TEAL, text_size=19)
    c.edge([(250,645),(250,825)], 'render', (290,741))
    c.edge([(800,645),(800,825)], 'revalidate, then write', (945,741), write=True)
    c.edge([(1350,645),(1350,720)])
    c.edge([(1350,835),(1350,940)])
    return c


DRAWINGS = {
    '01-system-context': system_context,
    '02-reading-generations': reading_generations,
    '03-reading-pipeline': reading_pipeline,
    '04-status-update': status_update,
    '05-contract-and-commands': contract_commands,
}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--width', type=int, default=3200, help='PNG output width (default: 3200)')
    args = parser.parse_args()
    if args.width < 400 or args.width > 10000:
        parser.error('--width must be between 400 and 10000 pixels')
    try:
        import cairosvg
    except ImportError as exc:
        raise SystemExit('Install CairoSVG first: python -m pip install cairosvg') from exc
    destination = Path(__file__).resolve().parent
    for name, draw in DRAWINGS.items():
        canvas = draw()
        svg = canvas.svg()
        (destination / f'{name}.svg').write_text(svg, encoding='utf-8')
        cairosvg.svg2png(bytestring=svg.encode(), write_to=str(destination / f'{name}.png'), output_width=args.width)
        print(f'{name}.svg / {name}.png')


if __name__ == '__main__':
    main()
