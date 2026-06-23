package examples

import (
	"context"
	"fmt"
	"testing"

	"github.com/rwestlund/quickbooks-go/v2"
	"github.com/stretchr/testify/require"
)

func TestReadDepartments(t *testing.T) {
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

	// Make a request!
	info, err := qbClient.FindDepartments(
		ctx,
	)
	require.NoError(t, err)
	fmt.Printf("result: %+v\n", info)
}
