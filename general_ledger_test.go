package quickbooks

import (
	"context"
	"testing"
)

func TestFindGeneralLedgerReport_SendsEveryField(t *testing.T) {
	srv, gotPath, gotQuery := captureServer(t, `{"Header":{"ReportName":"GeneralLedger"}}`)
	c := newTestClient(t, srv.URL)
	report, err := c.FindGeneralLedgerReport(context.Background(), GeneralLedgerReportParams{
		StartDate:        "2026-09-01",
		EndDate:          "2026-09-30",
		AccountingMethod: AccountingMethodCash,
		AccountIds:       []string{"35", "80"},
		DepartmentIds:    []string{"2"},
		ClassIds:         []string{"7"},
		CustomerIds:      []string{"11"},
		VendorIds:        []string{"56"},
		Columns:          []string{"tx_date", "txn_type", "subt_nat_amount"},
		Extra:            map[string]string{"sort_order": "descend"},
	})
	if err != nil {
		t.Fatalf("FindGeneralLedgerReport: %v", err)
	}
	if report.Header.ReportName != "GeneralLedger" {
		t.Fatalf("report not decoded, got name %q", report.Header.ReportName)
	}
	if *gotPath != "/v3/company/test-realm/reports/GeneralLedger" {
		t.Fatalf("path = %q, want the GeneralLedger report", *gotPath)
	}
	requireQuery(t, *gotQuery, map[string]string{
		"start_date":        "2026-09-01",
		"end_date":          "2026-09-30",
		"accounting_method": "Cash",
		"account":           "35,80",
		"department":        "2",
		"class":             "7",
		"customer":          "11",
		"vendor":            "56",
		"columns":           "tx_date,txn_type,subt_nat_amount",
		"sort_order":        "descend",
	})
}

func TestFindGeneralLedgerReport_DatesOnlySendNoFilters(t *testing.T) {
	srv, _, gotQuery := captureServer(t, `{"Header":{"ReportName":"GeneralLedger"}}`)
	c := newTestClient(t, srv.URL)
	if _, err := c.FindGeneralLedgerReport(context.Background(), GeneralLedgerReportParams{StartDate: "2026-09-01", EndDate: "2026-09-30"}); err != nil {
		t.Fatalf("FindGeneralLedgerReport: %v", err)
	}
	requireQuery(t, *gotQuery, map[string]string{"start_date": "2026-09-01", "end_date": "2026-09-30"})
}

func TestFindGeneralLedgerReport_RejectsBadParamsBeforeAnyRequest(t *testing.T) {
	tests := map[string]GeneralLedgerReportParams{
		"missing end date":           {StartDate: "2026-09-01"},
		"unparseable start":          {StartDate: "Sept 1", EndDate: "2026-09-30"},
		"unknown accounting method":  {StartDate: "2026-09-01", EndDate: "2026-09-30", AccountingMethod: "Modified"},
		"extra repeats a field":      {StartDate: "2026-09-01", EndDate: "2026-09-30", Extra: map[string]string{"account": "35"}},
		"extra repeats the date":     {StartDate: "2026-09-01", EndDate: "2026-09-30", Extra: map[string]string{"end_date": "2026-12-31"}},
		"extra repeats minorversion": {StartDate: "2026-09-01", EndDate: "2026-09-30", Extra: map[string]string{"minorversion": "4"}},
	}
	for name, params := range tests {
		t.Run(name, func(t *testing.T) {
			// A nil http client panics if a request is attempted.
			c := &Client{}
			if _, err := c.FindGeneralLedgerReport(context.Background(), params); err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
}

func TestReport_DecodesGeneralLedgerSectionsWithIds(t *testing.T) {
	srv, _, _ := captureServer(t, `{
		"Header": {"ReportName": "GeneralLedger"},
		"Columns": {"Column": [
			{"ColTitle": "Date", "ColType": "Date", "MetaData": [{"Name": "ColKey", "Value": "tx_date"}]},
			{"ColTitle": "Transaction Type", "ColType": "String", "MetaData": [{"Name": "ColKey", "Value": "txn_type"}]},
			{"ColTitle": "Amount", "ColType": "Money", "MetaData": [{"Name": "ColKey", "Value": "subt_nat_amount"}]}
		]},
		"Rows": {"Row": [{
			"type": "Section",
			"Header": {"ColData": [{"value": "Cost of Goods Sold", "id": "80"}, {"value": ""}, {"value": ""}]},
			"Rows": {"Row": [{"type": "Data", "ColData": [{"value": "2026-09-03"}, {"value": "Bill", "id": "145"}, {"value": "42.50"}]}]},
			"Summary": {"ColData": [{"value": "Total for Cost of Goods Sold"}, {"value": ""}, {"value": "42.50"}]}
		}]}
	}`)
	c := newTestClient(t, srv.URL)
	report, err := c.FindGeneralLedgerReport(context.Background(), GeneralLedgerReportParams{StartDate: "2026-09-01", EndDate: "2026-09-30"})
	if err != nil {
		t.Fatalf("FindGeneralLedgerReport: %v", err)
	}
	if got := report.Columns.Column[1].MetaData[0].Value; got != "txn_type" {
		t.Fatalf("column key = %q, want txn_type", got)
	}
	section := report.Rows.Row[0]
	if section.Header.ColData[0].ID != "80" {
		t.Fatalf("section account id = %q, want 80", section.Header.ColData[0].ID)
	}
	data := section.Rows.Row[0].ColData
	if data[1].Value != "Bill" || data[1].ID != "145" || data[2].Value != "42.50" {
		t.Fatalf("data row = %+v, want the Bill with its transaction id and amount", data)
	}
}
