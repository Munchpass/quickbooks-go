package quickbooks

import (
	"context"
	"errors"
	"strconv"
)

// Department describes a QBO Department, used to track different
// segments of a business (e.g. locations or stores).
type Department struct {
	Id                 string        `json:"Id,omitempty"`
	SyncToken          string        `json:",omitempty"`
	MetaData           MetaData      `json:",omitempty"`
	Name               string        `json:",omitempty"`
	SubDepartment      bool          `json:",omitempty"`
	ParentRef          ReferenceType `json:",omitempty"`
	FullyQualifiedName string        `json:",omitempty"`
	Active             bool          `json:",omitempty"`
}

// CreateDepartment creates the given department within QuickBooks.
func (c *Client) CreateDepartment(ctx context.Context, department *Department) (*Department, error) {
	var resp struct {
		Department Department
		Time       Date
	}

	if err := c.post(ctx, "department", department, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Department, nil
}

// FindDepartments gets the full list of Departments in the QuickBooks account.
func (c *Client) FindDepartments(ctx context.Context) ([]Department, error) {
	var resp struct {
		QueryResponse struct {
			Departments   []Department `json:"Department"`
			MaxResults    int
			StartPosition int
			TotalCount    int
		}
	}

	if err := c.query(ctx, "SELECT COUNT(*) FROM Department", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no departments could be found")
	}

	departments := make([]Department, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM Department ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(ctx, query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.Departments == nil {
			return nil, errors.New("no departments could be found")
		}

		departments = append(departments, resp.QueryResponse.Departments...)
	}

	return departments, nil
}

// FindDepartmentById finds the department by the given id.
func (c *Client) FindDepartmentById(ctx context.Context, id string) (*Department, error) {
	var resp struct {
		Department Department
		Time       Date
	}

	if err := c.get(ctx, "department/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Department, nil
}

// QueryDepartments accepts an SQL query and returns all departments found using it.
func (c *Client) QueryDepartments(ctx context.Context, query string) ([]Department, error) {
	var resp struct {
		QueryResponse struct {
			Departments   []Department `json:"Department"`
			StartPosition int
			MaxResults    int
		}
	}

	if err := c.query(ctx, query, &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.Departments == nil {
		return nil, errors.New("could not find any departments")
	}

	return resp.QueryResponse.Departments, nil
}

// UpdateDepartment updates the department.
func (c *Client) UpdateDepartment(ctx context.Context, department *Department) (*Department, error) {
	if department.Id == "" {
		return nil, errors.New("missing department id")
	}

	existingDepartment, err := c.FindDepartmentById(ctx, department.Id)
	if err != nil {
		return nil, err
	}

	department.SyncToken = existingDepartment.SyncToken

	payload := struct {
		*Department
		Sparse bool `json:"sparse"`
	}{
		Department: department,
		Sparse:     true,
	}

	var departmentData struct {
		Department Department
		Time       Date
	}

	if err = c.post(ctx, "department", payload, &departmentData, nil); err != nil {
		return nil, err
	}

	return &departmentData.Department, err
}
