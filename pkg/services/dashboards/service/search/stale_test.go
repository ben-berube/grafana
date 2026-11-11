package dashboardsearch

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/apps/dashboard/pkg/apis/dashboard/v0alpha1"
	common "github.com/grafana/grafana/pkg/apimachinery/apis/common/v0alpha1"
	"github.com/grafana/grafana/pkg/storage/unified/resource"
	"github.com/grafana/grafana/pkg/storage/unified/resourcepb"
)

func TestIsDashboardStale(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		updated int64
		want    bool
	}{
		{name: "missing timestamp", updated: 0, want: false},
		{name: "updated today", updated: now.Add(-time.Hour).UnixMilli(), want: false},
		{name: "updated exactly 90 days ago", updated: now.Add(-StaleDashboardThreshold).UnixMilli(), want: false},
		{name: "updated more than 90 days ago", updated: now.Add(-StaleDashboardThreshold - time.Second).UnixMilli(), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsDashboardStale(tt.updated, now))
		})
	}
}

func TestAnnotateStale(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	old := now.Add(-StaleDashboardThreshold - 24*time.Hour).UnixMilli()
	fresh := now.Add(-24 * time.Hour).UnixMilli()

	hits := []v0alpha1.DashboardHit{
		dashboardHit("old", old),
		dashboardHit("fresh", fresh),
		{Resource: "folders", Name: "folder", Title: "Folder"},
		{Resource: v0alpha1.DASHBOARD_RESOURCE, Name: "unknown", Title: "No timestamp"},
	}

	AnnotateStale(hits, now)

	require.True(t, hitIsStale(hits[0]))
	require.False(t, hitIsStale(hits[1]))
	require.False(t, hitIsStale(hits[2]))
	require.False(t, hitIsStale(hits[3]))
	require.NotNil(t, hits[2].Field)
}

func TestSearchStaleDashboards(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	old := now.Add(-StaleDashboardThreshold - time.Hour).UnixMilli()
	fresh := now.Add(-time.Hour).UnixMilli()

	pages := []*resourcepb.ResourceSearchResponse{
		searchPage(3, row("old-1", "Old one", old), row("fresh", "Fresh", fresh)),
		searchPage(3, row("old-2", "Old two", old)),
	}
	var calls int
	fetch := func(_ context.Context, req *resourcepb.ResourceSearchRequest) (*resourcepb.ResourceSearchResponse, error) {
		require.Equal(t, int64(stalePageSize), req.Limit)
		require.Contains(t, []int64{0, 2}, req.Offset)
		if calls >= len(pages) {
			return searchPage(3), nil
		}
		page := pages[calls]
		calls++
		return page, nil
	}

	results, err := SearchStaleDashboards(context.Background(), &resourcepb.ResourceSearchRequest{
		Limit:  50,
		Offset: 0,
	}, fetch, now)
	require.NoError(t, err)
	require.Equal(t, int64(2), results.TotalHits)
	require.Len(t, results.Hits, 2)
	require.Equal(t, "old-1", results.Hits[0].Name)
	require.Equal(t, "old-2", results.Hits[1].Name)
	require.True(t, hitIsStale(results.Hits[0]))
}

func TestSearchStaleDashboardsAppliesOffset(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	old := now.Add(-StaleDashboardThreshold - time.Hour).UnixMilli()

	fetch := func(_ context.Context, _ *resourcepb.ResourceSearchRequest) (*resourcepb.ResourceSearchResponse, error) {
		return searchPage(2, row("old-1", "Old one", old), row("old-2", "Old two", old)), nil
	}

	results, err := SearchStaleDashboards(context.Background(), &resourcepb.ResourceSearchRequest{
		Limit:  1,
		Offset: 1,
	}, fetch, now)
	require.NoError(t, err)
	require.Equal(t, int64(2), results.TotalHits)
	require.Equal(t, int64(1), results.Offset)
	require.Equal(t, []string{"old-2"}, names(results.Hits))
}

func TestSearchStaleDashboardsFetchError(t *testing.T) {
	_, err := SearchStaleDashboards(context.Background(), &resourcepb.ResourceSearchRequest{}, func(context.Context, *resourcepb.ResourceSearchRequest) (*resourcepb.ResourceSearchResponse, error) {
		return nil, errors.New("search down")
	}, time.Now())
	require.Error(t, err)
}

func dashboardHit(name string, updated int64) v0alpha1.DashboardHit {
	field := &common.Unstructured{}
	field.Set(resource.SEARCH_FIELD_UPDATED, updated)
	return v0alpha1.DashboardHit{
		Resource: v0alpha1.DASHBOARD_RESOURCE,
		Name:     name,
		Title:    name,
		Field:    field,
	}
}

func searchPage(total int64, rows ...*resourcepb.ResourceTableRow) *resourcepb.ResourceSearchResponse {
	return &resourcepb.ResourceSearchResponse{
		TotalHits: total,
		Results: &resourcepb.ResourceTable{
			Columns: []*resourcepb.ResourceTableColumnDefinition{
				{Name: resource.SEARCH_FIELD_TITLE, Type: resourcepb.ResourceTableColumnDefinition_STRING},
				{Name: resource.SEARCH_FIELD_UPDATED, Type: resourcepb.ResourceTableColumnDefinition_INT64},
			},
			Rows: rows,
		},
	}
}

func row(name, title string, updated int64) *resourcepb.ResourceTableRow {
	return &resourcepb.ResourceTableRow{
		Key: &resourcepb.ResourceKey{Name: name, Resource: v0alpha1.DASHBOARD_RESOURCE},
		Cells: [][]byte{
			[]byte(title),
			[]byte(strconv.FormatInt(updated, 10)),
		},
	}
}

func names(hits []v0alpha1.DashboardHit) []string {
	out := make([]string, len(hits))
	for i, hit := range hits {
		out[i] = hit.Name
	}
	return out
}
