package quickbooks

type EntityTypeType string

const (
	EntityTypeTypeVendor   EntityTypeType = "Vendor"
	EntityTypeTypeEmployee EntityTypeType = "Employee"
	EntityTypeTypeCustomer EntityTypeType = "Customer"
)

type PostingType string

const (
	PostingTypeDebit  PostingType = "Debit"
	PostingTypeCredit PostingType = "Credit"
)

// Entity type
type EntityType struct {
	// Type of the entity.
	Type      *EntityTypeType     `json:"Type,omitempty" url:"Type,omitempty"`
	EntityRef *BasicReferenceType `json:"EntityRef,omitempty" url:"EntityRef,omitempty"`
}

type JournalEntryLineItemDetail struct {
	// Posting type
	PostingType   *PostingType        `json:"PostingType,omitempty" url:"PostingType,omitempty"`
	AccountRef    *BasicReferenceType `json:"AccountRef,omitempty" url:"AccountRef,omitempty"`
	Entity        *EntityType         `json:"Entity,omitempty" url:"Entity,omitempty"`
	ClassRef      *BasicReferenceType `json:"ClassRef,omitempty" url:"ClassRef,omitempty"`
	DepartmentRef *BasicReferenceType `json:"DepartmentRef,omitempty" url:"DepartmentRef,omitempty"`
	TaxCodeRef    *BasicReferenceType `json:"TaxCodeRef,omitempty" url:"TaxCodeRef,omitempty"`
	// Tax amount of the line item.
	TaxAmount *float64 `json:"TaxAmount,omitempty" url:"TaxAmount,omitempty"`
}

type JournalEntryLineItem struct {
	// Unique identifier for this object. Sort order is ASC by default.
	Id *string `json:"Id,omitempty" url:"Id,omitempty"`
	// Description of the line item.
	Description *string `json:"Description,omitempty" url:"Description,omitempty"`
	// Total amount of the line item.
	Amount *float64 `json:"Amount,omitempty" url:"Amount,omitempty"`
	// Detail type of the line item.
	DetailType *string `json:"DetailType,omitempty" url:"DetailType,omitempty"`
	// Line number of the line item.
	LineNum                *float64                    `json:"LineNum,omitempty" url:"LineNum,omitempty"`
	JournalEntryLineDetail *JournalEntryLineItemDetail `json:"JournalEntryLineDetail,omitempty" url:"JournalEntryLineDetail,omitempty"`
}

type JournalEntry struct {
	// Unique identifier for this object. Sort order is ASC by default.
	Id *string `json:"Id,omitempty" url:"Id,omitempty"`
	// Version number of the object. It is used to lock the object for use by one app at a time.
	SyncToken *string `json:"SyncToken,omitempty" url:"SyncToken,omitempty"`
	// Date and time when the journal entry was created.
	CreateDate *string `json:"createDate,omitempty" url:"createDate,omitempty"`
	// List of line items in the journal entry.
	Line []*JournalEntryLineItem `json:"Line,omitempty" url:"Line,omitempty"`
	// Document number of the journal entry.
	DocNumber *string `json:"DocNumber,omitempty" url:"DocNumber,omitempty"`
	// Private note of the journal entry.
	PrivateNote *string `json:"PrivateNote,omitempty" url:"PrivateNote,omitempty"`
	// Transaction date of the journal entry.
	TxnDate    *string             `json:"TxnDate,omitempty" url:"TxnDate,omitempty"`
	TaxRateRef *BasicReferenceType `json:"TaxRateRef,omitempty" url:"TaxRateRef,omitempty"`
	// Total amount of the journal entry.
	TotalAmt *float64 `json:"TotalAmt,omitempty" url:"TotalAmt,omitempty"`
	// Descriptive information about the object. The MetaData values are set by Data Services and are read only for all applications.
	MetaData *MetaData `json:"MetaData,omitempty" url:"MetaData,omitempty"`
}

// CreateBill creates the given Bill on the QuickBooks server, returning
// the resulting Bill object.
func (c *Client) CreateJournalEntry(req *JournalEntry) (*JournalEntry, error) {
	var resp struct {
		JournalEntry JournalEntry
		Time         Date
	}

	if err := c.post("journalentry", req, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.JournalEntry, nil
}
