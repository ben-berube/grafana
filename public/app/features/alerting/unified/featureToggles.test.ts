import { config } from '@grafana/runtime';

import { shouldAllowPermanentlyDeletingRules, shouldUseAlertingListViewV2 } from './featureToggles';
import { mockLocalStorage } from './mocks';
import { setPreviewToggle } from './previewToggles';
import * as misc from './utils/misc';

const localStorageMock = mockLocalStorage();

Object.defineProperty(window, 'localStorage', {
  value: localStorageMock,
  writable: true,
});

describe('featureToggles', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    localStorageMock.clear();
    config.featureToggles = {};
  });

  describe('shouldUseAlertingListViewV2', () => {
    describe('when alertingListViewV2 toggle is disabled', () => {
      beforeEach(() => {
        config.featureToggles.alertingListViewV2 = false;
      });

      it('should return false when no localStorage preference is set', () => {
        expect(shouldUseAlertingListViewV2()).toBe(false);
      });

      it('should return true when localStorage preference is true', () => {
        setPreviewToggle('alertingListViewV2', true);

        expect(shouldUseAlertingListViewV2()).toBe(true);
      });

      it('should return false when localStorage preference is false', () => {
        setPreviewToggle('alertingListViewV2', false);

        expect(shouldUseAlertingListViewV2()).toBe(false);
      });
    });

    describe('when alertingListViewV2 toggle is enabled', () => {
      beforeEach(() => {
        config.featureToggles.alertingListViewV2 = true;
      });

      it('should return true when no localStorage preference is set', () => {
        expect(shouldUseAlertingListViewV2()).toBe(true);
      });

      it('should return true when localStorage preference is true', () => {
        setPreviewToggle('alertingListViewV2', true);

        expect(shouldUseAlertingListViewV2()).toBe(true);
      });

      it('should return false when localStorage preference is false', () => {
        setPreviewToggle('alertingListViewV2', false);

        expect(shouldUseAlertingListViewV2()).toBe(false);
      });
    });
  });

  describe('shouldAllowPermanentlyDeletingRules', () => {
    beforeEach(() => {
      jest.spyOn(misc, 'isAdmin').mockReturnValue(true);
      config.featureToggles.alertingRuleRecoverDeleted = true;
      config.featureToggles.alertRuleRestore = true;
    });

    it('allows permanent delete for admins when restore is enabled', () => {
      expect(shouldAllowPermanentlyDeletingRules()).toBe(true);
    });

    it('does not allow permanent delete when alert rule restore is disabled', () => {
      config.featureToggles.alertRuleRestore = false;

      expect(shouldAllowPermanentlyDeletingRules()).toBe(false);
    });

    it('does not allow permanent delete when recovering deleted rules is disabled', () => {
      config.featureToggles.alertingRuleRecoverDeleted = false;

      expect(shouldAllowPermanentlyDeletingRules()).toBe(false);
    });

    it('does not allow permanent delete for non-admins', () => {
      jest.spyOn(misc, 'isAdmin').mockReturnValue(false);

      expect(shouldAllowPermanentlyDeletingRules()).toBe(false);
    });
  });
});
