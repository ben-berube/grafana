package dashboardsearch

import (
	"context"
	"time"

	"github.com/grafana/grafana/apps/dashboard/pkg/apis/dashboard/v0alpha1"
	common "github.com/grafana/grafana/pkg/apimachinery/apis/common/v0alpha1"
	"github.com/grafana/grafana/pkg/storage/unified/resource"
	"github.com/grafana/grafana/pkg/storage/unified/resourcepb"
)

// StaleDashboardThreshold is how long a dashboard can go without an update
// before browse and search mark it stale. Change this one value to move the cutoff.
const StaleDashboardThreshold = 90 * 24 * time.Hour

// The search index can return updated timestamps, but it cannot range-filter
// them. Stale filtering pages through results here and keeps the ones past the cutoff.
const (
	stalePageSize  = 100
	staleScanLimit = 5000
)

// IsDashboardStale reports whether updatedUnixMillis is older than StaleDashboardThreshold.
// A missing timestamp is not stale: we only badge dashboards we know were updated.
func IsDashboardStale(updatedUnixMillis int64, now time.Time) bool {
	if updatedUnixMillis <= 0 {
		return false
	}
	return now.Sub(time.UnixMilli(updatedUnixMillis)) > StaleDashboardThreshold
}

// AnnotateStale sets field.stale on every hit. Folders are never stale.
// The flag is set on every hit so the first row still carries the column.
func AnnotateStale(hits []v0alpha1.DashboardHit, now time.Time) {
	for i := range hits {
		stale := false
		if hits[i].Resource == v0alpha1.DASHBOARD_RESOURCE {
			stale = IsDashboardStale(updatedMillis(hits[i]), now)
		}
		if hits[i].Field == nil {
			hits[i].Field = &common.Unstructured{}
		}
		hits[i].Field.Set("stale", stale)
	}
}

func updatedMillis(hit v0alpha1.DashboardHit) int64 {
	if hit.Field == nil || hit.Field.Object == nil {
		return 0
	}
	switch n := hit.Field.Object[resource.SEARCH_FIELD_UPDATED].(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func hitIsStale(hit v0alpha1.DashboardHit) bool {
	if hit.Field == nil || hit.Field.Object == nil {
		return false
	}
	stale, ok := hit.Field.Object["stale"].(bool)
	return ok && stale
}

// SearchPageFunc loads one page of search hits.
type SearchPageFunc func(ctx context.Context, req *resourcepb.ResourceSearchRequest) (*resourcepb.ResourceSearchResponse, error)

// SearchStaleDashboards pages through search results and returns dashboards
// whose updated time is older than StaleDashboardThreshold. offset and limit
// apply to that filtered list.
func SearchStaleDashboards(ctx context.Context, req *resourcepb.ResourceSearchRequest, fetch SearchPageFunc, now time.Time) (v0alpha1.SearchResults, error) {
	empty := v0alpha1.SearchResults{Hits: []v0alpha1.DashboardHit{}}
	if req == nil || fetch == nil {
		return empty, nil
	}

	wantLimit := req.Limit
	if wantLimit <= 0 {
		wantLimit = 50
	}
	wantOffset := req.Offset
	if wantOffset < 0 {
		wantOffset = 0
	}

	var stale []v0alpha1.DashboardHit
	var scanned int64
	for scanned < staleScanLimit {
		pageReq := *req
		pageReq.Limit = stalePageSize
		pageReq.Offset = scanned
		pageReq.Page = scanned/stalePageSize + 1

		result, err := fetch(ctx, &pageReq)
		if err != nil {
			return v0alpha1.SearchResults{}, err
		}
		parsed, err := ParseResults(result, 0)
		if err != nil {
			return v0alpha1.SearchResults{}, err
		}

		rows := 0
		total := int64(0)
		if result != nil {
			total = result.TotalHits
			if result.Results != nil {
				rows = len(result.Results.Rows)
			}
		}

		AnnotateStale(parsed.Hits, now)
		for _, hit := range parsed.Hits {
			if hit.Resource == v0alpha1.DASHBOARD_RESOURCE && hitIsStale(hit) {
				stale = append(stale, hit)
			}
		}

		scanned += int64(rows)
		if rows == 0 || scanned >= total {
			break
		}
	}

	totalStale := int64(len(stale))
	if wantOffset > totalStale {
		wantOffset = totalStale
	}
	end := wantOffset + wantLimit
	if end > totalStale {
		end = totalStale
	}
	page := append([]v0alpha1.DashboardHit(nil), stale[wantOffset:end]...)
	if page == nil {
		page = []v0alpha1.DashboardHit{}
	}

	return v0alpha1.SearchResults{
		Offset:    wantOffset,
		TotalHits: totalStale,
		Hits:      page,
	}, nil
}
