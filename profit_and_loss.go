package quickbooks

import (
	"context"
	"errors"
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

// PnLReportParams narrows a P&L report request.
type PnLReportParams struct {
	// StartDate and EndDate are inclusive, formatted YYYY-MM-DD.
	StartDate string
	EndDate   string
	// DepartmentIds limits the report to lines on these Departments (shown as
	// "Location" in the QuickBooks UI). Empty means no Department filter, which
	// also keeps lines that have no Department.
	DepartmentIds []string
	// ClassIds limits the report to lines on these Classes. Empty means no Class
	// filter, which also keeps lines that have no Class.
	ClassIds []string
}

// FindPnLReport gets the P&L report for a date range
func (c *Client) FindPnLReport(ctx context.Context, startDate, endDate string) (*Report, error) {
	return c.FindPnLReportWithParams(ctx, PnLReportParams{StartDate: startDate, EndDate: endDate})
}

// FindPnLReportWithParams gets the P&L report for a date range, optionally
// filtered to Departments and Classes.
func (c *Client) FindPnLReportWithParams(ctx context.Context, params PnLReportParams) (*Report, error) {
	if params.StartDate == "" || params.EndDate == "" {
		return nil, errors.New("missing startDate or endDate")
	}

	_, err := time.Parse(time.DateOnly, params.StartDate)
	if err != nil {
		return nil, errors.New("startDate must be in YYYY-MM-DD format")
	}

	_, err = time.Parse(time.DateOnly, params.EndDate)
	if err != nil {
		return nil, errors.New("endDate must be in YYYY-MM-DD format")
	}

	query := map[string]string{"start_date": params.StartDate, "end_date": params.EndDate}
	if len(params.DepartmentIds) > 0 {
		query["department"] = strings.Join(params.DepartmentIds, ",")
	}
	if len(params.ClassIds) > 0 {
		query["class"] = strings.Join(params.ClassIds, ",")
	}

	var resp Report
	if err := c.get(ctx, "reports/ProfitAndLoss", &resp, query); err != nil {
		return nil, err
	}

	return &resp, nil
}
