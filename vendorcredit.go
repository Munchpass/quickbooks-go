package quickbooks

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
)

// VendorCredit is the accounts-payable mirror of a Bill: it records money the
// vendor owes back, which QuickBooks applies against the vendor's open bills.
// A Bill cannot carry a negative total, so a net credit is posted here instead.
type VendorCredit struct {
	Id           string        `json:"Id,omitempty"`
	VendorRef    ReferenceType `json:",omitempty"`
	Line         []Line
	SyncToken    string        `json:",omitempty"`
	CurrencyRef  ReferenceType `json:",omitempty"`
	TxnDate      Date          `json:",omitempty"`
	APAccountRef ReferenceType `json:",omitempty"`
	LinkedTxn    []LinkedTxn   `json:",omitempty"`
	// GlobalTaxCalculation is one of TaxExcluded, TaxInclusive or NotApplicable.
	// Not applicable to US companies; required for non-US ones.
	GlobalTaxCalculation string        `json:",omitempty"`
	MetaData             MetaData      `json:",omitempty"`
	DocNumber            string        `json:",omitempty"`
	PrivateNote          string        `json:",omitempty"`
	ExchangeRate         json.Number   `json:",omitempty"`
	DepartmentRef        ReferenceType `json:",omitempty"`
	IncludeInAnnualTPAR  bool          `json:",omitempty"`
	RecurDataRef         ReferenceType `json:",omitempty"`
	// TotalAmt is the total of every line, including charges, allowances,
	// discounts and taxes. QuickBooks calculates it and overwrites anything sent.
	TotalAmt json.Number `json:",omitempty"`
	// Balance is the credit left after any adjustments, starting equal to
	// TotalAmt. QuickBooks calculates it and overwrites anything sent. Unlike
	// Bill there is no HomeBalance companion.
	Balance json.Number `json:",omitempty"`
}

// CreateVendorCredit creates the given VendorCredit on the QuickBooks server,
// returning the resulting VendorCredit object.
func (c *Client) CreateVendorCredit(ctx context.Context, vendorCredit *VendorCredit) (*VendorCredit, error) {
	var resp struct {
		VendorCredit VendorCredit
		Time         Date
	}

	if err := c.post(ctx, "vendorcredit", vendorCredit, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.VendorCredit, nil
}

// DeleteVendorCredit deletes the vendor credit
func (c *Client) DeleteVendorCredit(ctx context.Context, vendorCredit *VendorCredit) error {
	if vendorCredit.Id == "" || vendorCredit.SyncToken == "" {
		return errors.New("missing id/sync token")
	}

	return c.post(ctx, "vendorcredit", vendorCredit, nil, map[string]string{"operation": "delete"})
}

// FindVendorCredits gets the full list of VendorCredits in the QuickBooks account.
func (c *Client) FindVendorCredits(ctx context.Context) ([]VendorCredit, error) {
	var resp struct {
		QueryResponse struct {
			VendorCredits []VendorCredit `json:"VendorCredit"`
			MaxResults    int
			StartPosition int
			TotalCount    int
		}
	}

	if err := c.query(ctx, "SELECT COUNT(*) FROM VendorCredit", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no vendor credits could be found")
	}

	vendorCredits := make([]VendorCredit, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM VendorCredit ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(ctx, query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.VendorCredits == nil {
			return nil, errors.New("no vendor credits could be found")
		}

		vendorCredits = append(vendorCredits, resp.QueryResponse.VendorCredits...)
	}

	return vendorCredits, nil
}

// FindVendorCreditById finds the vendor credit by the given id
func (c *Client) FindVendorCreditById(ctx context.Context, id string) (*VendorCredit, error) {
	var resp struct {
		VendorCredit VendorCredit
		Time         Date
	}

	if err := c.get(ctx, "vendorcredit/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.VendorCredit, nil
}

// QueryVendorCredits accepts an SQL query and returns all vendor credits found using it
func (c *Client) QueryVendorCredits(ctx context.Context, query string) ([]VendorCredit, error) {
	var resp struct {
		QueryResponse struct {
			VendorCredits []VendorCredit `json:"VendorCredit"`
			StartPosition int
			MaxResults    int
		}
	}

	if err := c.query(ctx, query, &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.VendorCredits == nil {
		return nil, errors.New("could not find any vendor credits")
	}

	return resp.QueryResponse.VendorCredits, nil
}

// UpdateVendorCredit updates the vendor credit
func (c *Client) UpdateVendorCredit(ctx context.Context, vendorCredit *VendorCredit) (*VendorCredit, error) {
	if vendorCredit.Id == "" {
		return nil, errors.New("missing vendor credit id")
	}

	existingVendorCredit, err := c.FindVendorCreditById(ctx, vendorCredit.Id)
	if err != nil {
		return nil, err
	}

	vendorCredit.SyncToken = existingVendorCredit.SyncToken

	payload := struct {
		*VendorCredit
		Sparse bool `json:"sparse"`
	}{
		VendorCredit: vendorCredit,
		Sparse:       true,
	}

	var vendorCreditData struct {
		VendorCredit VendorCredit
		Time         Date
	}

	if err = c.post(ctx, "vendorcredit", payload, &vendorCreditData, nil); err != nil {
		return nil, err
	}

	return &vendorCreditData.VendorCredit, err
}
