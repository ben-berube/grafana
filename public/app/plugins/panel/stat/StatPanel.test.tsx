import { render, screen } from '@testing-library/react';
import { uniqueId } from 'lodash';

import {
  dateMath,
  dateTime,
  type EventBus,
  FieldType,
  LoadingState,
  type PanelProps,
  type TimeRange,
  toDataFrame,
} from '@grafana/data';
import { BigValueGraphMode, VizOrientation } from '@grafana/schema';

import { StatPanel } from './StatPanel';
import { defaultOptions, type Options } from './panelcfg.gen';

jest.mock('uplot', () => {
  return jest.fn().mockImplementation(() => ({
    setData: jest.fn(),
    setSize: jest.fn(),
    destroy: jest.fn(),
  }));
});

describe('StatPanel', () => {
  it.each([BigValueGraphMode.Area, BigValueGraphMode.Line])('draws a trend when graph mode is %s', (graphMode) => {
    render(<StatPanel {...buildPanelProps(graphMode)} />);

    expect(screen.getByText('30')).toBeInTheDocument();
    expect(screen.getByTestId('uplot-main-div')).toBeInTheDocument();
  });

  it('does not draw a trend when graph mode is none', () => {
    render(<StatPanel {...buildPanelProps(BigValueGraphMode.None)} />);

    expect(screen.getByText('30')).toBeInTheDocument();
    expect(screen.queryByTestId('uplot-main-div')).not.toBeInTheDocument();
  });
});

function buildPanelProps(graphMode: BigValueGraphMode): PanelProps<Options> {
  const timeRange = createTimeRange();

  return {
    id: Number(uniqueId()),
    data: {
      series: [
        toDataFrame({
          fields: [
            { name: 'Time', type: FieldType.time, values: [1_000, 2_000, 3_000] },
            { name: 'p99', type: FieldType.number, values: [10, 20, 30] },
          ],
        }),
      ],
      state: LoadingState.Done,
      timeRange,
    },
    timeRange,
    timeZone: 'utc',
    options: {
      ...defaultOptions,
      graphMode,
      orientation: VizOrientation.Auto,
      reduceOptions: {
        calcs: ['lastNotNull'],
        values: false,
      },
    } as Options,
    transparent: false,
    title: 'p99 checkout latency',
    fieldConfig: {
      defaults: {},
      overrides: [],
    },
    renderCounter: 0,
    width: 300,
    height: 200,
    eventBus: {} as EventBus,
    onOptionsChange: jest.fn(),
    onFieldConfigChange: jest.fn(),
    onChangeTimeRange: jest.fn(),
    replaceVariables: (value: string) => value,
  };
}

function createTimeRange(): TimeRange {
  return {
    from: dateMath.parse('now-6h') || dateTime(),
    to: dateMath.parse('now') || dateTime(),
    raw: { from: 'now-6h', to: 'now' },
  };
}
