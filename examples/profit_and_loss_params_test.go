package examples

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/rwestlund/quickbooks-go/v2"
	"github.com/stretchr/testify/require"
)

// These run against a live QuickBooks company. Use a sandbox company with
// Location tracking and Class tracking turned on, and some transactions tagged
// with Locations and Classes (and some left untagged) in the date range.
const (
	pnlParamsStart = "2025-01-01"
	pnlParamsEnd   = "2025-09-30"
)

func newPnLParamsClient(t *testing.T) *quickbooks.Client {
	t.Helper()
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
	return qbClient
}

func pnlParams() quickbooks.PnLReportParams {
	return quickbooks.PnLReportParams{StartDate: pnlParamsStart, EndDate: pnlParamsEnd}
}

// netIncomeByColumn reads the top-level NetIncome row: one amount per money
// column, keyed by column key, plus the column titles for logging.
func netIncomeByColumn(t *testing.T, report *quickbooks.Report) (map[string]float64, map[string]string) {
	t.Helper()
	var summary []quickbooks.ReportCell
	for _, row := range report.Rows.Row {
		if row.Group == "NetIncome" {
			summary = row.Summary.ColData
		}
	}
	require.NotEmpty(t, summary, "the report has no NetIncome row; is there activity in %s..%s?", pnlParamsStart, pnlParamsEnd)
	require.Len(t, summary, len(report.Columns.Column), "every column must have a NetIncome cell")

	amounts := map[string]float64{}
	titles := map[string]string{}
	for i, col := range report.Columns.Column {
		if col.ColType != "Money" {
			continue
		}
		// ColKey is the Department or Class id on a grouped report.
		var key string
		for _, meta := range col.MetaData {
			if meta.Name == "ColKey" {
				key = meta.Value
			}
		}
		amounts[key] = parseAmount(t, summary[i].Value)
		titles[key] = col.ColTitle
	}
	return amounts, titles
}

func parseAmount(t *testing.T, value string) float64 {
	t.Helper()
	if strings.TrimSpace(value) == "" {
		return 0
	}
	amount, err := strconv.ParseFloat(value, 64)
	require.NoError(t, err, "amount %q", value)
	return amount
}

func netIncome(t *testing.T, report *quickbooks.Report) float64 {
	t.Helper()
	amounts, _ := netIncomeByColumn(t, report)
	require.Len(t, amounts, 1, "an ungrouped report must have one money column")
	for _, amount := range amounts {
		return amount
	}
	return 0
}

func requireCents(t *testing.T, want, got float64, msgAndArgs ...any) {
	t.Helper()
	require.InDelta(t, want, got, 0.005, msgAndArgs...)
}

// dimension is one way to group and filter a P&L: Departments (Locations) or
// Classes.
type dimension struct {
	name   string
	by     quickbooks.SummarizeColumnBy
	ids    []string
	filter func(params *quickbooks.PnLReportParams, ids []string)
}

