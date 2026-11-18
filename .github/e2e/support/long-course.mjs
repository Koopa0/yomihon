// A sidebar offers its filter only past twenty rows, and the fixture's courses
// are short. The probes that are about the filter lengthen the course they read
// on their own copy of the folder, as the server sees it, and wait until the
// sidebar draws the box, so what they exercise is the page a reader with a long
// course gets rather than a rewritten one. The server is shared with every probe
// that runs after, so the returned function puts the course back and waits for
// the sidebar to lose its box again.
import { readFile, rm, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

const SPARE = 20;
const name = (index) => `Spare ${String(index + 1).padStart(2, '0')}`;

export async function lengthenCourse(base, page) {
  const root = process.env.YOMIHON_FIXTURE_ROOT;
  if (!root) throw new Error('lengthenCourse: YOMIHON_FIXTURE_ROOT is not set; run the probe under serve.sh');
  const map = join(root, 'Maps', 'branches.md');
  const source = await readFile(map, 'utf8');
  const anchor = '- [[C04 Unwritten]]\n';
  if (!source.includes(anchor)) throw new Error('lengthenCourse: the branch course no longer ends its main line with C04 Unwritten');
  const rows = Array.from({ length: SPARE }, (_, index) => `- [[${name(index)}]]\n`).join('');
  for (let index = 0; index < SPARE; index += 1) {
    await writeFile(join(root, 'Course', `${name(index)}.md`), '---\ntype: lesson\n\n\ndraft: true\n---\nSpare body\n');
  }
  const waitFor = async (wanted) => {
    const deadline = Date.now() + 30000;
    while (Date.now() < deadline) {
      const response = await fetch(base + page);
      if (response.ok && /<input\b[^>]*\bdata-nav-filter\b/.test(await response.text()) === wanted) return;
      await new Promise((resolve) => setTimeout(resolve, 500));
    }
    throw new Error(`lengthenCourse: ${page} ${wanted ? 'drew no filter' : 'still draws its filter'} within 30s of the course changing`);
  };
  await writeFile(map, source.replace(anchor, anchor + rows));
  const restore = async () => {
    await writeFile(map, source);
    for (let index = 0; index < SPARE; index += 1) await rm(join(root, 'Course', `${name(index)}.md`), { force: true });
    await waitFor(false);
  };
  try {
    await waitFor(true);
  } catch (error) {
    await restore().catch(() => {});
    throw error;
  }
  return restore;
}
