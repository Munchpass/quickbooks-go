package quickbooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
)

type ContentType string

const (
	AI   ContentType = "application/postscript"
	CSV  ContentType = "text/csv"
	DOC  ContentType = "application/msword"
	DOCX ContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	EPS  ContentType = "application/postscript"
	GIF  ContentType = "image/gif"
	JPEG ContentType = "image/jpeg"
	JPG  ContentType = "image/jpg"
	ODS  ContentType = "application/vnd.oasis.opendocument.spreadsheet"
	PDF  ContentType = "application/pdf"
	PNG  ContentType = "image/png"
	RTF  ContentType = "text/rtf"
	TIF  ContentType = "image/tif"
	TXT  ContentType = "text/plain"
	XLS  ContentType = "application/vnd/ms-excel"
	XLSX ContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	XML  ContentType = "text/xml"
)

type Attachable struct {
	Id                       string          `json:"Id,omitempty"`
	SyncToken                string          `json:",omitempty"`
	FileName                 string          `json:",omitempty"`
	Note                     string          `json:",omitempty"`
	Category                 string          `json:",omitempty"`
	ContentType              ContentType     `json:",omitempty"`
	PlaceName                string          `json:",omitempty"`
	AttachableRef            []AttachableRef `json:",omitempty"`
	Long                     string          `json:",omitempty"`
	Tag                      string          `json:",omitempty"`
	Lat                      string          `json:",omitempty"`
	MetaData                 MetaData        `json:",omitempty"`
	FileAccessUri            string          `json:",omitempty"`
	Size                     json.Number     `json:",omitempty"`
	ThumbnailFileAccessUri   string          `json:",omitempty"`
	TempDownloadUri          string          `json:",omitempty"`
	ThumbnailTempDownloadUri string          `json:",omitempty"`
}

type AttachableRef struct {
	IncludeOnSend bool   `json:",omitempty"`
	LineInfo      string `json:",omitempty"`
	NoRefOnly     bool   `json:",omitempty"`
	// CustomField[0..n]
	Inactive  bool          `json:",omitempty"`
	EntityRef ReferenceType `json:",omitempty"`
}

// CreateAttachable creates the given Attachable on the QuickBooks server,
// returning the resulting Attachable object.
func (c *Client) CreateAttachable(ctx context.Context, attachable *Attachable) (*Attachable, error) {
	var resp struct {
		Attachable Attachable
		Time       Date
	}

	if err := c.post(ctx, "attachable", attachable, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Attachable, nil
}

// DeleteAttachable deletes the attachable
func (c *Client) DeleteAttachable(ctx context.Context, attachable *Attachable) error {
	if attachable.Id == "" || attachable.SyncToken == "" {
		return errors.New("missing id/sync token")
	}

	return c.post(ctx, "attachable", attachable, nil, map[string]string{"operation": "delete"})
}

// DownloadAttachable downloads the attachable
func (c *Client) DownloadAttachable(ctx context.Context, id string) (string, error) {
	endpointUrl := *c.endpoint
	endpointUrl.Path += "download/" + id

	urlValues := url.Values{}
	urlValues.Add("minorversion", c.minorVersion)
	endpointUrl.RawQuery = urlValues.Encode()

	resp, err := c.doWithThrottle(ctx, func() (*http.Request, error) {
		return http.NewRequestWithContext(ctx, "GET", endpointUrl.String(), nil)
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", parseFailure(resp)
	}

	downloadUrl, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(downloadUrl), err
}

// FindAttachables gets the full list of Attachables in the QuickBooks attachable.
func (c *Client) FindAttachables(ctx context.Context) ([]Attachable, error) {
	var resp struct {
		QueryResponse struct {
			Attachables   []Attachable `json:"Attachable"`
			MaxResults    int
			StartPosition int
			TotalCount    int
		}
	}

	if err := c.query(ctx, "SELECT COUNT(*) FROM Attachable", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no attachables could be found")
	}

	attachables := make([]Attachable, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM Attachable ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(ctx, query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.Attachables == nil {
			return nil, errors.New("no attachables could be found")
		}

		attachables = append(attachables, resp.QueryResponse.Attachables...)
	}

	return attachables, nil
}

// FindAttachableById finds the attachable by the given id
func (c *Client) FindAttachableById(ctx context.Context, id string) (*Attachable, error) {
	var resp struct {
		Attachable Attachable
		Time       Date
	}

	if err := c.get(ctx, "attachable/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Attachable, nil
}

// QueryAttachables accepts an SQL query and returns all attachables found using it
func (c *Client) QueryAttachables(ctx context.Context, query string) ([]Attachable, error) {
	var resp struct {
		QueryResponse struct {
			Attachables   []Attachable `json:"Attachable"`
			StartPosition int
			MaxResults    int
		}
	}

	if err := c.query(ctx, query, &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.Attachables == nil {
		return nil, errors.New("could not find any attachables")
	}

	return resp.QueryResponse.Attachables, nil
}

// UpdateAttachable updates the attachable
func (c *Client) UpdateAttachable(ctx context.Context, attachable *Attachable) (*Attachable, error) {
	if attachable.Id == "" {
		return nil, errors.New("missing attachable id")
	}

	existingAttachable, err := c.FindAttachableById(ctx, attachable.Id)
	if err != nil {
		return nil, err
	}

	attachable.SyncToken = existingAttachable.SyncToken

	payload := struct {
		*Attachable
		Sparse bool `json:"sparse"`
	}{
		Attachable: attachable,
		Sparse:     true,
	}

	var attachableData struct {
		Attachable Attachable
		Time       Date
	}

	if err = c.post(ctx, "attachable", payload, &attachableData, nil); err != nil {
		return nil, err
	}

	return &attachableData.Attachable, err
}

// UploadAttachable uploads the attachable
func (c *Client) UploadAttachable(ctx context.Context, attachable *Attachable, data io.Reader) (*Attachable, error) {
	endpointUrl := *c.endpoint
	endpointUrl.Path += "upload"

	urlValues := url.Values{}
	urlValues.Add("minorversion", c.minorVersion)
	endpointUrl.RawQuery = urlValues.Encode()

	var buffer bytes.Buffer
	mWriter := multipart.NewWriter(&buffer)

	// Add file metadata
	metadataHeader := make(textproto.MIMEHeader)
	metadataHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "file_metadata_01", "attachment.json"))
	metadataHeader.Set("Content-Type", "application/json")

	metadataContent, err := mWriter.CreatePart(metadataHeader)
	if err != nil {
		return nil, err
	}

	j, err := json.Marshal(attachable)
	if err != nil {
		return nil, err
	}

	if _, err = metadataContent.Write(j); err != nil {
		return nil, err
	}

	// Add file content
	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "file_content_01", attachable.FileName))
	fileHeader.Set("Content-Type", string(attachable.ContentType))

	fileContent, err := mWriter.CreatePart(fileHeader)
	if err != nil {
		return nil, err
	}

	if _, err = io.Copy(fileContent, data); err != nil {
		return nil, err
	}

	mWriter.Close()

	bodyBytes := buffer.Bytes()
	contentType := mWriter.FormDataContentType()

	resp, err := c.doWithThrottle(ctx, func() (*http.Request, error) {
		r, err := http.NewRequestWithContext(ctx, "POST", endpointUrl.String(), bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		r.Header.Add("Content-Type", contentType)
		r.Header.Add("Accept", "application/json")
		return r, nil
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseFailure(resp)
	}

	var r struct {
		AttachableResponse []struct {
			Attachable Attachable
		}
		Time Date
	}

	if err = json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	return &r.AttachableResponse[0].Attachable, nil
}
