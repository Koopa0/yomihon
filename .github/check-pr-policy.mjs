import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

const gateLabels = [
  'Gate 1 — Architecture and open-source engineering quality',
  'Gate 2 — Real-user and third-party-agent usability',
  'Gate 3 — Test and evidence-system quality',
];

const envelopeLabels = [
  'Reviewed commit',
  'Builder',
  'Independent reviewer',
  'Evidence',
  ...gateLabels,
  'Final verdict',
  'Unverified or blocked checks',
  'Unresolved owner decisions',
  'Active exceptions',
];

const placeholder = /^(?:pending|none yet|n\/a|tbd|todo|replace-|list-)/i;

const field = (body, label) => {
  const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = body.match(new RegExp(`^${escaped}:\\s*(.+?)\\s*$`, 'mu'));
  return match?.[1].trim().replace(/^`|`$/g, '') || '';
};

const fieldCount = (body, label) => {
  const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return [...body.matchAll(new RegExp(`^${escaped}:`, 'gmu'))].length;
};

const identity = (value) => value.replace(/^@/u, '').toLowerCase();

const approvedException = (id, root) => {
  const directory = join(root, 'docs', 'exceptions');
  const candidates = readdirSync(directory).filter((name) => name.startsWith(`${id}-`) && name.endsWith('.md'));
  if (candidates.length !== 1) return `${id} resolves to ${candidates.length} exception records`;
  const record = readFileSync(join(directory, candidates[0]), 'utf8');
  if (!new RegExp(`^Exception ID:\\s*\`?${id}\`?\\s{2}$`, 'mu').test(record)) {
    return `${id} has no matching record identity`;
  }
  if (!/^Status:\s*Approved\s{2}$/mu.test(record)) return `${id} is not Approved`;
  if (/^Independent approver:\s*(?:not yet assigned|pending)\s*$/imu.test(record)) return `${id} has no independent approver`;
  if (!/^Approved \/ rejected:\s*Approved\s*$/mu.test(record)) return `${id} has no completed approval block`;
  return '';
};

export const validatePullRequest = ({ body, head, author, root = process.cwd() }) => {
  const errors = [];
  for (const label of envelopeLabels) {
    if (fieldCount(body, label) !== 1) errors.push(`${label} must appear exactly once`);
  }

  const reviewed = field(body, 'Reviewed commit');
  if (!/^[0-9a-f]{40}$/i.test(reviewed)) {
    errors.push('Reviewed commit must be one 40-character hexadecimal commit');
  } else if (head && reviewed.toLowerCase() !== head.toLowerCase()) {
    errors.push(`Reviewed commit ${reviewed} does not match pull-request head ${head}`);
  }

  for (const label of gateLabels) {
    if (field(body, label) !== 'PASS') errors.push(`${label} must be PASS`);
  }
  if (field(body, 'Final verdict') !== 'GO') errors.push('Final verdict must be GO');

  const builder = field(body, 'Builder');
  const reviewer = field(body, 'Independent reviewer');
  if (!builder || placeholder.test(builder)) errors.push('Builder identity is missing');
  if (!reviewer || placeholder.test(reviewer)) errors.push('Independent reviewer identity is missing');
  if (builder && reviewer && identity(builder) === identity(reviewer)) {
    errors.push('Builder and independent reviewer must differ');
  }
  if (author && reviewer && identity(author) === identity(reviewer)) {
    errors.push('Pull-request author cannot be the independent reviewer');
  }

  const evidence = field(body, 'Evidence');
  const reviewPath = /^docs\/reviews\/[A-Za-z0-9][A-Za-z0-9._/-]*\.md$/u.test(evidence)
    && !evidence.split('/').includes('..');
  const evidenceShape = /^https:\/\/[^\s]+$/u.test(evidence)
    || /^sha256:[0-9a-f]{64}$/iu.test(evidence)
    || reviewPath;
  if (!evidence || placeholder.test(evidence) || !evidenceShape) {
    errors.push('Evidence must be an HTTPS URL, a docs/reviews path, or a sha256 digest');
  } else if (reviewPath) {
    const reviewFile = join(root, evidence);
    if (!existsSync(reviewFile) || !statSync(reviewFile).isFile() || statSync(reviewFile).size === 0) {
      errors.push(`Review evidence does not name a non-empty file: ${evidence}`);
    }
  }
  if (field(body, 'Unverified or blocked checks').toLowerCase() !== 'none') {
    errors.push('Unverified or blocked checks must be none for GO');
  }
  if (field(body, 'Unresolved owner decisions').toLowerCase() !== 'none') {
    errors.push('Unresolved owner decisions must be none for GO');
  }

  const exceptionField = field(body, 'Active exceptions');
  if (!exceptionField || placeholder.test(exceptionField)) {
    errors.push('Active exceptions must be none or approved exception IDs');
  } else if (exceptionField.toLowerCase() !== 'none') {
    const ids = exceptionField.split(/[\s,]+/u).filter(Boolean);
    for (const id of ids) {
      if (!/^EX-\d{4}-\d{3}$/u.test(id)) {
        errors.push(`Invalid exception ID ${id}`);
        continue;
      }
      const error = approvedException(id, root);
      if (error) errors.push(error);
    }
  }
  return errors;
};

