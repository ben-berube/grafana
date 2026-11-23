---
name: react19-parity
description: >-
  Gate for the React 18 to React 19 strangler migration in the Grafana repo. Runs the same
  parity checks (Jest on both runtimes, console budget, types, e2e on both bundles, Storybook
  visuals) before a unit joins the React 19 allowlist or a React 18 seam is deleted. Use when
  migrating a feature, plugin, or package to React 19, fixing a React 19 test failure, adding
  to scripts/react19/allowlist.json, or removing any part of the React 18 path.
---

# React 19 parity

The migration grows React 19 one unit at a time behind seams that already exist, then deletes the React 18 side of each seam. This skill is the only way a unit joins the allowlist or a seam is removed. Run every step that applies, in order, and report with the format at the end. For generic hygiene (lint, prettier, coverage of new code) also run the checks in `AGENTS.md`; this skill does not repeat them.

## The seams

| Seam | React 18 side | React 19 side |
| --- | --- | --- |
| Browser bundle | `assets-manifest.json` | `assets-manifest-react19.json`, chosen per request by the `react19` flag in `pkg/api/webassets/webassets.go` |
| Jest | `jest.config.js` | `jest.config.react19.js` (maps `react` and `react-dom` to the `react-19` alias packages) |
| Types | `@types/react` 18 | `tsconfig.react19.json` |
| Packages | peer ranges `^18.0.0` | peer ranges `^18.0.0 \|\| ^19.0.0` |

A **unit** is one entry from `node scripts/react19/units.js --remaining`: a feature directory, a core plugin, a `public/app` top-level directory, or a `packages/*` workspace. Decoupled plugin workspaces (loki, mysql, and others listed in `scripts/react19/units.js`) are migrated with their own Jest config, using the same steps.

## Parity means

A unit has parity when all of these hold. "Looks fine" is not parity.

1. Its Jest files pass on React 18 and on React 19, with no runtime-specific branches in tests or source.
2. The React 19 run emits no `console.error` or `console.warn` that is not in the unit's committed baseline. React 19 warnings (`act(...)`, keys, ref cleanup) are fixed, not baselined. Baseline only messages that already appear on React 18.
3. `yarn typecheck` passes, and the unit is clean under `tsconfig.react19.json` once that exists.
4. Its e2e projects pass against a server serving the React 18 bundle and against one serving React 19, and the `react-runtime` project proves which bundle each run actually got.
5. For `packages/grafana-ui`, Storybook screenshots match across runtimes or the difference is accepted in the report.

## Unit mode

Input: a unit path, for example `public/app/features/playlist`.

### Step 0: Map the unit

```sh
node scripts/react19/units.js --remaining | rg '^<unit>$'      # must be a real, unmigrated unit
find <unit> -name '*.test.ts*' | wc -l
rg -l '<unit>' playwright.config.ts e2e-playwright             # e2e projects that exercise it
```

Map the unit to Playwright projects by what the UI does, not only by path: alerting goes to `alerting`, dashboards and dashboard-scene to `dashboards`, `dashboard-new-layouts`, and `dashboard-cujs`, panels to `panels`, plugins to `plugin-e2e` and `test-plugins`. Every unit also gets `smoke`.

### Step 1: Jest on both runtimes

```sh
yarn jest --no-watch <unit>
node scripts/react19/run.js --record <unit>
```

`jest.config.react19.js` fails every test file if React 19 is not actually loaded, so a green run is a real React 19 run. Fix failures in source or tests until both runs are green. Common React 19 causes:

- State updates now batch everywhere, so assertions right after an event need `await` or `findBy*`.
- A ref callback that returns a value is treated as a cleanup function. Use a block body.
- `useRef()` with no argument and the global `JSX` namespace are type errors under React 19 types; use `useRef<T>(null)` and `React.JSX`.
- Libraries that read `element.type` or use `react-is` 18 may not recognize React 19 elements. Pin or upgrade the library rather than mocking around it.
- Class components fail only through removed APIs. Convert one only when it fails, and keep the diff to that component.

### Step 2: Console budget

```sh
node scripts/react19/console-budget.js check <unit>
```

New messages fail. To baseline the messages that remain, record the same unit on React 18 and update:

```sh
REACT_RUNTIME=18 node scripts/react19/run.js --record <unit>
node scripts/react19/console-budget.js update <unit>
```

