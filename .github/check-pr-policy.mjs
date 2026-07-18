import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
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
const privateMergeException = 'EX-2026-001';

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

const approvedException = (id, root, builder, author) => {
  const directory = join(root, 'docs', 'exceptions');
  const candidates = readdirSync(directory).filter((name) => name.startsWith(`${id}-`) && name.endsWith('.md'));
  if (candidates.length !== 1) return `${id} resolves to ${candidates.length} exception records`;
  const record = readFileSync(join(directory, candidates[0]), 'utf8');
  if (fieldCount(record, 'Exception ID') !== 1 || field(record, 'Exception ID') !== id) {
    return `${id} has no matching record identity`;
  }
  if (fieldCount(record, 'Status') !== 1 || field(record, 'Status') !== 'Approved') {
    return `${id} must have exactly one Approved status`;
  }
  const independentApprover = field(record, 'Independent approver');
  if (fieldCount(record, 'Independent approver') !== 1
      || !independentApprover
      || placeholder.test(independentApprover)) {
    return `${id} has no independent approver`;
  }
  if (fieldCount(record, 'Approved / rejected') !== 1
      || field(record, 'Approved / rejected') !== 'Approved') {
    return `${id} must have exactly one Approved decision`;
  }
  const approvalBlockApprover = field(record, 'Approver');
  if (fieldCount(record, 'Approver') !== 1
      || !approvalBlockApprover
      || placeholder.test(approvalBlockApprover)
      || identity(approvalBlockApprover) !== identity(independentApprover)) {
    return `${id} approval identities are incomplete or inconsistent`;
  }
  if ((builder && identity(independentApprover) === identity(builder))
      || (author && identity(independentApprover) === identity(author))) {
    return `${id} was not independently approved`;
  }
  if (fieldCount(record, 'Date') !== 1 || !/^\d{4}-\d{2}-\d{2}$/u.test(field(record, 'Date'))) {
    return `${id} has no completed approval date`;
  }
  if (fieldCount(record, 'Snapshot') !== 1 || !/^[0-9a-f]{40}$/iu.test(field(record, 'Snapshot'))) {
    return `${id} has no immutable approval snapshot`;
  }
  const reason = field(record, 'Reason');
  if (fieldCount(record, 'Reason') !== 1 || !reason || placeholder.test(reason)) {
    return `${id} has no completed approval reason`;
  }
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
    errors.push('Unverified or blocked checks must be none for merge acceptance');
  }
  if (field(body, 'Unresolved owner decisions').toLowerCase() !== 'none') {
    errors.push('Unresolved owner decisions must be none for merge acceptance');
  }

  const exceptionField = field(body, 'Active exceptions');
  let exceptionPermitted = false;
  if (!exceptionField || placeholder.test(exceptionField)) {
    errors.push('Active exceptions must be none or approved exception IDs');
  } else if (exceptionField.toLowerCase() !== 'none') {
    const ids = exceptionField.split(/[\s,]+/u).filter(Boolean);
    let privateExceptionApproved = false;
    if (ids.length !== 1 || ids[0] !== privateMergeException) {
      errors.push(`Only ${privateMergeException} permits an exception-backed private merge`);
    }
    for (const id of ids) {
      if (!/^EX-\d{4}-\d{3}$/u.test(id)) {
        errors.push(`Invalid exception ID ${id}`);
        continue;
      }
      const error = approvedException(id, root, builder, author);
      if (error) errors.push(error);
      if (!error && id === privateMergeException) privateExceptionApproved = true;
    }
    exceptionPermitted = ids.length === 1
      && ids[0] === privateMergeException
      && privateExceptionApproved;
  }

  const expectedVerdict = exceptionPermitted ? 'ACCEPT-WITH-GATES' : 'GO';
  if (field(body, 'Final verdict') !== expectedVerdict) {
    errors.push(`Final verdict must be ${expectedVerdict}`);
  }
  return errors;
};

