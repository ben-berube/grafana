#!/usr/bin/env node
// Fails when the React 19 allowlist loses a unit relative to a git ref (default origin/main),
// names a directory that is not a unit, or is not sorted. The allowlist only grows.
//   node scripts/react19/check-allowlist.js [base-ref]
const { execFileSync } = require('node:child_process');

const { root, listUnits, readAllowlist } = require('./units');

const baseRef = process.argv[2] ?? process.env.REACT19_ALLOWLIST_BASE ?? 'origin/main';
const current = readAllowlist();
const units = new Set(listUnits());
const problems = [];

let base = [];
try {
  const raw = execFileSync('git', ['show', `${baseRef}:scripts/react19/allowlist.json`], {
    cwd: root,
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'ignore'],
  });
  base = JSON.parse(raw).units;
} catch {
  console.log(`No allowlist at ${baseRef}; treating it as empty.`);
}

const currentSet = new Set(current);
for (const unit of base) {
  if (!currentSet.has(unit)) {
    problems.push(`Removed from the allowlist since ${baseRef}: ${unit}. Fix the React 19 failure instead.`);
  }
}
for (const unit of current) {
  if (!units.has(unit)) {
    problems.push(`Not a migration unit: ${unit}. See scripts/react19/units.js.`);
  }
}
if (new Set(current).size !== current.length) {
  problems.push('The allowlist has duplicate entries.');
}
if (current.join('\n') !== [...current].sort().join('\n')) {
  problems.push('The allowlist must be sorted.');
}

if (problems.length) {
  console.error(problems.join('\n'));
  process.exit(1);
}
console.log(`Allowlist OK: ${current.length} of ${units.size} units, nothing removed since ${baseRef}.`);
