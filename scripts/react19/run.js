#!/usr/bin/env node
// Runs Jest on React 19 for the given units, or for every allowlisted unit when none are given.
//   node scripts/react19/run.js                       # whole allowlist
//   node scripts/react19/run.js public/app/features/x  # one unit
//   node scripts/react19/run.js --record ...           # also write console reports for the budget check
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

const { root, readAllowlist } = require('./units');

const args = process.argv.slice(2);
const record = args.includes('--record');
const jestArgs = [];
const units = [];
for (const arg of args) {
  if (arg === '--record') {
    continue;
  }
  (arg.startsWith('-') ? jestArgs : units).push(arg);
}

const targets = units.length ? units : readAllowlist();
if (!targets.length) {
  console.log('No units on the React 19 allowlist yet.');
  process.exit(0);
}

const env = { ...process.env };
if (record) {
  const runtime = env.REACT_RUNTIME ?? '19';
  env.REACT19_CONSOLE_REPORT_DIR = path.join(root, runtime === '19' ? '.react19-console' : `.react${runtime}-console`);
  fs.rmSync(env.REACT19_CONSOLE_REPORT_DIR, { recursive: true, force: true });
}

// Anchor each unit to the repo root so a unit like public/app/features/query never also
// matches public/app/features/query-library.
const patterns = targets.map((unit) => `^${root}/${unit}/`.replace(/[.+?]/g, '\\$&'));

const result = spawnSync('yarn', ['jest', '--no-watch', '-c', 'jest.config.react19.js', ...jestArgs, ...patterns], {
  cwd: root,
  env,
  stdio: 'inherit',
});
process.exit(result.status ?? 1);
