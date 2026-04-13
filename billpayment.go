package quickbooks

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
)

type BillPayment struct {
	Id string `json:"Id,omitempty"`
	// Reference number for the transaction. If not explicitly provided at create time, a custom value can be provided. If no value is supplied, the resulting DocNumber is null. Throws an error when duplicate DocNumber is sent in the request. Recommended best practice: check the setting of Preferences:OtherPrefs before setting DocNumber. If a duplicate DocNumber needs to be supplied, add the query parameter name/value pair, include=allowduplicatedocnum to the URI. Sort order is ASC by default.
	DocNumber         string             `json:",omitempty"`
	SyncToken         string             `json:",omitempty"`
	MetaData          MetaData           `json:",omitempty"`
	VendorRef         ReferenceType      `json:",omitempty"`
	PayType           string             `json:",omitempty"`
	CheckPayment      *CheckPayment      `json:",omitempty"`
	CreditCardPayment *CreditCardPayment `json:",omitempty"`
	TotalAmt          json.Number        `json:",omitempty"`
	PrivateNote       string             `json:",omitempty"`
	TxnDate           Date               `json:",omitempty"`
	CurrencyRef       ReferenceType      `json:",omitempty"`
	ExchangeRate      json.Number        `json:",omitempty"`
	DepartmentRef     ReferenceType      `json:",omitempty"`
	Line              []BillPaymentLine  `json:",omitempty"`
}

type CheckPayment struct {
	BankAccountRef ReferenceType `json:",omitempty"`
	PrintStatus    string        `json:",omitempty"`
}

type CreditCardPayment struct {
	CCAccountRef ReferenceType `json:",omitempty"`
}

type BillPaymentLine struct {
	Amount    json.Number `json:",omitempty"`
	LinkedTxn []LinkedTxn `json:",omitempty"`
}

func (c *Client) CreateBillPayment(ctx context.Context, billPayment *BillPayment) (*BillPayment, error) {
	var resp struct {
		BillPayment BillPayment
		Time        Date
	}

	if err := c.post(ctx, "billpayment", billPayment, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.BillPayment, nil
}

func (c *Client) DeleteBillPayment(ctx context.Context, billPayment *BillPayment) error {
	if billPayment.Id == "" || billPayment.SyncToken == "" {
		return errors.New("missing id/sync token")
	}

	return c.post(ctx, "billpayment", billPayment, nil, map[string]string{"operation": "delete"})
}

func (c *Client) FindBillPayments(ctx context.Context) ([]BillPayment, error) {
	var resp struct {
		QueryResponse struct {
			BillPayments  []BillPayment `json:"BillPayment"`
			MaxResults    int
			StartPosition int
			TotalCount    int
		}
	}

	if err := c.query(ctx, "SELECT COUNT(*) FROM BillPayment", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no bill payments could be found")
	}

	billPayments := make([]BillPayment, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM BillPayment ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(ctx, query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.BillPayments == nil {
			return nil, errors.New("no bill payments could be found")
		}

		billPayments = append(billPayments, resp.QueryResponse.BillPayments...)
	}

	return billPayments, nil
}

func (c *Client) FindBillPaymentById(ctx context.Context, id string) (*BillPayment, error) {
	var resp struct {
		BillPayment BillPayment
		Time        Date
	}

	if err := c.get(ctx, "billpayment/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.BillPayment, nil
}

func (c *Client) QueryBillPayments(ctx context.Context, query string) ([]BillPayment, error) {
	var resp struct {
		QueryResponse struct {
			BillPayments  []BillPayment `json:"BillPayment"`
			StartPosition int
			MaxResults    int
		}
	}

	if err := c.query(ctx, query, &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.BillPayments == nil {
		return nil, errors.New("could not find any bill payments")
	}

	return resp.QueryResponse.BillPayments, nil
}

func (c *Client) UpdateBillPayment(ctx context.Context, billPayment *BillPayment) (*BillPayment, error) {
	if billPayment.Id == "" {
		return nil, errors.New("missing bill payment id")
	}

	existingBillPayment, err := c.FindBillPaymentById(ctx, billPayment.Id)
	if err != nil {
		return nil, err
	}

	billPayment.SyncToken = existingBillPayment.SyncToken

	payload := struct {
		*BillPayment
		Sparse bool `json:"sparse"`
	}{
		BillPayment: billPayment,
		Sparse:      true,
	}

	var billPaymentData struct {
		BillPayment BillPayment
		Time        Date
	}

	if err = c.post(ctx, "billpayment", payload, &billPaymentData, nil); err != nil {
		return nil, err
	}

	return &billPaymentData.BillPayment, err
}
