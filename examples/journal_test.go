package examples

import (
	"fmt"
	"testing"

	"github.com/rwestlund/quickbooks-go/v2"
	"github.com/stretchr/testify/require"
)

func toPtr[T any](r T) *T {
	return &r
}

func TestCreateJournalEntry(t *testing.T) {
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

	// Make a request!
	info, err := qbClient.CreateJournalEntry(
		&quickbooks.JournalEntry{
			TxnDate:   toPtr("2025-06-01"),
			DocNumber: toPtr("MUN-1000"),
			Line: []*quickbooks.JournalEntryLineItem{
				{
					JournalEntryLineDetail: &quickbooks.JournalEntryLineItemDetail{
						PostingType: toPtr(quickbooks.PostingTypeDebit),
						AccountRef: &quickbooks.BasicReferenceType{
							Name:  "Opening Bal Equity",
							Value: "39",
						},
					},
					DetailType: toPtr("JournalEntryLineDetail"),
					Amount:     toPtr(100.),
				},
				{
					JournalEntryLineDetail: &quickbooks.JournalEntryLineItemDetail{
						PostingType: toPtr(quickbooks.PostingTypeCredit),
						AccountRef: &quickbooks.BasicReferenceType{
							Name:  "Notes Payable",
							Value: "44",
						},
					},
					DetailType:  toPtr("JournalEntryLineDetail"),
					Amount:      toPtr(100.),
					Description: toPtr("Payment for services"),
				},
			},
		},
	)
	require.NoError(t, err)
	fmt.Printf("result: %+v\n", info)
}
