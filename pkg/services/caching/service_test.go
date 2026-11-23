package caching

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana/pkg/services/contexthandler/ctxkey"
	contextmodel "github.com/grafana/grafana/pkg/services/contexthandler/model"
	"github.com/grafana/grafana/pkg/web"
	"github.com/stretchr/testify/require"
)

func TestWithQueryDataCaching(t *testing.T) {
	t.Run("caching is a no-op when service is nil", func(t *testing.T) {
		var s *CachingServiceClient
		req := backend.QueryDataRequest{}
		fakeResponse := &backend.QueryDataResponse{}
		response, err := s.WithQueryDataCaching(t.Context(), &req, func() (*backend.QueryDataResponse, error) {
			return fakeResponse, nil
		})
		require.NoError(t, err)
		require.Equal(t, fakeResponse, response)
	})

	t.Run("cache status is included in the response if a request context is available", func(t *testing.T) {
		fakeCachingService := NewFakeOSSCachingService()
		fakeCachingService.ReturnStatus = StatusMiss
		client := ProvideCachingServiceClient(fakeCachingService)

		req := backend.QueryDataRequest{}

		reqCtx := &contextmodel.ReqContext{
			Context: &web.Context{
				Resp: web.NewResponseWriter("", httptest.NewRecorder()),
			},
		}
		ctx := context.WithValue(t.Context(), ctxkey.Key{}, reqCtx)
		fakeResponse := &backend.QueryDataResponse{}
		response, err := client.WithQueryDataCaching(ctx, &req, func() (*backend.QueryDataResponse, error) {
			return fakeResponse, nil
		})
		require.NoError(t, err)
		require.Equal(t, fakeResponse, response)
		require.EqualValues(t, StatusMiss, reqCtx.Resp.Header().Get(XCacheHeader))
	})

	t.Run("caching can be used without a request context", func(t *testing.T) {
		fakeCachingService := NewFakeOSSCachingService()
		fakeCachingService.ReturnStatus = StatusMiss
		client := ProvideCachingServiceClient(fakeCachingService)

		req := backend.QueryDataRequest{}

		fakeResponse := &backend.QueryDataResponse{}
		// Using the default test context, no request context.
		response, err := client.WithQueryDataCaching(t.Context(), &req, func() (*backend.QueryDataResponse, error) {
			return fakeResponse, nil
		})
		require.NoError(t, err)
		require.Equal(t, fakeResponse, response)
	})
}

func TestWithCallResourceCaching(t *testing.T) {
	t.Run("caching is a no-op when service is nil", func(t *testing.T) {
		var s *CachingServiceClient
		req := backend.CallResourceRequest{}
		fakeErr := errors.New("oops")
		err := s.WithCallResourceCaching(t.Context(), &req, nil, func(backend.CallResourceResponseSender) error {
			return fakeErr
		})
		require.ErrorIs(t, err, fakeErr)
	})

	t.Run("cache status is included in the response if a request context is available", func(t *testing.T) {
		fakeCachingService := NewFakeOSSCachingService()
		fakeCachingService.ReturnStatus = StatusMiss
		client := ProvideCachingServiceClient(fakeCachingService)

		req := backend.CallResourceRequest{}

		reqCtx := &contextmodel.ReqContext{
			Context: &web.Context{
				Resp: web.NewResponseWriter("", httptest.NewRecorder()),
			},
		}
		ctx := context.WithValue(t.Context(), ctxkey.Key{}, reqCtx)
		sender := func(*backend.CallResourceResponse) error {
			return nil
		}
		var fakeErr = errors.New("oops")
		err := client.WithCallResourceCaching(ctx, &req, backend.CallResourceResponseSenderFunc(sender), func(backend.CallResourceResponseSender) error {
			return fakeErr
		})
		require.ErrorIs(t, err, fakeErr)
		require.EqualValues(t, StatusMiss, reqCtx.Resp.Header().Get(XCacheHeader))
	})

	t.Run("caching can be used without a request context", func(t *testing.T) {
		fakeCachingService := NewFakeOSSCachingService()
		fakeCachingService.ReturnStatus = StatusMiss
		client := ProvideCachingServiceClient(fakeCachingService)

		req := backend.CallResourceRequest{}

		sender := func(*backend.CallResourceResponse) error {
			return nil
		}
		var fakeErr = errors.New("oops")
		// Using the default test context, no request context.
		err := client.WithCallResourceCaching(t.Context(), &req, backend.CallResourceResponseSenderFunc(sender), func(_ backend.CallResourceResponseSender) error {
			return fakeErr
		})
		require.ErrorIs(t, err, fakeErr)
	})
}

func TestWithQueryDataCachingAsyncResults(t *testing.T) {
	origShouldCacheQuery := ShouldCacheQuery
	t.Cleanup(func() {
		ShouldCacheQuery = origShouldCacheQuery
	})

	reqCtx := &contextmodel.ReqContext{
		Context: &web.Context{
			Resp: web.NewResponseWriter("", httptest.NewRecorder()),
		},
	}
	ctx := context.WithValue(t.Context(), ctxkey.Key{}, reqCtx)
	req := &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{
			DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{Type: "grafana-athena-datasource"},
		},
	}

	t.Run("finished queries are cached", func(t *testing.T) {
		ShouldCacheQuery = func(*backend.QueryDataResponse) bool { return true }
		updated := false
		fakeCachingService := NewFakeOSSCachingService()
		fakeCachingService.ReturnQueryResponse = CachedQueryDataResponse{
			UpdateCacheFn: func(context.Context, *backend.QueryDataResponse) {
				updated = true
			},
		}
		client := ProvideCachingServiceClient(fakeCachingService)
		_, err := client.WithQueryDataCaching(ctx, req, func() (*backend.QueryDataResponse, error) {
			return &backend.QueryDataResponse{}, nil
		})
		require.NoError(t, err)
		require.True(t, updated)
	})

	t.Run("running async queries are not cached", func(t *testing.T) {
		ShouldCacheQuery = func(*backend.QueryDataResponse) bool { return false }
		updated := false
		fakeCachingService := NewFakeOSSCachingService()
		fakeCachingService.ReturnQueryResponse = CachedQueryDataResponse{
			UpdateCacheFn: func(context.Context, *backend.QueryDataResponse) {
				updated = true
			},
		}
		client := ProvideCachingServiceClient(fakeCachingService)
		_, err := client.WithQueryDataCaching(ctx, req, func() (*backend.QueryDataResponse, error) {
			return &backend.QueryDataResponse{}, nil
		})
		require.NoError(t, err)
		require.False(t, updated)
	})
}

func TestGetDatasourceType(t *testing.T) {
	t.Parallel()

	require.Equal(t, "unknown", getDatasourceType(backend.PluginContext{}))
	require.Equal(t, "name", getDatasourceType(backend.PluginContext{
		DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{
			Name: "name",
		},
	}))
}
