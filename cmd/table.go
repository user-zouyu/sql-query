package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"sql-query/internal/db"
	"sql-query/internal/errutil"
)

var tableCmd = &cobra.Command{
	Use:   "table <name>",
	Short: "查看指定表的 DDL 和结构信息",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tableName := args[0]

		ddl, err := db.GetTableDDL(database, tableName)
		if err != nil {
			errutil.Exit(errutil.ExitGenericError, "sql_syntax_error",
				fmt.Sprintf("查询表结构失败: %s", err), jsonFlag)
		}
		if ddl == nil {
			errutil.Exit(errutil.ExitTableNotFound, "table_not_found",
				fmt.Sprintf("表 '%s' 不存在", tableName), jsonFlag)
		}

		if jsonFlag {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			return enc.Encode(ddl)
		}

		fmt.Println(ddl.RawDDL)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tableCmd)
}
