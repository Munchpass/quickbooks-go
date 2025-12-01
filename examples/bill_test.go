package examples

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/rwestlund/quickbooks-go/v2"
	"github.com/stretchr/testify/require"
)

func TestCreateBill(t *testing.T) {
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

	accs, err := qbClient.FindAccounts()
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

	vendors, err := qbClient.FindVendors()
	require.NoError(t, err)

	var vendorId string
	for _, vendor := range vendors {
		if vendor.CompanyName == "Norton Lumber and Building Materials" {
			vendorId = vendor.Id
		}
	}
	require.NotEmpty(t, vendorId)

	invoiceDate := time.Date(2025, time.June, 2, 0, 0, 0, 0, time.UTC)
	bill, err := qbClient.CreateBill(&quickbooks.Bill{
		TxnDate: quickbooks.Date{Time: invoiceDate},
		VendorRef: quickbooks.ReferenceType{
			Value: vendorId,
		},
		APAccountRef: quickbooks.ReferenceType{
			Value: accPayableId,
		},
		DocNumber: "MUN-1000",
		TotalAmt:  json.Number("103.55"),
		CurrencyRef: quickbooks.ReferenceType{
			Name:  "United States Dollar",
			Value: "USD",
		},
		Line: []quickbooks.Line{
			{
				DetailType:  "AccountBasedExpenseLineDetail",
				Description: "Lumber",
				Amount:      json.Number("103.55"),
				AccountBasedExpenseLineDetail: quickbooks.AccountBasedExpenseLineDetail{
					AccountRef: quickbooks.ReferenceType{
						Value: accExpenseId,
					},
				},
			},
		},
	})
	require.NoError(t, err)
	t.Logf("bill: %+v", bill)

	// Create the attachment
	rawData, err := os.ReadFile("test.png")
	require.NoError(t, err)
	attachable, err := qbClient.UploadAttachable(&quickbooks.Attachable{
		FileName:    "test.png",
		ContentType: quickbooks.PNG,
		AttachableRef: []quickbooks.AttachableRef{
			{
				EntityRef: quickbooks.ReferenceType{
					Type:  "Bill",
					Value: bill.Id,
				},
			},
		},
	}, bytes.NewReader(rawData))
	require.NoError(t, err)
	t.Logf("attachable: %+v", attachable)

	// TODO: need to support creating the bill payment
	billPayment, err := qbClient.CreateBillPayment(&quickbooks.BillPayment{
		PrivateNote: "Payment for bill " + bill.DocNumber,
		VendorRef: quickbooks.ReferenceType{
			Name:  bill.VendorRef.Name,
			Value: bill.VendorRef.Value,
		},
		TotalAmt: json.Number("103.55"),
		PayType:  "Check",
		TxnDate:  quickbooks.Date{Time: time.Now()},
		CurrencyRef: quickbooks.ReferenceType{
			Name:  "United States Dollar",
			Value: "USD",
		},
		Line: []quickbooks.BillPaymentLine{
			{
				Amount: json.Number("103.55"),
				LinkedTxn: []quickbooks.LinkedTxn{
					{
						TxnID:   bill.Id,
						TxnType: "Bill",
					},
				},
			},
		},
		CheckPayment: &quickbooks.CheckPayment{
			BankAccountRef: quickbooks.ReferenceType{
				Value: "35",
			},
			PrintStatus: "NotSet",
		},
	})
	require.NoError(t, err)
	t.Logf("billPayment: %+v", billPayment)
}
