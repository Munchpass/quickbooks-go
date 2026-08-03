package examples

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rwestlund/quickbooks-go/v2"
	"github.com/stretchr/testify/require"
)

// TestCreateBillAndVendorCredit exercises the shape an invoice takes when its
// credits outweigh its items. QuickBooks has no valid bill with a negative
// total, so the credit is applied to the bill only as far as $0 and the
// remainder posts as a vendor credit the vendor's next bill can draw against.
//
// The invoice modelled here is $20 of goods against a $70 return:
//
//	bill          20.00 goods + -20.00 credit =  0.00
//	vendor credit                     50.00   = 50.00
func TestCreateBillAndVendorCredit(t *testing.T) {
	// If just testing, get values from:
	// https://developer.intuit.com/app/developer/playground
	clientId := "<your-client-id>"
	clientSecret := "<your-client-secret>"
	realmId := "<realm-id>"

	token := quickbooks.BearerToken{
		RefreshToken: "<saved-refresh-token>",
		AccessToken:  "<saved-access-token>",
	}
	qbClient, err := quickbooks.NewClient(clientId, clientSecret, realmId, false, "", &token)
	require.NoError(t, err)

	ctx := context.Background()

	accs, err := qbClient.FindAccounts(ctx)
	require.NoError(t, err)

	var accPayableId string
	var accExpenseId string
	for _, acc := range accs {
		if acc.Name == "Accounts Payable (A/P)" {
			accPayableId = acc.Id
		}

		if acc.AccountType == quickbooks.CostOfGoodsSoldAccountType {
			accExpenseId = acc.Id
		}
	}
	require.NotEmpty(t, accPayableId)
	require.NotEmpty(t, accExpenseId)

	vendors, err := qbClient.FindVendors(ctx)
	require.NoError(t, err)

	var vendorId string
	for _, vendor := range vendors {
		if vendor.CompanyName == "Norton Lumber and Building Materials" {
			vendorId = vendor.Id
		}
	}
	require.NotEmpty(t, vendorId)

	invoiceDate := time.Date(2025, time.June, 2, 0, 0, 0, 0, time.UTC)
	usd := quickbooks.ReferenceType{Name: "United States Dollar", Value: "USD"}
	expenseAccount := quickbooks.ReferenceType{Value: accExpenseId}

	// The bill keeps the goods and absorbs credit down to exactly zero. A bill
	// carrying the full 70 credit would total -50, which QuickBooks rejects.
	bill, err := qbClient.CreateBill(ctx, &quickbooks.Bill{
		TxnDate:      quickbooks.Date{Time: invoiceDate},
		VendorRef:    quickbooks.ReferenceType{Value: vendorId},
		APAccountRef: quickbooks.ReferenceType{Value: accPayableId},
		DocNumber:    "MUN-2000",
		TotalAmt:     json.Number("0.00"),
		CurrencyRef:  usd,
		Line: []quickbooks.Line{
			{
				DetailType:  "AccountBasedExpenseLineDetail",
				Description: "Lumber",
				Amount:      json.Number("20.00"),
				AccountBasedExpenseLineDetail: quickbooks.AccountBasedExpenseLineDetail{
					AccountRef: expenseAccount,
				},
			},
			{
				DetailType:  "AccountBasedExpenseLineDetail",
				Description: "Returned pallet (applied)",
				Amount:      json.Number("-20.00"),
				AccountBasedExpenseLineDetail: quickbooks.AccountBasedExpenseLineDetail{
					AccountRef: expenseAccount,
				},
			},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, bill.Id)
	t.Logf("bill: %+v", bill)

	billTotal, err := bill.TotalAmt.Float64()
	require.NoError(t, err)
	require.Equal(t, 0.00, billTotal, "the bill must land on exactly zero, not a negative total")

	// The credit QuickBooks could not fit onto the bill. A vendor credit states
	// its amounts positively — the document itself is the credit, so a negative
	// line would reverse it back into a payable.
	vendorCredit, err := qbClient.CreateVendorCredit(ctx, &quickbooks.VendorCredit{
		TxnDate:      quickbooks.Date{Time: invoiceDate},
		VendorRef:    quickbooks.ReferenceType{Value: vendorId},
		APAccountRef: quickbooks.ReferenceType{Value: accPayableId},
		DocNumber:    "MUN-2000",
		PrivateNote:  "Credit remaining after bill MUN-2000",
		TotalAmt:     json.Number("50.00"),
		CurrencyRef:  usd,
		Line: []quickbooks.Line{
			{
				DetailType:  "AccountBasedExpenseLineDetail",
				Description: "Returned pallet (remaining)",
				Amount:      json.Number("50.00"),
				AccountBasedExpenseLineDetail: quickbooks.AccountBasedExpenseLineDetail{
					AccountRef: expenseAccount,
				},
			},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, vendorCredit.Id)
	t.Logf("vendorCredit: %+v", vendorCredit)

	creditTotal, err := vendorCredit.TotalAmt.Float64()
	require.NoError(t, err)
	require.Equal(t, 50.00, creditTotal)

	// Read it back the way a re-export would, to confirm the id round-trips.
	found, err := qbClient.FindVendorCreditById(ctx, vendorCredit.Id)
	require.NoError(t, err)
	require.Equal(t, vendorCredit.Id, found.Id)
	require.Equal(t, vendorId, found.VendorRef.Value)
	require.Len(t, found.Line, 1)

	// Re-exporting the same invoice must overwrite the credit's lines rather than
	// stack a second set on it, or the vendor is credited twice.
	updated, err := qbClient.UpdateVendorCredit(ctx, &quickbooks.VendorCredit{
		Id:           vendorCredit.Id,
		TxnDate:      quickbooks.Date{Time: invoiceDate},
		VendorRef:    quickbooks.ReferenceType{Value: vendorId},
		APAccountRef: quickbooks.ReferenceType{Value: accPayableId},
		DocNumber:    "MUN-2000",
		PrivateNote:  "Credit remaining after bill MUN-2000 (revised)",
		TotalAmt:     json.Number("45.00"),
		CurrencyRef:  usd,
		Line: []quickbooks.Line{
			{
				DetailType:  "AccountBasedExpenseLineDetail",
				Description: "Returned pallet (remaining)",
				Amount:      json.Number("45.00"),
				AccountBasedExpenseLineDetail: quickbooks.AccountBasedExpenseLineDetail{
					AccountRef: expenseAccount,
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, vendorCredit.Id, updated.Id, "an update must not mint a second credit")
	require.Len(t, updated.Line, 1, "the line set is replaced wholesale, not appended to")

	updatedTotal, err := updated.TotalAmt.Float64()
	require.NoError(t, err)
	require.Equal(t, 45.00, updatedTotal)
	t.Logf("updatedVendorCredit: %+v", updated)
}
