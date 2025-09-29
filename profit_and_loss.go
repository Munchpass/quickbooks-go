package quickbooks

import (
	"errors"
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

// FindPnLReport gets the P&L report for a date range
func (c *Client) FindPnLReport(startDate, endDate string) (*Report, error) {
	if startDate == "" || endDate == "" {
		return nil, errors.New("missing startDate or endDate")
	}

	_, err := time.Parse(time.DateOnly, startDate)
	if err != nil {
		return nil, errors.New("startDate must be in YYYY-MM-DD format")
	}

	_, err = time.Parse(time.DateOnly, endDate)
	if err != nil {
		return nil, errors.New("endDate must be in YYYY-MM-DD format")
	}

	var resp Report
	if err := c.get("reports/ProfitAndLoss", &resp,
		map[string]string{"start_date": startDate, "end_date": endDate}); err != nil {
		return nil, err
	}

	return &resp, nil
}
