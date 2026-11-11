import { render, screen } from 'test/test-utils';

import { wellFormedDashboard } from '../fixtures/dashboardsTreeItem.fixture';

import { NameCell } from './NameCell';

function renderName(stale: boolean, title: string) {
  const item = wellFormedDashboard(1, {}, { stale, title, url: '/d/abc' });
  const props = {
    row: { original: item },
    onFolderClick: jest.fn(),
  } as unknown as React.ComponentProps<typeof NameCell>;

  render(<NameCell {...props} />);
}

describe('NameCell', () => {
  it('shows a Stale badge on a stale dashboard', () => {
    renderName(true, 'Old board');

    expect(screen.getByText('Old board')).toBeInTheDocument();
    expect(screen.getByText('Stale')).toBeInTheDocument();
  });

  it('does not show a Stale badge on a dashboard that was updated recently', () => {
    renderName(false, 'New board');

    expect(screen.getByText('New board')).toBeInTheDocument();
    expect(screen.queryByText('Stale')).not.toBeInTheDocument();
  });
});
