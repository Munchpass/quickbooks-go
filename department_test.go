package quickbooks

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDepartment(t *testing.T) {
	jsonFile, err := os.Open("data/testing/department.json")
	require.NoError(t, err)
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	require.NoError(t, err)

	var r struct {
		Department Department
		Time       Date
	}
	err = json.Unmarshal(byteValue, &r)
	require.NoError(t, err)

	assert.Equal(t, "Store 1", r.Department.FullyQualifiedName)
	assert.Equal(t, "Store 1", r.Department.Name)
	assert.False(t, r.Department.SubDepartment)
	assert.Equal(t, "2015-03-12T11:38:53-07:00", r.Department.MetaData.CreateTime.String())
	assert.Equal(t, "2015-03-12T11:38:53-07:00", r.Department.MetaData.LastUpdatedTime.String())
	assert.True(t, r.Department.Active)
	assert.Equal(t, "0", r.Department.SyncToken)
	assert.Equal(t, "1", r.Department.Id)
}