const selfTest = () => {
  const head = '0123456789abcdef0123456789abcdef01234567';
  const workspace = mkdtempSync(join(tmpdir(), 'yomihon-pr-policy-'));
  const writeException = (root, {
    status = 'Approved',
    independentApprover = 'exception-reviewer',
    decision = 'Approved',
    approver = independentApprover,
    date = '2026-07-18',
    snapshot = head,
    reason = 'independent review approved the private merge exception',
    extra = '',
  } = {}) => {
    const exceptionDir = join(root, 'docs', 'exceptions');
    mkdirSync(exceptionDir, { recursive: true });
    writeFileSync(join(exceptionDir, 'EX-2026-001-private-branch-protection.md'), `Exception ID: \`EX-2026-001\`${'  '}
Status: ${status}${'  '}
Independent approver: ${independentApprover}

## Ownership and closure

Implementation owner: @Koopa0
Risk owner: @Koopa0

## Approval

Approved / rejected: ${decision}
Approver: ${approver}
Date: ${date}
Snapshot: ${snapshot}
Reason: ${reason}
${extra}
`);
  };
  const approvedRoot = join(workspace, 'approved');
  const proposedRoot = join(workspace, 'proposed');
  const selfApprovedRoot = join(workspace, 'self-approved');
  const incompleteRoot = join(workspace, 'incomplete');
  const duplicateStatusRoot = join(workspace, 'duplicate-status');
  const contradictoryDecisionRoot = join(workspace, 'contradictory-decision');
  writeException(approvedRoot);
  writeException(proposedRoot, {
    status: 'Proposed',
    independentApprover: 'not yet assigned',
    decision: 'pending',
    approver: 'not yet assigned',
    date: 'pending',
    snapshot: 'pending',
    reason: 'pending',
  });
  writeException(selfApprovedRoot, {
    independentApprover: 'builder-session',
    approver: 'builder-session',
  });
  writeException(incompleteRoot, { date: 'pending', snapshot: 'pending', reason: 'pending' });
  writeException(duplicateStatusRoot, { extra: `Status: Proposed${'  '}` });
  writeException(contradictoryDecisionRoot, { extra: 'Approved / rejected: Rejected' });
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
  const exceptionPermitted = valid
    .replace('Final verdict: GO', 'Final verdict: ACCEPT-WITH-GATES')
    .replace('Active exceptions: `none`', 'Active exceptions: `EX-2026-001`');
  const cases = [
    ['valid envelope', valid, 0],
    ['approved private-merge exception', exceptionPermitted, 0],
    ['proposed exception has no force', exceptionPermitted, 1, proposedRoot],
    ['builder cannot approve an exception', exceptionPermitted, 1, selfApprovedRoot],
    ['approval details must be complete', exceptionPermitted, 1, incompleteRoot],
    ['exception status must be singular', exceptionPermitted, 1, duplicateStatusRoot],
    ['exception decision must be singular', exceptionPermitted, 1, contradictoryDecisionRoot],
    ['GO overstates an active exception', exceptionPermitted.replace('Final verdict: ACCEPT-WITH-GATES', 'Final verdict: GO'), 1],
    ['conditional verdict needs an exception', valid.replace('Final verdict: GO', 'Final verdict: ACCEPT-WITH-GATES'), 1],
    ['additional exception is unsupported', exceptionPermitted.replace('Active exceptions: `EX-2026-001`', 'Active exceptions: `EX-2026-001 EX-2026-002`'), 1],
    ['stale snapshot', valid.replace(head, '1123456789abcdef0123456789abcdef01234567'), 1],
    ['partial gate', valid.replace('Gate 2 — Real-user and third-party-agent usability: PASS', 'Gate 2 — Real-user and third-party-agent usability: PARTIAL'), 1],
    ['same reviewer', valid.replace('reviewer-session', 'builder-session'), 1],
    ['same reviewer alias', valid.replace('Builder: `builder-session`', 'Builder: `@reviewer-session`'), 1],
    ['unverified work', valid.replace('Unverified or blocked checks: `none`', 'Unverified or blocked checks: `branch protection`'), 1],
    ['unresolved work', valid.replace('Unresolved owner decisions: `none`', 'Unresolved owner decisions: `wire contract`'), 1],
    ['missing evidence', valid.replace('https://example.invalid/review/1', 'replace-with-review'), 1],
    ['malformed digest evidence', valid.replace('https://example.invalid/review/1', 'sha256:no'), 1],
    ['traversing review evidence', valid.replace('https://example.invalid/review/1', 'docs/reviews/../../outside.md'), 1],
    ['missing review evidence', valid.replace('https://example.invalid/review/1', 'docs/reviews/missing.md'), 1],
    ['duplicate gate', `${valid}\nGate 1 — Architecture and open-source engineering quality: PENDING`, 1],
  ];
  let failed = false;
  try {
    for (const [name, body, wantErrors, root = approvedRoot] of cases) {
      const errors = validatePullRequest({ body, head, author: 'pull-request-author', root });
      const gotErrors = errors.length === 0 ? 0 : 1;
      if (gotErrors !== wantErrors) {
        console.error(`self-test ${name}: error class ${gotErrors}, want ${wantErrors}: ${errors.join('; ')}`);
        failed = true;
      }
    }
  } finally {
    rmSync(workspace, { force: true, recursive: true });
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
