import { render, screen } from '@testing-library/react';

import { createDataFrame, dateTime, EventBusSrv, FieldType, LoadingState, type PanelProps } from '@grafana/data';
import {
  BigValueColorMode,
  BigValueGraphMode,
  BigValueJustifyMode,
  BigValueTextMode,
  PercentChangeColorMode,
  VizOrientation,
} from '@grafana/schema';

import { StatPanel } from './StatPanel';
import { type Options } from './panelcfg.gen';

function buildProps(graphMode: BigValueGraphMode): PanelProps<Options> {
  const timeRange = {
    from: dateTime('2026-11-13T06:00:00Z'),
    to: dateTime('2026-11-13T12:00:00Z'),
    raw: { from: 'now-6h', to: 'now' },
  };

  return {
    id: 1,
    title: 'p99 checkout latency',
    timeRange,
    timeZone: 'utc',
    width: 400,
    height: 200,
    transparent: false,
    renderCounter: 0,
    eventBus: new EventBusSrv(),
    options: {
      reduceOptions: { calcs: ['lastNotNull'], values: false },
      orientation: VizOrientation.Auto,
      textMode: BigValueTextMode.Auto,
      wideLayout: true,
      colorMode: BigValueColorMode.Value,
      graphMode,
      justifyMode: BigValueJustifyMode.Auto,
      showPercentChange: false,
      percentChangeColorMode: PercentChangeColorMode.Standard,
    },
    fieldConfig: { defaults: {}, overrides: [] },
    data: {
      state: LoadingState.Done,
      timeRange,
      series: [
        createDataFrame({
          refId: 'A',
          fields: [
            { name: 'time', type: FieldType.time, values: [1, 2, 3, 4], config: {} },
            { name: 'latency', type: FieldType.number, values: [10, 12, 11, 14.2], config: {} },
          ],
        }),
      ],
    },
    replaceVariables: (value: string) => value,
    onChangeTimeRange: jest.fn(),
    onFieldConfigChange: jest.fn(),
    onOptionsChange: jest.fn(),
  };
}

describe('StatPanel sparkline', () => {
  it.each([BigValueGraphMode.Area, BigValueGraphMode.Line])('draws the trend when graph mode is %s', (graphMode) => {
    render(<StatPanel {...buildProps(graphMode)} />);

    expect(screen.getByText('14.2')).toBeInTheDocument();
    expect(screen.getByTestId('uplot-main-div')).toBeInTheDocument();
  });

  it('keeps the value and omits the trend when graph mode is none', () => {
    render(<StatPanel {...buildProps(BigValueGraphMode.None)} />);

    expect(screen.getByText('14.2')).toBeInTheDocument();
    expect(screen.queryByTestId('uplot-main-div')).not.toBeInTheDocument();
  });
});