`update` keeps only messages that also appear in the React 18 recording and fails, listing them, if any appear only on React 19.

Commit `public/test/react19-baselines/<unit>.json`. Never update a baseline to hide a message React 19 introduced.

### Step 3: Types

```sh
yarn typecheck
```

When `tsconfig.react19.json` exists, also run `yarn tsc -p tsconfig.react19.json --noEmit` and fix every error under the unit's path.

### Step 4: e2e on both bundles

Both bundles must exist: `yarn build` writes `assets-manifest.json` and `assets-manifest-react19.json`. A checkout that only ran `yarn start` has only the React 19 bundle, and the React 18 run will show "Grafana has failed to load its application files".

```sh
GRAFANA_REACT_RUNTIME=18 yarn e2e:playwright --project react-runtime --project smoke --project <mapped projects>
GRAFANA_REACT_RUNTIME=19 yarn e2e:playwright --project react-runtime --project smoke --project <mapped projects>
```

`GRAFANA_REACT_RUNTIME` is read by `scripts/grafana-server/start-server` and by the `react-runtime` spec. Against an already running server (`GRAFANA_URL=...`), set the flag on that server first and pass the same value, or the `react-runtime` project fails on the mismatch. That failure is the point.

### Step 5: Storybook visuals (packages/grafana-ui only)

```sh
yarn e2e:playwright:storybook
```

Run once per runtime and compare. Accept a pixel difference only with a written reason in the report.

### Step 6: Allowlist

Add the unit to `scripts/react19/allowlist.json` (keep it sorted), then:

```sh
node scripts/react19/check-allowlist.js
node scripts/react19/units.js
```

The CI job `React 19 allowlist (Jest)` in `.github/workflows/fieldsphere-ci.yml` runs the whole allowlist on React 19 and fails if a unit was removed. A unit leaves the allowlist never; fix the regression instead.

## Seam-removal mode

Input: one seam name, removed in this order: `jest`, `dependency`, `backend`, `toggle`, `cleanup`. Frontend and backend removals ship as separate PRs.

Before removing any seam:

```sh
node scripts/react19/units.js --remaining      # must print nothing
node scripts/react19/run.js --record
node scripts/react19/console-budget.js check
```

Then run Step 4 with `GRAFANA_REACT_RUNTIME=19` on `smoke`, `dashboards`, `panels`, and `alerting`, and remove the seam:

- **jest**: point `jest.config.js` at React 19 (fold in the `moduleNameMapper` and the recorder), delete `jest.config.react19.js`, and make the CI job run the full suite with the console budget.
- **dependency**: set `react` and `react-dom` to 19.2.x and `@types/react`, `@types/react-dom` to 19; delete the `react-19` and `react-dom-19` aliases; delete the `NormalModuleReplacementPlugin` block, the `-react19` names, and the conditional `clean` in `scripts/webpack/webpack.{common,dev,prod}.ts`; drop `build:react19` and `--env react19=1` from `package.json`; delete `tsconfig.react19.json`. One manifest remains.
- **backend**: remove the manifest choice from `pkg/api/webassets/webassets.go` and `pkg/services/frontend/webassets/webassets.go` and their tests, the second manifest write in `pkg/tests/testinfra/testinfra.go`, and the React 19 manifest lines in `devenv/frontend-service/Tiltfile`.
- **toggle**: delete `react19` from `pkg/services/featuremgmt/registry.go` and `devenv/frontend-service/goff-flags.yaml`, run `make gen-feature-toggles`, remove `GRAFANA_REACT_RUNTIME` from `start-server` and the `react-runtime` project, and update `docs/sources/upgrade-guide/`.
- **cleanup**: remove the `react-19` and `react-dom-19` ignores in `knip.config.ts`, delete `scripts/react19/`, `public/test/react19/`, `public/test/react19-baselines/`, and this skill.

After each removal, rerun `yarn typecheck`, the full Jest suite, and the four e2e projects.

## Report format

End every run with:

- Unit or seam, and the commit it was checked at.
- One line per step: passed, failed, or skipped with the reason (for example "Step 5 skipped, not grafana-ui").
- Console baseline: entries added, with the React 18 evidence for each.
- Source changes made for React 19, one line each.
- Anything the unit still does differently on React 19.

Do not report a unit as migrated while a step that applies is failing or unrun. Do not `git push` without approval, per `AGENTS.md`.
