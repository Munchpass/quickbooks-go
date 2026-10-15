package quickbooks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// type ReportLineItem struct {
// 	AccountName string  `json:"account_name"`
// 	Amount      float64 `json:"amount"`
// 	AccountType string  `json:"account_type"`
// }

type ReportCell struct {
	Value string `json:"value"`
	ID    string `json:"id,omitempty"`
}

type ReportRowHeader struct {
	ColData []ReportCell `json:"ColData"`
}

type ReportRowsData struct {
	Row []ReportRow `json:"Row"`
}

type Summary struct {
	ColData []ReportCell `json:"ColData"`
}

type ReportRow struct {
	Header  ReportRowHeader `json:"Header,omitempty"`
	Rows    ReportRowsData  `json:"Rows,omitempty"`
	Summary Summary         `json:"Summary,omitempty"`
	ColData []ReportCell    `json:"ColData,omitempty"`
	Type    string          `json:"type,omitempty"`
	Group   string          `json:"group,omitempty"`
}

type Report struct {
	Header struct {
		Time        string `json:"Time"`
		ReportName  string `json:"ReportName"`
		ReportBasis string `json:"ReportBasis"`
		StartPeriod string `json:"StartPeriod"`
		EndPeriod   string `json:"EndPeriod"`
		// Total
		SummarizeColumnsBy string `json:"SummarizeColumnsBy"`
		Currency           string `json:"Currency"`
		Option             []struct {
			Name  string `json:"Name"`
			Value string `json:"Value"`
		} `json:"Option"`
	} `json:"Header"`
	Columns struct {
		Column []struct {
			ColTitle string `json:"ColTitle"`
			ColType  string `json:"ColType"`
			MetaData []struct {
				Name  string `json:"Name"`
				Value string `json:"Value"`
			} `json:"MetaData"`
		} `json:"Column"`
	}
	Rows ReportRowsData `json:"Rows"`
}

// SummarizeColumnBy splits a report's amounts into columns.
type SummarizeColumnBy string

const (
	SummarizeByTotal               SummarizeColumnBy = "Total"
	SummarizeByDays                SummarizeColumnBy = "Days"
	SummarizeByWeek                SummarizeColumnBy = "Week"
	SummarizeByMonth               SummarizeColumnBy = "Month"
	SummarizeByQuarter             SummarizeColumnBy = "Quarter"
	SummarizeByYear                SummarizeColumnBy = "Year"
	SummarizeByCustomers           SummarizeColumnBy = "Customers"
	SummarizeByVendors             SummarizeColumnBy = "Vendors"
	SummarizeByEmployees           SummarizeColumnBy = "Employees"
	SummarizeByProductsAndServices SummarizeColumnBy = "ProductsAndServices"
	// SummarizeByClasses gives one column per Class, plus one for lines with
	// no Class.
	SummarizeByClasses SummarizeColumnBy = "Classes"
	// SummarizeByDepartments gives one column per Department (shown as
	// "Location" in the QuickBooks UI), plus one for lines with no Department.
	SummarizeByDepartments SummarizeColumnBy = "Departments"
)

// AccountingMethod is the basis a report is run on.
type AccountingMethod string

const (
	AccountingMethodCash    AccountingMethod = "Cash"
	AccountingMethodAccrual AccountingMethod = "Accrual"
)

