import { createHash } from 'node:crypto';
import { mkdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import React from 'react';

// Only loaded by jest.config.react19.js. Fails fast if the moduleNameMapper stops applying,
// because a React 19 run that silently executes on React 18 would report false parity.
const expectedRuntime = process.env.REACT_RUNTIME ?? '19';
if (!React.version.startsWith(`${expectedRuntime}.`)) {
  throw new Error(`jest.config.react19.js expected React ${expectedRuntime}, got React ${React.version}`);
}

// When REACT19_CONSOLE_REPORT_DIR is set, every console.error and console.warn emitted by a
// test file is written to that directory so scripts/react19/console-budget.js can diff it
// against the committed per-unit baseline.
const reportDir = process.env.REACT19_CONSOLE_REPORT_DIR;

const messages: Array<{ level: 'error' | 'warn'; message: string }> = [];
const originals = { error: console.error, warn: console.warn };

function firstLine(args: unknown[]): string {
  const [head, ...rest] = args;
  let text = typeof head === 'string' ? head : head instanceof Error ? head.message : String(head);
  // React formats with %s placeholders; substitute so baseline entries are readable.
  for (const value of rest) {
    if (!text.includes('%s')) {
      break;
    }
    text = text.replace('%s', typeof value === 'string' ? value : String(value));
  }
  return text.split('\n')[0].trim().slice(0, 300);
}

if (reportDir) {
  const recordTo =
    (level: 'error' | 'warn', passThrough: (...args: unknown[]) => void) =>
    (...args: unknown[]) => {
      messages.push({ level, message: firstLine(args) });
      passThrough(...args);
    };

  beforeEach(() => {
    console.error = recordTo('error', console.error);
    console.warn = recordTo('warn', console.warn);
  });

  afterEach(() => {
    console.error = originals.error;
    console.warn = originals.warn;
  });

  afterAll(() => {
    const testPath = path.relative(process.cwd(), expect.getState().testPath ?? 'unknown');
    const file = createHash('sha1').update(testPath).digest('hex') + '.json';
    mkdirSync(reportDir, { recursive: true });
    writeFileSync(path.join(reportDir, file), JSON.stringify({ testPath, messages }, null, 2));
  });
}
