package quickbooks

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
)

const (
	BankAccountType                  = "Bank"
	OtherCurrentAssetAccountType     = "Other Current Asset"
	FixedAssetAccountType            = "Fixed Asset"
	OtherAssetAccountType            = "Other Asset"
	AccountsReceivableAccountType    = "Accounts Receivable"
	EquityAccountType                = "Equity"
	ExpenseAccountType               = "Expense"
	OtherExpenseAccountType          = "Other Expense"
	CostOfGoodsSoldAccountType       = "Cost of Goods Sold"
	AccountsPayableAccountType       = "Accounts Payable"
	CreditCardAccountType            = "Credit Card"
	LongTermLiabilityAccountType     = "Long Term Liability"
	OtherCurrentLiabilityAccountType = "Other Current Liability"
	IncomeAccountType                = "Income"
	OtherIncomeAccountType           = "Other Income"
)

type Account struct {
	Id                            string        `json:"Id,omitempty"`
	Name                          string        `json:",omitempty"`
	SyncToken                     string        `json:",omitempty"`
	AcctNum                       string        `json:",omitempty"`
	CurrencyRef                   ReferenceType `json:",omitempty"`
	ParentRef                     ReferenceType `json:",omitempty"`
	Description                   string        `json:",omitempty"`
	Active                        bool          `json:",omitempty"`
	MetaData                      MetaData      `json:",omitempty"`
	SubAccount                    bool          `json:",omitempty"`
	Classification                string        `json:",omitempty"`
	FullyQualifiedName            string        `json:",omitempty"`
	TxnLocationType               string        `json:",omitempty"`
	AccountType                   string        `json:",omitempty"`
	CurrentBalanceWithSubAccounts json.Number   `json:",omitempty"`
	AccountAlias                  string        `json:",omitempty"`
	TaxCodeRef                    ReferenceType `json:",omitempty"`
	AccountSubType                string        `json:",omitempty"`
	CurrentBalance                json.Number   `json:",omitempty"`
}

// CreateAccount creates the given account within QuickBooks
func (c *Client) CreateAccount(ctx context.Context, account *Account) (*Account, error) {
	var resp struct {
		Account Account
		Time    Date
	}

	if err := c.post(ctx, "account", account, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Account, nil
}

// FindAccounts gets the full list of Accounts in the QuickBooks account.
func (c *Client) FindAccounts(ctx context.Context) ([]Account, error) {
	var resp struct {
		QueryResponse struct {
			Accounts      []Account `json:"Account"`
			MaxResults    int
			StartPosition int
			TotalCount    int
		}
	}

	if err := c.query(ctx, "SELECT COUNT(*) FROM Account", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no accounts could be found")
	}

	accounts := make([]Account, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM Account ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(ctx, query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.Accounts == nil {
			return nil, errors.New("no accounts could be found")
		}

		accounts = append(accounts, resp.QueryResponse.Accounts...)
	}

	return accounts, nil
}

// FindAccountById returns an account with a given Id.
func (c *Client) FindAccountById(ctx context.Context, id string) (*Account, error) {
	var resp struct {
		Account Account
		Time    Date
	}

	if err := c.get(ctx, "account/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Account, nil
}

// QueryAccounts accepts an SQL query and returns all accounts found using it
func (c *Client) QueryAccounts(ctx context.Context, query string) ([]Account, error) {
	var resp struct {
		QueryResponse struct {
			Accounts      []Account `json:"Account"`
			StartPosition int
			MaxResults    int
		}
	}

	if err := c.query(ctx, query, &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.Accounts == nil {
		return nil, errors.New("could not find any accounts")
	}

	return resp.QueryResponse.Accounts, nil
}

// UpdateAccount updates the account
func (c *Client) UpdateAccount(ctx context.Context, account *Account) (*Account, error) {
	if account.Id == "" {
		return nil, errors.New("missing account id")
	}

	existingAccount, err := c.FindAccountById(ctx, account.Id)
	if err != nil {
		return nil, err
	}

	account.SyncToken = existingAccount.SyncToken

	payload := struct {
		*Account
		Sparse bool `json:"sparse"`
	}{
		Account: account,
		Sparse:  true,
	}

	var accountData struct {
		Account Account
		Time    Date
	}

	if err = c.post(ctx, "account", payload, &accountData, nil); err != nil {
		return nil, err
	}

	return &accountData.Account, err
}
