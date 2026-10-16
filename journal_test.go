package quickbooks

import (
	"context"
	"testing"
)

func TestQueryJournalEntries_SendsTheQueryAndDecodesEntries(t *testing.T) {
	srv, gotPath, gotQuery := captureServer(t, `{"QueryResponse": {"JournalEntry": [{
		"Id": "301", "DocNumber": "SJ-2026-09-03", "TxnDate": "2026-09-03", "PrivateNote": "Sales",
		"Line": [{
			"Description": "Food sales", "Amount": 120.5, "DetailType": "JournalEntryLineDetail",
			"JournalEntryLineDetail": {
				"PostingType": "Credit",
				"AccountRef": {"value": "79", "name": "Sales"},
				"Entity": {"Type": "Customer", "EntityRef": {"value": "11", "name": "Walk-in"}},
				"ClassRef": {"value": "7", "name": "Kitchen"},
				"DepartmentRef": {"value": "2", "name": "Downtown"}
			}
		}]
	}], "startPosition": 1, "maxResults": 1}}`)
	c := newTestClient(t, srv.URL)
	query := "SELECT * FROM JournalEntry WHERE TxnDate >= '2026-09-01' STARTPOSITION 1 MAXRESULTS 1000"
	entries, err := c.QueryJournalEntries(context.Background(), query)
	if err != nil {
		t.Fatalf("QueryJournalEntries: %v", err)
	}
	if *gotPath != "/v3/company/test-realm/query" || gotQuery.Get("query") != query {
		t.Fatalf("sent %s?query=%q, want the query endpoint with the query as given", *gotPath, gotQuery.Get("query"))
	}
	if len(entries) != 1 || *entries[0].Id != "301" || *entries[0].DocNumber != "SJ-2026-09-03" {
		t.Fatalf("entries = %+v, want entry 301", entries)
	}
	detail := entries[0].Line[0].JournalEntryLineDetail
	if *detail.PostingType != PostingTypeCredit || detail.AccountRef.Value != "79" || detail.AccountRef.Name != "Sales" ||
		*detail.Entity.Type != EntityTypeTypeCustomer || detail.Entity.EntityRef.Value != "11" ||
		detail.ClassRef.Value != "7" || detail.DepartmentRef.Name != "Downtown" {
		t.Fatalf("line detail = %+v, want every reference decoded", detail)
	}
}

func TestQueryJournalEntries_NoMatchIsEmptyNotAnError(t *testing.T) {
	srv, _, _ := captureServer(t, `{"QueryResponse": {}}`)
	c := newTestClient(t, srv.URL)
	entries, err := c.QueryJournalEntries(context.Background(), "SELECT * FROM JournalEntry")
	if err != nil {
		t.Fatalf("no match must not be an error: %v", err)
	}
	if entries == nil || len(entries) != 0 {
		t.Fatalf("entries = %#v, want an empty slice", entries)
	}
}
