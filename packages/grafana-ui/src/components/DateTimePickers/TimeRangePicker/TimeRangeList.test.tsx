import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { type TimeOption } from '@grafana/data';

import { TimeRangeList } from './TimeRangeList';

const options: TimeOption[] = [
  { from: 'now-12h', to: 'now', display: 'Last 12 hours' },
  { from: 'now-24h', to: 'now', display: 'Last 24 hours' },
  { from: 'now-2d', to: 'now', display: 'Last 2 days' },
];

describe('TimeRangeList', () => {
  it('selects the clicked quick range instead of the following one', async () => {
    const user = userEvent.setup();
    const onChange = jest.fn();
    render(<TimeRangeList options={options} onChange={onChange} />);

    await user.click(screen.getByText('Last 24 hours'));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(options[1]);
  });

  it('selects the last quick range when that row is clicked', async () => {
    const user = userEvent.setup();
    const onChange = jest.fn();
    render(<TimeRangeList options={options} onChange={onChange} />);

    await user.click(screen.getByText('Last 2 days'));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(options[2]);
  });

  it('selects the keyboard-focused quick range when Enter is pressed', async () => {
    const user = userEvent.setup();
    const onChange = jest.fn();
    render(<TimeRangeList options={options} onChange={onChange} />);

    await user.tab();
    await user.keyboard('{ArrowDown}');

    const radios = document.querySelectorAll<HTMLInputElement>('[data-role="item"]');
    expect(radios[1]).toHaveFocus();

    await user.keyboard('{Enter}');

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(options[1]);
  });
});
