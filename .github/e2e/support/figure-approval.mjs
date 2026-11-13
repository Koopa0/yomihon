import { createHash } from 'node:crypto';
import { mkdir, readFile, realpath, writeFile } from 'node:fs/promises';
import { basename, dirname, join } from 'node:path';

// The approval command needs a person at a terminal, which a probe is not, so
// the record it would write is written here in its documented shape: the
// per-vault directory named by the digest of the resolved root, in a home
// that sits beside the disposable vault and nowhere else.
export const approveFigures = async (directory, names) => {
  const suppliedRoot = process.env.YOMIHON_FIXTURE_ROOT;
  if (!suppliedRoot) throw new Error('no disposable fixture root');
  const root = await realpath(suppliedRoot);
  if (basename(root) !== 'vault' || !basename(dirname(root)).startsWith('yomihon-serve.')) throw new Error('unsafe fixture root');
  const suppliedHome = process.env.YOMIHON_FIXTURE_HOME;
  if (!suppliedHome) throw new Error('no disposable server home');
  const home = await realpath(suppliedHome);
  if (basename(home) !== 'home' || dirname(home) !== dirname(root)) throw new Error('the server home is not beside the disposable vault');
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