const selfTest = () => {
  const head = '0123456789abcdef0123456789abcdef01234567';
  const valid = `Reviewed commit: \`${head}\`
Builder: \`builder-session\`
Independent reviewer: \`reviewer-session\`
Evidence: \`https://example.invalid/review/1\`
Gate 1 — Architecture and open-source engineering quality: PASS
Gate 2 — Real-user and third-party-agent usability: PASS
Gate 3 — Test and evidence-system quality: PASS
Final verdict: GO
Unverified or blocked checks: \`none\`
Unresolved owner decisions: \`none\`
Active exceptions: \`none\``;
  const cases = [
    ['valid envelope', valid, 0],
    ['stale snapshot', valid.replace(head, '1123456789abcdef0123456789abcdef01234567'), 1],
    ['partial gate', valid.replace('Gate 2 — Real-user and third-party-agent usability: PASS', 'Gate 2 — Real-user and third-party-agent usability: PARTIAL'), 1],
    ['same reviewer', valid.replace('reviewer-session', 'builder-session'), 1],
    ['same reviewer alias', valid.replace('Builder: `builder-session`', 'Builder: `@reviewer-session`'), 1],
    ['unresolved work', valid.replace('Unresolved owner decisions: `none`', 'Unresolved owner decisions: `wire contract`'), 1],
    ['missing evidence', valid.replace('https://example.invalid/review/1', 'replace-with-review'), 1],
    ['malformed digest evidence', valid.replace('https://example.invalid/review/1', 'sha256:no'), 1],
    ['traversing review evidence', valid.replace('https://example.invalid/review/1', 'docs/reviews/../../outside.md'), 1],
    ['missing review evidence', valid.replace('https://example.invalid/review/1', 'docs/reviews/missing.md'), 1],
    ['duplicate gate', `${valid}\nGate 1 — Architecture and open-source engineering quality: PENDING`, 1],
  ];
  let failed = false;
  for (const [name, body, wantErrors] of cases) {
    const errors = validatePullRequest({ body, head, author: 'pull-request-author' });
    const gotErrors = errors.length === 0 ? 0 : 1;
    if (gotErrors !== wantErrors) {
      console.error(`self-test ${name}: error class ${gotErrors}, want ${wantErrors}: ${errors.join('; ')}`);
      failed = true;
    }
  }
  if (failed) process.exit(1);
  console.log('check-pr-policy: self-test passed');
};

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  if (process.argv[2] === '--self-test') {
    selfTest();
  } else {
    const errors = validatePullRequest({
      body: process.env.PR_BODY || '',
      head: process.env.PR_HEAD_SHA || '',
      author: process.env.PR_AUTHOR || '',
    });
    if (errors.length > 0) {
      for (const error of errors) console.error(`PR policy: ${error}`);
      process.exit(1);
    }
    console.log(`PR policy: three-gate evidence is complete for ${process.env.PR_HEAD_SHA}`);
  }
}
