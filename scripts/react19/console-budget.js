#!/usr/bin/env node
// Compares console output from a recorded React 19 run against the committed per-unit baseline.
//   node scripts/react19/console-budget.js check  [unit...]  # fail on any message not in the baseline
//   node scripts/react19/console-budget.js update [unit...]  # rewrite the baseline from the last runs
// Record first:
//   node scripts/react19/run.js --record <unit...>                    # React 19 (check and update)
//   REACT_RUNTIME=18 node scripts/react19/run.js --record <unit...>   # React 18 (update only)
// update only baselines messages that the same tests also print on React 18. A message that only
// React 19 prints is a regression to fix, so update refuses it.
const fs = require('node:fs');
const path = require('node:path');

const { root, readAllowlist } = require('./units');

const react19Dir = path.join(root, '.react19-console');
const react18Dir = path.join(root, '.react18-console');
const baselineDir = path.join(root, 'public/test/react19-baselines');

const [mode, ...unitArgs] = process.argv.slice(2);
if (mode !== 'check' && mode !== 'update') {
  console.error('Usage: console-budget.js check|update [unit...]');
  process.exit(2);
}
const units = unitArgs.length ? unitArgs : readAllowlist();
if (!units.length) {
  console.log('No units on the React 19 allowlist yet.');
  process.exit(0);
}

function baselineFile(unit) {
  return path.join(baselineDir, `${unit.replace(/\//g, '__')}.json`);
}

// Strip values that differ run to run so the same warning always produces the same key.
function normalize(message) {
  return message
    .replace(/\d+(\.\d+)?\s?ms/g, '<ms>')
    .replace(/\b[0-9a-f]{8,}\b/gi, '<id>')
    .replace(/:\d+:\d+/g, ':<line>');
}

function readReports(dir) {
  if (!fs.existsSync(dir)) {
    return null;
  }
  return fs.readdirSync(dir).map((file) => JSON.parse(fs.readFileSync(path.join(dir, file), 'utf8')));
}

function observe(reports, unit) {
  const counts = {};
  let files = 0;
  for (const report of reports) {
    if (!report.testPath.startsWith(`${unit}/`)) {
      continue;
    }
    files++;
    for (const { level, message } of report.messages) {
      const key = `${level}: ${normalize(message)}`;
      counts[key] = (counts[key] ?? 0) + 1;
    }
  }
  return { counts, files };
}

// Code-point order, so committed baselines sort the same in every locale.
const sortKeys = (object) =>
  Object.fromEntries(Object.entries(object).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)));

const react19Reports = readReports(react19Dir);
if (!react19Reports) {
  console.error(`No React 19 console reports. Run: node scripts/react19/run.js --record ${units.join(' ')}`);
  process.exit(2);
}
const react18Reports = mode === 'update' ? readReports(react18Dir) : null;
if (mode === 'update' && !react18Reports) {
  console.error(
    `update needs a React 18 recording to compare against. Run: REACT_RUNTIME=18 node scripts/react19/run.js --record ${units.join(' ')}`
  );
  process.exit(2);
}

let failed = false;
for (const unit of units) {
  const observed = observe(react19Reports, unit);
  if (!observed.files) {
    failed = true;
    console.error(`${unit}: no React 19 recording for this unit. Record it before checking.`);
    continue;
  }

  if (mode === 'update') {
    const react18 = observe(react18Reports, unit).counts;
    const baseline = {};
    const react19Only = [];
    for (const [key, count] of Object.entries(observed.counts)) {
      if (key in react18) {
        baseline[key] = count;
      } else {
        react19Only.push(key);
      }
    }
    if (react19Only.length) {
      failed = true;
      console.error(`${unit}: these messages appear only on React 19. Fix them; they cannot be baselined.`);
      for (const key of react19Only) {
        console.error(`  ${key}`);
      }
      continue;
    }
    fs.mkdirSync(baselineDir, { recursive: true });
    fs.writeFileSync(baselineFile(unit), JSON.stringify({ unit, messages: sortKeys(baseline) }, null, 2) + '\n');
    console.log(`${unit}: baseline has ${Object.keys(baseline).length} distinct messages, all also on React 18.`);
    continue;
  }

  const file = baselineFile(unit);
  const baseline = fs.existsSync(file) ? JSON.parse(fs.readFileSync(file, 'utf8')).messages : {};
  const added = Object.keys(observed.counts).filter((key) => !(key in baseline));
  const grew = Object.keys(observed.counts).filter((key) => key in baseline && observed.counts[key] > baseline[key]);
  if (added.length || grew.length) {
    failed = true;
    console.error(`${unit}: console budget exceeded`);
    for (const key of added) {
      console.error(`  new: ${key}`);
    }
    for (const key of grew) {
      console.error(`  more often (${baseline[key]} -> ${observed.counts[key]}): ${key}`);
    }
  } else {
    console.log(`${unit}: within console budget.`);
  }
}
process.exit(failed ? 1 : 0);
