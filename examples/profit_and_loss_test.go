package examples

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/rwestlund/quickbooks-go/v2"
	"github.com/stretchr/testify/require"
)

func TestFindPnLReport(t *testing.T) {
	// If just testing, get values from:
	// https://developer.intuit.com/app/developer/playground
	clientId := "<your-client-id>"
	clientSecret := "<your-client-secret>"
	realmId := "<realm-id>"

	token := quickbooks.BearerToken{
		RefreshToken: "<saved-refresh-token>",
		AccessToken:  "<saved-access-token>",
	}
	qbClient, err := quickbooks.NewClient(clientId, clientSecret, realmId, false, "75", &token)
	require.NoError(t, err)

	resp, err := qbClient.FindPnLReport("2025-01-01", "2025-09-30")
	require.NoError(t, err)

	raw, err := os.ReadFile("./profit_and_loss.json")
	require.NoError(t, err)

	var correctResp *quickbooks.Report
	err = json.Unmarshal(raw, &correctResp)
	require.NoError(t, err)

	// Make it equal since Header.Time is the timestamp when the req
	// was made
	correctResp.Header.Time = resp.Header.Time
	require.Equal(t, correctResp, resp, "unexpected parsed response")
}
