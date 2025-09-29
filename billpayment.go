package quickbooks

import (
	"encoding/json"
	"errors"
	"strconv"
)

type BillPayment struct {
	Id                string             `json:"Id,omitempty"`
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
	CheckNum       string        `json:",omitempty"`
}

type CreditCardPayment struct {
	CCAccountRef ReferenceType `json:",omitempty"`
}

type BillPaymentLine struct {
	Amount    json.Number `json:",omitempty"`
	LinkedTxn []LinkedTxn `json:",omitempty"`
}

func (c *Client) CreateBillPayment(billPayment *BillPayment) (*BillPayment, error) {
	var resp struct {
		BillPayment BillPayment
		Time        Date
	}

	if err := c.post("billpayment", billPayment, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.BillPayment, nil
}

func (c *Client) DeleteBillPayment(billPayment *BillPayment) error {
	if billPayment.Id == "" || billPayment.SyncToken == "" {
		return errors.New("missing id/sync token")
	}

	return c.post("billpayment", billPayment, nil, map[string]string{"operation": "delete"})
}

func (c *Client) FindBillPayments() ([]BillPayment, error) {
	var resp struct {
		QueryResponse struct {
			BillPayments  []BillPayment `json:"BillPayment"`
			MaxResults    int
			StartPosition int
			TotalCount    int
		}
	}

	if err := c.query("SELECT COUNT(*) FROM BillPayment", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no bill payments could be found")
	}

	billPayments := make([]BillPayment, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM BillPayment ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.BillPayments == nil {
			return nil, errors.New("no bill payments could be found")
		}

		billPayments = append(billPayments, resp.QueryResponse.BillPayments...)
	}

	return billPayments, nil
}

func (c *Client) FindBillPaymentById(id string) (*BillPayment, error) {
	var resp struct {
		BillPayment BillPayment
		Time        Date
	}

	if err := c.get("billpayment/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.BillPayment, nil
}

func (c *Client) QueryBillPayments(query string) ([]BillPayment, error) {
	var resp struct {
		QueryResponse struct {
			BillPayments  []BillPayment `json:"BillPayment"`
			StartPosition int
			MaxResults    int
		}
	}

	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.BillPayments == nil {
		return nil, errors.New("could not find any bill payments")
	}

	return resp.QueryResponse.BillPayments, nil
}

func (c *Client) UpdateBillPayment(billPayment *BillPayment) (*BillPayment, error) {
	if billPayment.Id == "" {
		return nil, errors.New("missing bill payment id")
	}

	existingBillPayment, err := c.FindBillPaymentById(billPayment.Id)
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

	if err = c.post("billpayment", payload, &billPaymentData, nil); err != nil {
		return nil, err
	}

	return &billPaymentData.BillPayment, err
}
