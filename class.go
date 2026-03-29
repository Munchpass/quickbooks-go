package quickbooks

import (
	"errors"
	"strconv"
)

// Class describes a QBO Class (used as departments/cost centers).
type Class struct {
	Id                 string        `json:"Id,omitempty"`
	SyncToken          string        `json:",omitempty"`
	MetaData           MetaData      `json:",omitempty"`
	Name               string        `json:",omitempty"`
	SubClass           bool          `json:",omitempty"`
	ParentRef          ReferenceType `json:",omitempty"`
	FullyQualifiedName string        `json:",omitempty"`
	Active             bool          `json:",omitempty"`
}

// FindClasses gets the full list of Classes in the QuickBooks account.
func (c *Client) FindClasses() ([]Class, error) {
	var resp struct {
		QueryResponse struct {
			Classes       []Class `json:"Class"`
			MaxResults    int
			StartPosition int
			TotalCount    int
		}
	}

	if err := c.query("SELECT COUNT(*) FROM Class", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no classs could be found")
	}

	classes := make([]Class, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM Class ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.Classes == nil {
			return nil, errors.New("no classs could be found")
		}

		classes = append(classes, resp.QueryResponse.Classes...)
	}

	return classes, nil
}

// FindClassById finds the class by the given id.
func (c *Client) FindClassById(id string) (*Class, error) {
	var resp struct {
		Class Class
		Time  Date
	}

	if err := c.get("class/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Class, nil
}
