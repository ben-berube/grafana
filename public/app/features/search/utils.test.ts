import { type SearchQueryParams } from './types';
import { needsListLayout, parseRouteParams } from './utils';

describe('Search utils', () => {
  describe('parseRouteParams', () => {
    it('should remove all undefined keys', () => {
      const params: Partial<SearchQueryParams> = { sort: undefined, tag: undefined, query: 'test' };

      expect(parseRouteParams(params)).toEqual({
        query: 'test',
      });
    });

    it('should return tag as array, if present', () => {
      const params = { sort: undefined, tag: 'test', query: 'test' };
      expect(parseRouteParams(params)).toEqual({
        query: 'test',
        tag: ['test'],
      });

      const params2: Partial<SearchQueryParams> = { sort: undefined, tag: ['test'], query: 'test' };
      expect(parseRouteParams(params2)).toEqual({
        query: 'test',
        tag: ['test'],
      });
    });

    it('should return ownerReference as array, if present', () => {
      const params = { ownerReference: 'iam.grafana.app/v0alpha1/Team/test-team', query: 'test' };
      expect(parseRouteParams(params)).toEqual({
        query: 'test',
        ownerReference: ['iam.grafana.app/v0alpha1/Team/test-team'],
      });

      const params2: Partial<SearchQueryParams> = {
        ownerReference: ['iam.grafana.app/v0alpha1/Team/test-team'],
        query: 'test',
      };
      expect(parseRouteParams(params2)).toEqual({
        query: 'test',
        ownerReference: ['iam.grafana.app/v0alpha1/Team/test-team'],
      });
    });

    it('should prepend folder:{folder} to the query if folder is present', () => {
      expect(parseRouteParams({ folder: 'current' })).toEqual({
        folder: 'current',
        query: 'folder:current ',
      });
      // Prepend to exiting query
      const params: Partial<SearchQueryParams> = { query: 'test', folder: 'current' };
      expect(parseRouteParams(params)).toEqual({
        folder: 'current',
        query: 'folder:current test',
      });
    });

    it('reads stale=true from the url as a boolean', () => {
      expect(parseRouteParams({ stale: 'true' })).toEqual({ stale: true });
      expect(parseRouteParams({ stale: 'false' })).toEqual({ stale: false });
    });
  });

  describe('needsListLayout', () => {
    it('uses the list layout when the stale filter is on', () => {
      expect(needsListLayout({ stale: true })).toBe(true);
      expect(needsListLayout({})).toBeFalsy();
    });
  });
});
