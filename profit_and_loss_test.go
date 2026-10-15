package quickbooks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestFindPnLReportWithParams_SendsFilters(t *testing.T) {
	tests := []struct {
		name   string
		params PnLReportParams
		want   url.Values
	}{
		{
			name:   "dates only",
			params: PnLReportParams{StartDate: "2026-09-01", EndDate: "2026-09-30"},
			want: url.Values{
				"start_date":   {"2026-09-01"},
				"end_date":     {"2026-09-30"},
				"minorversion": {"65"},
			},
		},
		{
			name: "departments and classes are comma joined",
			params: PnLReportParams{
				StartDate:     "2026-09-01",
				EndDate:       "2026-09-30",
				DepartmentIds: []string{"1", "4"},
				ClassIds:      []string{"7", "8", "9"},
			},
			want: url.Values{
				"start_date":   {"2026-09-01"},
				"end_date":     {"2026-09-30"},
				"department":   {"1,4"},
				"class":        {"7,8,9"},
				"minorversion": {"65"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got url.Values
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.URL.Query()
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Header": map[string]string{"ReportName": "ProfitAndLoss"},
				})
			}))
			defer srv.Close()

			c := newTestClient(t, srv.URL)
			report, err := c.FindPnLReportWithParams(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("FindPnLReportWithParams: %v", err)
			}
			if report.Header.ReportName != "ProfitAndLoss" {
				t.Fatalf("report not decoded, got name %q", report.Header.ReportName)
			}
			if gotPath != "/v3/company/test-realm/reports/ProfitAndLoss" {
				t.Fatalf("path = %q, want the ProfitAndLoss report", gotPath)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("query = %v, want exactly %v (an empty filter must not be sent: it would drop untagged lines)", got, tt.want)
			}
			for key, want := range tt.want {
				if got.Get(key) != want[0] {
					t.Errorf("query[%s] = %q, want %q", key, got.Get(key), want[0])
				}
			}
		})
	}
}

func TestFindPnLReportWithParams_RejectsBadDates(t *testing.T) {
	c := &Client{}
	for _, params := range []PnLReportParams{
		{StartDate: "", EndDate: "2026-09-30"},
		{StartDate: "2026-09-01", EndDate: "09/30/2026"},
	} {
		if _, err := c.FindPnLReportWithParams(context.Background(), params); err == nil {
			t.Errorf("params %+v: expected a date validation error before any request", params)
		}
	}
}
