import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { type TimeOption } from '@grafana/data';

import { TimeRangeList } from './TimeRangeList';

const options: TimeOption[] = [
  { from: 'now-5m', to: 'now', display: 'Last 5 minutes' },
  { from: 'now-15m', to: 'now', display: 'Last 15 minutes' },
  { from: 'now-6h', to: 'now', display: 'Last 6 hours' },
];

describe('TimeRangeList', () => {
  it('selects the quick range that was clicked', async () => {
    const onChange = jest.fn();
    render(<TimeRangeList options={options} onChange={onChange} />);

    await userEvent.click(screen.getByText('Last 5 minutes'));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(options[0]);
  });

  it('selects the last quick range when it is clicked', async () => {
    const onChange = jest.fn();
    render(<TimeRangeList options={options} onChange={onChange} />);

    await userEvent.click(screen.getByText('Last 6 hours'));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(options[2]);
  });

  it('selects the focused quick range when Enter is pressed', async () => {
    const user = userEvent.setup();
    const onChange = jest.fn();
    render(<TimeRangeList options={options} onChange={onChange} />);

    screen.getByRole('checkbox', { name: 'Last 5 minutes' }).focus();
    await user.keyboard('{ArrowDown}{Enter}');

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(options[1]);
  });
});
