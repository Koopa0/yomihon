import { createHash } from 'node:crypto';
import { mkdir, readFile, realpath, writeFile } from 'node:fs/promises';
import { basename, dirname, join } from 'node:path';

// A fixture the probe will not write approvals into: the caller's own message,
// which each probe says in its own name.
export class FixtureRefused extends Error {}

// The vault a probe writes beside has to be the disposable copy serve.sh made
// and nothing else, because the record written next to it says which figures to
// run.
export const fixtureRoot = async () => {
  const supplied = process.env.YOMIHON_FIXTURE_ROOT;
  if (!supplied) throw new FixtureRefused('no disposable fixture root');
  const root = await realpath(supplied);
  if (basename(root) !== 'vault' || !basename(dirname(root)).startsWith('yomihon-serve.')) {
    throw new FixtureRefused('unsafe fixture root');
  }
  return root;
};

export const fixtureHome = async (root) => {
  const supplied = process.env.YOMIHON_FIXTURE_HOME;
  if (!supplied) throw new FixtureRefused('no disposable server home');
  const home = await realpath(supplied);
  if (basename(home) !== 'home' || dirname(home) !== dirname(root)) {
    throw new FixtureRefused('the server home is not beside the disposable vault');
  }
  return home;
};

// The approval command needs a person at a terminal, which a probe is not, so
// the record it would write is written here in its documented shape: the
// per-vault directory named by the digest of the resolved root. It returns the
// file, which the caller removes when it is done.
export const approveFigures = async (root, home, directory, names) => {
  const config = process.platform === 'darwin' ? join(home, 'Library', 'Application Support') : join(home, '.config');
  const vaultDir = join(config, 'yomihon', createHash('sha256').update(root.normalize('NFC')).digest('hex').slice(0, 32));
  const approvals = [];
  for (const name of names) {
    const bytes = await readFile(join(root, directory, name));
    approvals.push({ path: `${directory}/${name}`, sha256: createHash('sha256').update(bytes).digest('hex'), time: new Date().toISOString() });
  }
  await mkdir(vaultDir, { recursive: true, mode: 0o700 });
  const file = join(vaultDir, 'figures.json');
  await writeFile(file, `${JSON.stringify({ version: 1, vault: root, approvals }, null, 2)}\n`, { mode: 0o600 });
  return file;
};
