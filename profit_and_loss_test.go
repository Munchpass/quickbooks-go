package quickbooks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// captureServer answers every request with body and records the last request's
// path and query.
func captureServer(t *testing.T, body string) (*httptest.Server, *string, *url.Values) {
	t.Helper()
	var gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &gotPath, &gotQuery
}

func requireQuery(t *testing.T, got url.Values, want map[string]string) {
	t.Helper()
	want["minorversion"] = "65"
	if len(got) != len(want) {
		t.Fatalf("query = %v, want exactly %v (an unset field must send nothing: an empty filter would drop untagged lines)", got, want)
	}
	for key, value := range want {
		if got.Get(key) != value {
			t.Errorf("query[%s] = %q, want %q", key, got.Get(key), value)
		}
	}
}

const emptyPnL = `{"Header":{"ReportName":"ProfitAndLoss"}}`

func TestFindPnLReportWithParams_SendsEveryField(t *testing.T) {
	tests := []struct {
		name   string
		params PnLReportParams
		want   map[string]string
	}{
		{
			name:   "dates only",
			params: PnLReportParams{StartDate: "2026-09-01", EndDate: "2026-09-30"},
			want:   map[string]string{"start_date": "2026-09-01", "end_date": "2026-09-30"},
		},
		{
			name: "every typed field, ids comma joined",
			params: PnLReportParams{
				StartDate:         "2026-09-01",
				EndDate:           "2026-09-30",
				AccountingMethod:  AccountingMethodAccrual,
				SummarizeColumnBy: SummarizeByDepartments,
				DepartmentIds:     []string{"1", "4"},
				ClassIds:          []string{"7", "8", "9"},
				CustomerIds:       []string{"11"},
				VendorIds:         []string{"21", "22"},
				ItemIds:           []string{"31"},
			},
			want: map[string]string{
				"start_date":          "2026-09-01",
				"end_date":            "2026-09-30",
				"accounting_method":   "Accrual",
				"summarize_column_by": "Departments",
				"department":          "1,4",
				"class":               "7,8,9",
				"customer":            "11",
				"vendor":              "21,22",
				"item":                "31",
			},
		},
		{
			name: "extra parameters pass through as given",
			params: PnLReportParams{
				StartDate: "2026-09-01",
				EndDate:   "2026-09-30",
				Extra:     map[string]string{"sort_order": "descend", "adjusted_gain_loss": "true"},
			},
			want: map[string]string{
				"start_date":         "2026-09-01",
				"end_date":           "2026-09-30",
				"sort_order":         "descend",
				"adjusted_gain_loss": "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, gotPath, gotQuery := captureServer(t, emptyPnL)
			c := newTestClient(t, srv.URL)
			report, err := c.FindPnLReportWithParams(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("FindPnLReportWithParams: %v", err)
			}
			if report.Header.ReportName != "ProfitAndLoss" {
				t.Fatalf("report not decoded, got name %q", report.Header.ReportName)
			}
			if *gotPath != "/v3/company/test-realm/reports/ProfitAndLoss" {
				t.Fatalf("path = %q, want the ProfitAndLoss report", *gotPath)
			}
			requireQuery(t, *gotQuery, tt.want)
		})
	}
}

func TestFindPnLDetailReport_UsesDetailEndpointAndColumns(t *testing.T) {
	srv, gotPath, gotQuery := captureServer(t, `{"Header":{"ReportName":"ProfitAndLossDetail"}}`)
	c := newTestClient(t, srv.URL)
	report, err := c.FindPnLDetailReport(context.Background(), PnLReportParams{
		StartDate: "2026-09-01",
		EndDate:   "2026-09-30",
		ClassIds:  []string{"7"},
		Columns:   []string{"tx_date", "txn_type", "subt_nat_amount"},
	})
	if err != nil {
		t.Fatalf("FindPnLDetailReport: %v", err)
	}
	if report.Header.ReportName != "ProfitAndLossDetail" {
		t.Fatalf("report not decoded, got name %q", report.Header.ReportName)
	}
	if *gotPath != "/v3/company/test-realm/reports/ProfitAndLossDetail" {
		t.Fatalf("path = %q, want the ProfitAndLossDetail report", *gotPath)
	}
	requireQuery(t, *gotQuery, map[string]string{
		"start_date": "2026-09-01",
		"end_date":   "2026-09-30",
		"class":      "7",
		"columns":    "tx_date,txn_type,subt_nat_amount",
	})
}

