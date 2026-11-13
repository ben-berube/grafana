// Runs the same tests as jest.config.js, but with every import of react and react-dom
// (including the ones inside node_modules) resolved to the React 19 alias packages.
// Units migrate onto this runtime one at a time; see scripts/react19/allowlist.json.
//
// REACT_RUNTIME=18 keeps React 18 but still records console output, so a console baseline can
// be checked against what the same tests already print on React 18.
const os = require('node:os');
const path = require('node:path');

const base = require('./jest.config');

const runtime = process.env.REACT_RUNTIME ?? '19';
const react19 = path.resolve(__dirname, 'node_modules/react-19');
const reactDom19 = path.resolve(__dirname, 'node_modules/react-dom-19');

module.exports = {
  ...base,
  // Keep a separate transform cache per runtime so switching never serves a stale module graph.
  cacheDirectory: path.join(os.tmpdir(), `jest-react${runtime}`),
  moduleNameMapper: {
    ...(runtime === '19' && {
      '^react$': react19,
      '^react/(.*)$': `${react19}/$1`,
      '^react-dom$': reactDom19,
      '^react-dom/(.*)$': `${reactDom19}/$1`,
    }),
    ...base.moduleNameMapper,
  },
  setupFilesAfterEnv: [...base.setupFilesAfterEnv, './public/test/react19/consoleRecorder.ts'],
};
