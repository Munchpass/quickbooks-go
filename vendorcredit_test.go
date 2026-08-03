package quickbooks

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVendorCredit(t *testing.T) {
	jsonFile, err := os.Open("data/testing/vendorcredit.json")
	require.NoError(t, err, "unexpectedly failed to open vendorcredit.json")
	defer jsonFile.Close()

	byteValue, err := io.ReadAll(jsonFile)
	require.NoErrorf(t, err, "unexpectedly failed to read json")

	var r struct {
		VendorCredit VendorCredit
		Time         Date
	}
	err = json.Unmarshal(byteValue, &r)
	if err != nil {
		log.Fatal("When decoding JSON file: ", err)
	}
	assert.Equal(t, "0", r.VendorCredit.SyncToken)
	assert.Equal(t, "Accounts Payable (A/P)", r.VendorCredit.APAccountRef.Name)
	assert.Equal(t, "33", r.VendorCredit.APAccountRef.Value)
	assert.Equal(t, "Bob's Burger Joint", r.VendorCredit.VendorRef.Name)
	assert.Equal(t, "56", r.VendorCredit.VendorRef.Value)
	assert.Equal(t, "2015-07-27T00:00:00+00:00", r.VendorCredit.TxnDate.String())
	totalAmt, _ := r.VendorCredit.TotalAmt.Float64()
	assert.Equal(t, 100.00, totalAmt)
	assert.Equal(t, "United States Dollar", r.VendorCredit.CurrencyRef.Name)
	assert.Equal(t, "USD", r.VendorCredit.CurrencyRef.Value)
	assert.Equal(t, "CM-9", r.VendorCredit.DocNumber)
	assert.Equal(t, "Returned pallet", r.VendorCredit.PrivateNote)
	assert.Equal(t, 1, len(r.VendorCredit.Line))
	// A vendor credit states its amounts positively: the document itself is the
	// credit, so a negative line would reverse it back into a payable.
	amount, _ := r.VendorCredit.Line[0].Amount.Float64()
	assert.Equal(t, 100.00, amount)
	assert.Equal(t, "AccountBasedExpenseLineDetail", r.VendorCredit.Line[0].DetailType)
	assert.Equal(t, "64", r.VendorCredit.Line[0].AccountBasedExpenseLineDetail.AccountRef.Value)
	assert.Equal(t, "5000000000000123456", r.VendorCredit.Line[0].AccountBasedExpenseLineDetail.ClassRef.Value)
	balance, _ := r.VendorCredit.Balance.Int64()
	assert.Equal(t, int64(100), balance)
	assert.Equal(t, "197", r.VendorCredit.Id)
	assert.Equal(t, "2015-07-27T10:37:26-07:00", r.VendorCredit.MetaData.CreateTime.String())
	assert.Equal(t, "2015-07-27T10:37:26-07:00", r.VendorCredit.MetaData.LastUpdatedTime.String())
}
