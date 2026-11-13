import { expect, test } from '@playwright/test';

// Proves which React runtime the server actually handed the browser, so a "passes on both
// runtimes" e2e claim cannot quietly run twice on the same bundle. Run it first, with the same
// GRAFANA_REACT_RUNTIME the server was started with (see scripts/grafana-server/start-server).
const expected = process.env.GRAFANA_REACT_RUNTIME ?? '19';

declare global {
  interface Window {
    __reactRendererVersions?: string[];
  }
}

test.describe('React runtime', { tag: ['@react-runtime'] }, () => {
  test(`serves and boots React ${expected}`, async ({ page }) => {
    // React DOM registers each renderer with the devtools hook at startup, passing its version.
    await page.addInitScript(() => {
      window.__reactRendererVersions = [];
      Object.defineProperty(window, '__REACT_DEVTOOLS_GLOBAL_HOOK__', {
        configurable: true,
        value: {
          isDisabled: false,
          supportsFiber: true,
          renderers: new Map(),
          inject(renderer: { version?: string }) {
            window.__reactRendererVersions!.push(renderer.version ?? 'unknown');
            return window.__reactRendererVersions!.length;
          },
          checkDCE() {},
          onCommitFiberRoot() {},
          onCommitFiberUnmount() {},
          onPostCommitFiberRoot() {},
          onScheduleFiberRoot() {},
        },
      });
    });

    const response = await page.goto('/login');
    const html = (await response?.text()) ?? '';
    const scripts = [...html.matchAll(/src="([^"]*public\/build\/[^"]+\.js)"/g)].map((match) => match[1]);
    expect(scripts.length, 'the login page should load built scripts').toBeGreaterThan(0);

    const react19Bundles = scripts.filter((src) => src.includes('-react19.'));
    if (expected === '19') {
      expect(react19Bundles, 'every entry script should come from the React 19 build').toHaveLength(
        scripts.filter((src) => !src.endsWith('/boot.js')).length
      );
    } else {
      expect(react19Bundles, 'no entry script should come from the React 19 build').toHaveLength(0);
    }

    await expect
      .poll(() => page.evaluate(() => window.__reactRendererVersions?.length ?? 0), {
        message: 'React DOM should register a renderer',
      })
      .toBeGreaterThan(0);
    const versions = await page.evaluate(() => window.__reactRendererVersions ?? []);
    for (const version of versions) {
      expect(version.split('.')[0]).toBe(expected);
    }
  });
});
