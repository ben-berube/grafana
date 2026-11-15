import { ContextSrv, RedirectToUrlKey } from './context_srv';

describe('ContextSrv.setRedirectToUrl', () => {
  const originalLocation = window.location;

  beforeEach(() => {
    sessionStorage.clear();
  });

  afterEach(() => {
    sessionStorage.clear();
    Object.defineProperty(window, 'location', { configurable: true, value: originalLocation });
  });

  it('stores the current path so login can return the user there', () => {
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: {
        href: 'http://grafana.local/d/abc?orgId=1',
        origin: 'http://grafana.local',
      },
    });

    const srv = new ContextSrv();
    srv.setRedirectToUrl();

    expect(sessionStorage.getItem(RedirectToUrlKey)).toBe(encodeURIComponent('/d/abc?orgId=1'));
  });
});