func TestFindPnLReportWithParams_RejectsBadParamsBeforeAnyRequest(t *testing.T) {
	valid := func() PnLReportParams { return PnLReportParams{StartDate: "2026-09-01", EndDate: "2026-09-30"} }
	tests := map[string]PnLReportParams{
		"missing start date": {EndDate: "2026-09-30"},
		"unparseable end":    {StartDate: "2026-09-01", EndDate: "09/30/2026"},
		"unknown accounting method": func() PnLReportParams {
			p := valid()
			p.AccountingMethod = "Modified"
			return p
		}(),
		"extra repeats a typed field that is set": func() PnLReportParams {
			p := valid()
			p.DepartmentIds = []string{"1"}
			p.Extra = map[string]string{"department": "2"}
			return p
		}(),
		"extra names a typed field even when it is unset": func() PnLReportParams {
			p := valid()
			p.Extra = map[string]string{"class": "2"}
			return p
		}(),
		"extra overrides the minor version": func() PnLReportParams {
			p := valid()
			p.Extra = map[string]string{"minorversion": "4"}
			return p
		}(),
	}
	for name, params := range tests {
		t.Run(name, func(t *testing.T) {
			// A nil http client panics if a request is attempted.
			c := &Client{}
			if _, err := c.FindPnLReportWithParams(context.Background(), params); err == nil {
				t.Fatal("expected a validation error")
			}
			if _, err := c.FindPnLDetailReport(context.Background(), params); err == nil {
				t.Fatal("expected the detail report to validate the same way")
			}
		})
	}
}

// A report summarized by Departments has one amount column per Location plus
// the total; Report must keep every column's metadata and cell.
func TestReport_DecodesOneColumnPerDepartment(t *testing.T) {
	const body = `{
		"Header": {"ReportName": "ProfitAndLoss", "SummarizeColumnsBy": "Departments", "StartPeriod": "2026-09-01", "EndPeriod": "2026-09-30"},
		"Columns": {"Column": [
			{"ColTitle": "", "ColType": "Account", "MetaData": [{"Name": "ColKey", "Value": "account"}]},
			{"ColTitle": "Downtown", "ColType": "Money", "MetaData": [{"Name": "ColKey", "Value": "1"}]},
			{"ColTitle": "Not Specified", "ColType": "Money", "MetaData": [{"Name": "ColKey", "Value": "0"}]},
			{"ColTitle": "Total", "ColType": "Money", "MetaData": [{"Name": "ColKey", "Value": "total"}]}
		]},
		"Rows": {"Row": [{
			"type": "Section", "group": "Income",
			"Header": {"ColData": [{"value": "Income"}, {"value": ""}, {"value": ""}, {"value": ""}]},
			"Rows": {"Row": [{"type": "Data", "ColData": [{"value": "Sales", "id": "79"}, {"value": "1000.00"}, {"value": "25.50"}, {"value": "1025.50"}]}]},
			"Summary": {"ColData": [{"value": "Total Income"}, {"value": "1000.00"}, {"value": "25.50"}, {"value": "1025.50"}]}
		}]}
	}`
	srv, _, gotQuery := captureServer(t, body)
	c := newTestClient(t, srv.URL)
	report, err := c.FindPnLReportWithParams(context.Background(), PnLReportParams{
		StartDate:         "2026-09-01",
		EndDate:           "2026-09-30",
		SummarizeColumnBy: SummarizeByDepartments,
	})
	if err != nil {
		t.Fatalf("FindPnLReportWithParams: %v", err)
	}
	if gotQuery.Get("summarize_column_by") != "Departments" {
		t.Fatalf("summarize_column_by = %q", gotQuery.Get("summarize_column_by"))
	}

	if report.Header.SummarizeColumnsBy != "Departments" {
		t.Errorf("SummarizeColumnsBy = %q", report.Header.SummarizeColumnsBy)
	}
	var titles, keys []string
	for _, col := range report.Columns.Column {
		titles = append(titles, col.ColTitle)
		keys = append(keys, col.MetaData[0].Value)
	}
	if got := strings.Join(titles, "|"); got != "|Downtown|Not Specified|Total" {
		t.Errorf("column titles = %q", got)
	}
	if got := strings.Join(keys, "|"); got != "account|1|0|total" {
		t.Errorf("column keys = %q, want each Department's id so columns can be matched to Locations", got)
	}
	data := report.Rows.Row[0].Rows.Row[0].ColData
	if len(data) != 4 || data[0].ID != "79" || data[1].Value != "1000.00" || data[2].Value != "25.50" {
		raw, _ := json.Marshal(data)
		t.Errorf("data row cells = %s, want the account id and one amount per column", raw)
	}
}
