package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// sqlCmd represents the sql command
var sqlCmd = &cobra.Command{
	Use:   "sql <query>",
	Short: "Execute SQL directly against the Dolt database",
	Long: `Execute arbitrary SQL queries against the Cortex Dolt database.

Provides direct access to Dolt tables, system tables, and SQL functions.
Useful for advanced queries, debugging, and accessing Dolt-specific features.

Common Dolt system tables:
  dolt_log          Commit history
  dolt_branches     Branch information
  dolt_status       Working set status
  dolt_diff()       Compare changes between refs

Output formats:
  table   ASCII table format (default)
  yaml    YAML format
  json    JSON format

Examples:
  cx sql "SELECT * FROM entities LIMIT 5"
  cx sql "SELECT COUNT(*) FROM dependencies"
  cx sql "SELECT * FROM dolt_log LIMIT 10"
  cx sql "SELECT * FROM dolt_branches"
  cx sql "SELECT * FROM dolt_status"
  cx sql "CALL dolt_status()"
  cx sql "SELECT * FROM DOLT_DIFF('HEAD~1', 'HEAD', 'entities')"`,
	Args: cobra.ExactArgs(1),
	RunE: runSQL,
}

var (
	sqlOutputFormat string
)

func init() {
	rootCmd.AddCommand(sqlCmd)

	sqlCmd.Flags().StringVar(&sqlOutputFormat, "format", "table", "Output format: table|yaml|json")
}

// SQLResult represents the output of a SQL query
type SQLResult struct {
	Columns []string                 `yaml:"columns" json:"columns"`
	Rows    []map[string]interface{} `yaml:"rows" json:"rows"`
	Count   int                      `yaml:"count" json:"count"`
}

func runSQL(cmd *cobra.Command, args []string) error {
	query := args[0]

	// Find config directory
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return fmt.Errorf("cx not initialized: run 'cx scan' first")
	}

	// Open store
	st, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	// Execute the query
	db := st.DB()

	// Determine if this is a query (SELECT) or execution (INSERT, UPDATE, etc)
	trimmedQuery := strings.TrimSpace(strings.ToUpper(query))
	isQuery := strings.HasPrefix(trimmedQuery, "SELECT") ||
		strings.HasPrefix(trimmedQuery, "SHOW") ||
		strings.HasPrefix(trimmedQuery, "DESCRIBE") ||
		strings.HasPrefix(trimmedQuery, "EXPLAIN")

	if isQuery {
		return runSQLQuery(cmd, db, query)
	}
	return runSQLExec(cmd, db, query)
}

func runSQLQuery(cmd *cobra.Command, db *sql.DB, query string) error {
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("get columns: %w", err)
	}

	// Collect results
	result := SQLResult{
		Columns: columns,
		Rows:    make([]map[string]interface{}, 0),
	}

	// Create scan destination
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		err := rows.Scan(valuePtrs...)
		if err != nil {
			return fmt.Errorf("scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Convert byte slices to strings for readability
			if b, ok := val.([]byte); ok {
				val = string(b)
			}
			row[col] = val
		}
		result.Rows = append(result.Rows, row)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate rows: %w", err)
	}

	result.Count = len(result.Rows)

	// Output based on format
	switch sqlOutputFormat {
	case "yaml":
		return outputYAML(cmd, result)
	case "json":
		return outputJSON(cmd, result)
	default:
		return outputTable(cmd, result)
	}
}

func runSQLExec(cmd *cobra.Command, db *sql.DB, query string) error {
	result, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("exec error: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	lastInsertID, _ := result.LastInsertId()

	out := cmd.OutOrStdout()

	switch sqlOutputFormat {
	case "yaml":
		data := map[string]interface{}{
			"rows_affected":  rowsAffected,
			"last_insert_id": lastInsertID,
		}
		enc := yaml.NewEncoder(out)
		enc.SetIndent(2)
		return enc.Encode(data)
	case "json":
		data := map[string]interface{}{
			"rows_affected":  rowsAffected,
			"last_insert_id": lastInsertID,
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	default:
		fmt.Fprintf(out, "Query OK, %d rows affected\n", rowsAffected)
		return nil
	}
}

func outputTable(cmd *cobra.Command, result SQLResult) error {
	out := cmd.OutOrStdout()

	if len(result.Rows) == 0 {
		fmt.Fprintln(out, "Empty set")
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	// Header
	headers := make([]string, len(result.Columns))
	for i, col := range result.Columns {
		headers[i] = col
	}
	fmt.Fprintln(w, strings.Join(headers, "\t"))

	// Separator
	seps := make([]string, len(result.Columns))
	for i, col := range result.Columns {
		seps[i] = strings.Repeat("-", len(col))
	}
	fmt.Fprintln(w, strings.Join(seps, "\t"))

	// Rows
	for _, row := range result.Rows {
		vals := make([]string, len(result.Columns))
		for i, col := range result.Columns {
			val := row[col]
			if val == nil {
				vals[i] = "NULL"
			} else {
				vals[i] = fmt.Sprintf("%v", val)
			}
		}
		fmt.Fprintln(w, strings.Join(vals, "\t"))
	}

	w.Flush()
	fmt.Fprintf(out, "\n%d rows in set\n", result.Count)
	return nil
}

func outputYAML(cmd *cobra.Command, result SQLResult) error {
	formatter, err := output.GetFormatter(output.FormatYAML)
	if err != nil {
		return err
	}
	return formatter.FormatToWriter(cmd.OutOrStdout(), result, output.DensityMedium)
}

func outputJSON(cmd *cobra.Command, result SQLResult) error {
	formatter, err := output.GetFormatter(output.FormatJSON)
	if err != nil {
		return err
	}
	return formatter.FormatToWriter(cmd.OutOrStdout(), result, output.DensityMedium)
}