// PnLReportParams describes a ProfitAndLoss or ProfitAndLossDetail request.
//
// Every id filter keeps only lines tagged with one of the ids, so a non-empty
// filter also drops lines with no value on that dimension. An empty filter
// sends nothing and keeps every line.
type PnLReportParams struct {
	// StartDate and EndDate are inclusive, formatted YYYY-MM-DD.
	StartDate string
	EndDate   string
	// AccountingMethod overrides the company's default basis when set.
	AccountingMethod AccountingMethod
	// SummarizeColumnBy splits amounts into columns. Empty is one total
	// column. ProfitAndLossDetail ignores it.
	SummarizeColumnBy SummarizeColumnBy
	// DepartmentIds are QuickBooks Department ids, shown as "Location" in the
	// QuickBooks UI.
	DepartmentIds []string
	ClassIds      []string
	CustomerIds   []string
	VendorIds     []string
	ItemIds       []string
	// Columns picks the ProfitAndLossDetail columns, by Intuit's column
	// names. Empty is the report's default set. ProfitAndLoss ignores it.
	Columns []string
	// Extra sends any other report parameter as given, so a parameter this
	// type does not model needs no new release. It may not repeat one the
	// fields above set.
	Extra map[string]string
}

// FindPnLReport gets the P&L report for a date range
func (c *Client) FindPnLReport(ctx context.Context, startDate, endDate string) (*Report, error) {
	return c.FindPnLReportWithParams(ctx, PnLReportParams{StartDate: startDate, EndDate: endDate})
}

// FindPnLReportWithParams gets the ProfitAndLoss summary report.
func (c *Client) FindPnLReportWithParams(ctx context.Context, params PnLReportParams) (*Report, error) {
	return c.findPnLReport(ctx, "reports/ProfitAndLoss", params)
}

// FindPnLDetailReport gets the ProfitAndLossDetail report: the transaction
// lines behind each P&L account.
func (c *Client) FindPnLDetailReport(ctx context.Context, params PnLReportParams) (*Report, error) {
	return c.findPnLReport(ctx, "reports/ProfitAndLossDetail", params)
}

func (c *Client) findPnLReport(ctx context.Context, endpoint string, params PnLReportParams) (*Report, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}

	var resp Report
	if err := c.get(ctx, endpoint, &resp, query); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (p PnLReportParams) query() (map[string]string, error) {
	if p.StartDate == "" || p.EndDate == "" {
		return nil, errors.New("missing startDate or endDate")
	}
	if _, err := time.Parse(time.DateOnly, p.StartDate); err != nil {
		return nil, errors.New("startDate must be in YYYY-MM-DD format")
	}
	if _, err := time.Parse(time.DateOnly, p.EndDate); err != nil {
		return nil, errors.New("endDate must be in YYYY-MM-DD format")
	}
	switch p.AccountingMethod {
	case "", AccountingMethodCash, AccountingMethodAccrual:
	default:
		return nil, fmt.Errorf("accountingMethod must be %q or %q", AccountingMethodCash, AccountingMethodAccrual)
	}

	query := map[string]string{"start_date": p.StartDate, "end_date": p.EndDate}
	setIfAny := func(key, value string) {
		if value != "" {
			query[key] = value
		}
	}
	setIfAny("accounting_method", string(p.AccountingMethod))
	setIfAny("summarize_column_by", string(p.SummarizeColumnBy))
	setIfAny("department", strings.Join(p.DepartmentIds, ","))
	setIfAny("class", strings.Join(p.ClassIds, ","))
	setIfAny("customer", strings.Join(p.CustomerIds, ","))
	setIfAny("vendor", strings.Join(p.VendorIds, ","))
	setIfAny("item", strings.Join(p.ItemIds, ","))
	setIfAny("columns", strings.Join(p.Columns, ","))

	for key, value := range p.Extra {
		if _, typed := pnlReportTypedParams[key]; typed {
			return nil, fmt.Errorf("extra parameter %q has a PnLReportParams field; set that instead", key)
		}
		query[key] = value
	}
	return query, nil
}

// pnlReportTypedParams are the parameters PnLReportParams sets itself, plus
// the minor version the client always sends.
var pnlReportTypedParams = map[string]struct{}{
	"start_date":          {},
	"end_date":            {},
	"accounting_method":   {},
	"summarize_column_by": {},
	"department":          {},
	"class":               {},
	"customer":            {},
	"vendor":              {},
	"item":                {},
	"columns":             {},
	"minorversion":        {},
}
