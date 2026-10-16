package quickbooks

import (
	"context"
	"strings"
)

// GeneralLedgerReportParams describes a GeneralLedger request: every posting
// to each account in a date range, grouped by account.
//
// Every id filter keeps only postings tagged with one of the ids, so a
// non-empty filter also drops postings with no value on that dimension. An
// empty filter sends nothing and keeps every posting.
type GeneralLedgerReportParams struct {
	// StartDate and EndDate are inclusive, formatted YYYY-MM-DD.
	StartDate string
	EndDate   string
	// AccountingMethod overrides the company's default basis when set.
	AccountingMethod AccountingMethod
	// AccountIds keeps only the postings to these accounts.
	AccountIds []string
	// DepartmentIds are QuickBooks Department ids, shown as "Location" in the
	// QuickBooks UI.
	DepartmentIds []string
	ClassIds      []string
	CustomerIds   []string
	VendorIds     []string
	// Columns picks the columns, by Intuit's column names (tx_date, txn_type,
	// doc_num, name, memo, split_acc, subt_nat_amount, klass_name, dept_name,
	// ...). Empty is the report's default set.
	Columns []string
	// Extra sends any other report parameter as given, so a parameter this
	// type does not model needs no new release. It may not repeat one the
	// fields above set.
	Extra map[string]string
}

// FindGeneralLedgerReport gets the GeneralLedger report.
func (c *Client) FindGeneralLedgerReport(ctx context.Context, params GeneralLedgerReportParams) (*Report, error) {
	query, err := reportQuery("GeneralLedgerReportParams", params.StartDate, params.EndDate, params.AccountingMethod, map[string]string{
		"account":    strings.Join(params.AccountIds, ","),
		"department": strings.Join(params.DepartmentIds, ","),
		"class":      strings.Join(params.ClassIds, ","),
		"customer":   strings.Join(params.CustomerIds, ","),
		"vendor":     strings.Join(params.VendorIds, ","),
		"columns":    strings.Join(params.Columns, ","),
	}, params.Extra)
	if err != nil {
		return nil, err
	}

	var resp Report
	if err := c.get(ctx, "reports/GeneralLedger", &resp, query); err != nil {
		return nil, err
	}
	return &resp, nil
}
