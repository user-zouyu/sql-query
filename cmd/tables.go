package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"sql-query/internal/db"
	"sql-query/internal/errutil"
)

var tablesCmd = &cobra.Command{
	Use:   "tables",
	Short: "列出当前数据库的所有表",
	RunE: func(cmd *cobra.Command, args []string) error {
		tables, err := db.GetTables(database)
		if err != nil {
			errutil.Exit(errutil.ExitGenericError, "sql_syntax_error",
				fmt.Sprintf("查询表列表失败: %s", err), jsonFlag)
		}

		if jsonFlag {
			result := struct {
				Count  int            `json:"count"`
				Tables []db.TableInfo `json:"tables"`
			}{Count: len(tables), Tables: tables}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			return enc.Encode(result)
		}

		fmt.Printf("Found %d tables:\n", len(tables))
		for _, t := range tables {
			comment := ""
			if t.TableComment != "" {
				comment = " - " + t.TableComment
			}
			fmt.Printf("  %-30s %8d rows%s\n", t.TableName, t.TableRows, comment)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tablesCmd)
}