func TestFindPnLReportWithParams_Live(t *testing.T) {
	ctx := context.Background()
	qbClient := newPnLParamsClient(t)

	whole, err := qbClient.FindPnLReportWithParams(ctx, pnlParams())
	require.NoError(t, err)
	wholeNetIncome := netIncome(t, whole)
	t.Logf("whole-company net income: %.2f", wholeNetIncome)

	departments, err := qbClient.FindDepartments(ctx)
	require.NoError(t, err)
	classes, err := qbClient.FindClasses(ctx)
	require.NoError(t, err)

	dimensions := []dimension{
		{
			name: "Departments",
			by:   quickbooks.SummarizeByDepartments,
			filter: func(params *quickbooks.PnLReportParams, ids []string) {
				params.DepartmentIds = ids
			},
		},
		{
			name: "Classes",
			by:   quickbooks.SummarizeByClasses,
			filter: func(params *quickbooks.PnLReportParams, ids []string) {
				params.ClassIds = ids
			},
		},
	}
	for _, d := range departments {
		dimensions[0].ids = append(dimensions[0].ids, d.Id)
	}
	for _, c := range classes {
		dimensions[1].ids = append(dimensions[1].ids, c.Id)
	}

	for _, dim := range dimensions {
		t.Run(dim.name, func(t *testing.T) {
			if len(dim.ids) == 0 {
				t.Skipf("the company has no %s; turn on that tracking and tag some transactions", dim.name)
			}
			known := map[string]bool{}
			for _, id := range dim.ids {
				known[id] = true
			}

			params := pnlParams()
			params.SummarizeColumnBy = dim.by
			grouped, err := qbClient.FindPnLReportWithParams(ctx, params)
			require.NoError(t, err)
			require.Equal(t, string(dim.by), grouped.Header.SummarizeColumnsBy)

			byColumn, titles := netIncomeByColumn(t, grouped)
			var total float64
			var totalKey, untaggedKey string
			var taggedKeys []string
			for key, amount := range byColumn {
				t.Logf("column %q (key %q): net income %.2f", titles[key], key, amount)
				switch {
				case known[key]:
					taggedKeys = append(taggedKeys, key)
				case strings.EqualFold(titles[key], "Total"):
					totalKey, total = key, amount
				default:
					require.Empty(t, untaggedKey,
						"more than one column is neither a known %s id nor Total: %q and %q",
						dim.name, titles[untaggedKey], titles[key])
					untaggedKey = key
				}
			}
			require.NotEmpty(t, totalKey, "grouped report must end with a Total column")
			require.NotEmpty(t, taggedKeys,
				"no column key matched a %s id; the API matches columns to ids by ColKey", dim.name)
			t.Logf("untagged column: %q (key %q)", titles[untaggedKey], untaggedKey)

			t.Run("Total equals the ungrouped report", func(t *testing.T) {
				requireCents(t, wholeNetIncome, total, "grouping must not change the company total")
			})

			t.Run("columns add up to Total", func(t *testing.T) {
				var sum float64
				for key, amount := range byColumn {
					if key != totalKey {
						sum += amount
					}
				}
				requireCents(t, total, sum, "every line lands in exactly one column")
			})

			t.Run("filtering to one id equals its column", func(t *testing.T) {
				id := taggedKeys[0]
				params := pnlParams()
				dim.filter(&params, []string{id})
				filtered, err := qbClient.FindPnLReportWithParams(ctx, params)
				require.NoError(t, err)
				requireCents(t, byColumn[id], netIncome(t, filtered),
					"filtered to %q must equal its grouped column", titles[id])
			})

			t.Run("comma-separated ids add their columns", func(t *testing.T) {
				if len(taggedKeys) < 2 {
					t.Skipf("needs activity on two %s", dim.name)
				}
				ids := taggedKeys[:2]
				params := pnlParams()
				dim.filter(&params, ids)
				filtered, err := qbClient.FindPnLReportWithParams(ctx, params)
				require.NoError(t, err)
				requireCents(t, byColumn[ids[0]]+byColumn[ids[1]], netIncome(t, filtered),
					"filtered to %q and %q", titles[ids[0]], titles[ids[1]])
			})

			// The Garde API relies on this: a filter keeps only tagged lines,
			// so subtracting a filter over every id leaves the untagged lines.
			t.Run("a filter drops untagged lines", func(t *testing.T) {
				params := pnlParams()
				dim.filter(&params, dim.ids)
				everyId, err := qbClient.FindPnLReportWithParams(ctx, params)
				require.NoError(t, err)
				untagged := wholeNetIncome - netIncome(t, everyId)
				if untaggedKey == "" {
					requireCents(t, 0, untagged,
						"with no untagged column, filtering to every id must keep everything")
					return
				}
				requireCents(t, byColumn[untaggedKey], untagged,
					"whole minus every-id filter must equal the untagged column %q", titles[untaggedKey])
			})
		})
	}

	t.Run("Departments and Classes together", func(t *testing.T) {
		if len(dimensions[0].ids) == 0 || len(dimensions[1].ids) == 0 {
			t.Skip("needs both Departments and Classes")
		}
		params := pnlParams()
		params.DepartmentIds = dimensions[0].ids[:1]
		params.ClassIds = dimensions[1].ids[:1]
		report, err := qbClient.FindPnLReportWithParams(ctx, params)
		require.NoError(t, err)
		t.Logf("Department %s and Class %s together: net income %.2f",
			dimensions[0].ids[0], dimensions[1].ids[0], netIncome(t, report))
	})

	t.Run("accounting method", func(t *testing.T) {
		for _, method := range []quickbooks.AccountingMethod{quickbooks.AccountingMethodCash, quickbooks.AccountingMethodAccrual} {
			params := pnlParams()
			params.AccountingMethod = method
			report, err := qbClient.FindPnLReportWithParams(ctx, params)
			require.NoError(t, err)
			require.Equal(t, string(method), report.Header.ReportBasis)
			t.Logf("%s net income: %.2f", method, netIncome(t, report))
		}
	})

	t.Run("detail report", func(t *testing.T) {
		params := pnlParams()
		params.Columns = []string{"tx_date", "txn_type", "doc_num", "name", "dept_name", "klass_name", "subt_nat_amount"}
		params.Extra = map[string]string{"sort_order": "ascend"}
		detail, err := qbClient.FindPnLDetailReport(ctx, params)
		require.NoError(t, err)
		require.Equal(t, "ProfitAndLossDetail", detail.Header.ReportName)

		var types []string
		for _, col := range detail.Columns.Column {
			types = append(types, col.ColType+"("+col.ColTitle+")")
		}
		t.Logf("detail columns: %s", strings.Join(types, ", "))
		require.Len(t, detail.Columns.Column, len(params.Columns),
			"QuickBooks must return exactly the requested columns; a missing one means a column name is wrong")
	})
}
