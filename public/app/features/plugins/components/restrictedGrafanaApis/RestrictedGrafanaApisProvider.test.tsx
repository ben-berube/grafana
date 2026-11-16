import { renderHook } from '@testing-library/react';
import { type ReactNode } from 'react';

import { useRestrictedGrafanaApis } from '@grafana/data';
import { config } from '@grafana/runtime';

import { RestrictedGrafanaApisProvider } from './RestrictedGrafanaApisProvider';

const allowListedPluginId = 'grafana-test-app';

describe('RestrictedGrafanaApisProvider', () => {
  beforeEach(() => {
    config.bootData.settings.pluginRestrictedAPIsAllowList = {
      alertingAlertRuleFormSchema: [allowListedPluginId],
      dashboardMutationAPI: [allowListedPluginId],
    };
    config.bootData.settings.pluginRestrictedAPIsBlockList = undefined;
  });

  afterEach(() => {
    config.bootData.settings.pluginRestrictedAPIsAllowList = undefined;
    config.bootData.settings.pluginRestrictedAPIsBlockList = undefined;
  });

  it('exposes the restricted APIs to an allow-listed plugin', () => {
    const { result } = renderHook(() => useRestrictedGrafanaApis(), {
      wrapper: ({ children }: { children: ReactNode }) => (
        <RestrictedGrafanaApisProvider pluginId={allowListedPluginId}>{children}</RestrictedGrafanaApisProvider>
      ),
    });

    expect(result.current.alertingAlertRuleFormSchema).toBeDefined();
    expect(result.current.dashboardMutationAPI).toEqual(
      expect.objectContaining({
        execute: expect.any(Function),
        getPayloadSchema: expect.any(Function),
        getAvailableCommands: expect.any(Function),
      })
    );
  });

  it('does not expose restricted APIs to a plugin that is not allow-listed', () => {
    const { result } = renderHook(() => useRestrictedGrafanaApis(), {
      wrapper: ({ children }: { children: ReactNode }) => (
        <RestrictedGrafanaApisProvider pluginId="other-plugin">{children}</RestrictedGrafanaApisProvider>
      ),
    });

    expect(result.current.alertingAlertRuleFormSchema).toBeUndefined();
    expect(result.current.dashboardMutationAPI).toBeUndefined();
  });
});
