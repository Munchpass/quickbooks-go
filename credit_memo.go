package quickbooks

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
)

type CreditMemo struct {
	TotalAmt              float64         `json:",omitempty"`
	RemainingCredit       json.Number     `json:",omitempty"`
	Line                  []Line          `json:",omitempty"`
	ApplyTaxAfterDiscount bool            `json:",omitempty"`
	DocNumber             string          `json:",omitempty"`
	TxnDate               Date            `json:",omitempty"`
	Sparse                bool            `json:"sparse,omitempty"`
	CustomerMemo          MemoRef         `json:",omitempty"`
	ProjectRef            ReferenceType   `json:",omitempty"`
	Balance               json.Number     `json:",omitempty"`
	CustomerRef           ReferenceType   `json:",omitempty"`
	TxnTaxDetail          *TxnTaxDetail   `json:",omitempty"`
	SyncToken             string          `json:",omitempty"`
	CustomField           []CustomField   `json:",omitempty"`
	ShipAddr              PhysicalAddress `json:",omitempty"`
	EmailStatus           string          `json:",omitempty"`
	BillAddr              PhysicalAddress `json:",omitempty"`
	MetaData              MetaData        `json:",omitempty"`
	BillEmail             EmailAddress    `json:",omitempty"`
	Id                    string          `json:",omitempty"`
}

// CreateCreditMemo creates the given CreditMemo witin QuickBooks.
func (c *Client) CreateCreditMemo(ctx context.Context, creditMemo *CreditMemo) (*CreditMemo, error) {
	var resp struct {
		CreditMemo CreditMemo
		Time       Date
	}

	if err := c.post(ctx, "creditmemo", creditMemo, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.CreditMemo, nil
}

// DeleteCreditMemo deletes the given credit memo.
func (c *Client) DeleteCreditMemo(ctx context.Context, creditMemo *CreditMemo) error {
	if creditMemo.Id == "" || creditMemo.SyncToken == "" {
		return errors.New("missing id/sync token")
	}

	return c.post(ctx, "creditmemo", creditMemo, nil, map[string]string{"operation": "delete"})
}

// FindCreditMemos retrieves the full list of credit memos from QuickBooks.
func (c *Client) FindCreditMemos(ctx context.Context) ([]CreditMemo, error) {
	var resp struct {
		QueryResponse struct {
			CreditMemos   []CreditMemo `json:"CreditMemo"`
			MaxResults    int
			StartPosition int
			TotalCount    int
		}
	}

	if err := c.query(ctx, "SELECT COUNT(*) FROM CreditMemo", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no credit memos could be found")
	}

	creditMemos := make([]CreditMemo, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM CreditMemo ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(ctx, query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.CreditMemos == nil {
			return nil, errors.New("no credit memos could be found")
		}

		creditMemos = append(creditMemos, resp.QueryResponse.CreditMemos...)
	}

	return creditMemos, nil
}

// FindCreditMemoById retrieves the given credit memo from QuickBooks.
func (c *Client) FindCreditMemoById(ctx context.Context, id string) (*CreditMemo, error) {
	var resp struct {
		CreditMemo CreditMemo
		Time       Date
	}

	if err := c.get(ctx, "creditmemo/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.CreditMemo, nil
}

// QueryCreditMemos accepts n SQL query and returns all credit memos found using it.
func (c *Client) QueryCreditMemos(ctx context.Context, query string) ([]CreditMemo, error) {
	var resp struct {
		QueryResponse struct {
			CreditMemos   []CreditMemo `json:"CreditMemo"`
			StartPosition int
			MaxResults    int
		}
	}

	if err := c.query(ctx, query, &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.CreditMemos == nil {
		return nil, errors.New("could not find any credit memos")
	}

	return resp.QueryResponse.CreditMemos, nil
}

// UpdateCreditMemo updates the given credit memo.
func (c *Client) UpdateCreditMemo(ctx context.Context, creditMemo *CreditMemo) (*CreditMemo, error) {
	if creditMemo.Id == "" {
		return nil, errors.New("missing credit memo id")
	}

	existingCreditMemo, err := c.FindCreditMemoById(ctx, creditMemo.Id)
	if err != nil {
		return nil, err
	}

	creditMemo.SyncToken = existingCreditMemo.SyncToken

	payload := struct {
		*CreditMemo
		Sparse bool `json:"sparse"`
	}{
		CreditMemo: creditMemo,
		Sparse:     true,
	}

	var creditMemoData struct {
		CreditMemo CreditMemo
		Time       Date
	}

	if err = c.post(ctx, "creditmemo", payload, &creditMemoData, nil); err != nil {
		return nil, err
	}

	return &creditMemoData.CreditMemo, err
}
