// The migration units for the React 19 strangler. A unit is a directory whose Jest tests move
// onto React 19 together. Phase 4 (deleting the React 18 path) is allowed only when the
// allowlist covers every unit returned by listUnits().
const fs = require('node:fs');
const path = require('node:path');

const root = path.resolve(__dirname, '../..');

// Decoupled plugins have their own Jest config and are migrated as workspaces, not as units.
const decoupledPlugins = new Set([
  'public/app/plugins/datasource/azuremonitor',
  'public/app/plugins/datasource/grafana-postgresql-datasource',
  'public/app/plugins/datasource/grafana-pyroscope-datasource',
  'public/app/plugins/datasource/grafana-testdata-datasource',
  'public/app/plugins/datasource/influxdb',
  'public/app/plugins/datasource/graphite',
  'public/app/plugins/datasource/jaeger',
  'public/app/plugins/datasource/loki',
  'public/app/plugins/datasource/mssql',
  'public/app/plugins/datasource/mysql',
]);

const testFile = /\.test\.(jsx?|tsx?)$/;

function hasTests(dir) {
  const stack = [path.join(root, dir)];
  while (stack.length) {
    const current = stack.pop();
    for (const entry of fs.readdirSync(current, { withFileTypes: true })) {
      if (entry.name === 'node_modules') {
        continue;
      }
      const full = path.join(current, entry.name);
      if (entry.isDirectory()) {
        stack.push(full);
      } else if (testFile.test(entry.name)) {
        return true;
      }
    }
  }
  return false;
}

function children(dir) {
  return fs
    .readdirSync(path.join(root, dir), { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => `${dir}/${entry.name}`);
}

function listUnits() {
  const units = [];
  for (const dir of children('public/app')) {
    if (dir === 'public/app/features' || dir === 'public/app/plugins') {
      continue;
    }
    units.push(dir);
  }
  units.push(...children('public/app/features'));
  for (const kind of ['panel', 'datasource']) {
    units.push(...children(`public/app/plugins/${kind}`).filter((dir) => !decoupledPlugins.has(dir)));
  }
  units.push(...children('packages'), 'public/test', 'scripts/tests');
  return units.filter(hasTests).sort();
}

function readAllowlist(file = path.join(__dirname, 'allowlist.json')) {
  return JSON.parse(fs.readFileSync(file, 'utf8')).units;
}

module.exports = { root, listUnits, readAllowlist, decoupledPlugins };

if (require.main === module) {
  const units = listUnits();
  const allowed = new Set(readAllowlist());
  const remaining = units.filter((unit) => !allowed.has(unit));
  if (process.argv.includes('--remaining')) {
    console.log(remaining.join('\n'));
  } else {
    console.log(`${allowed.size} of ${units.length} units run on React 19. ${remaining.length} remaining.`);
  }
}
