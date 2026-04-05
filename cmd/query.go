package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"sql-query/internal/db"
	"sql-query/internal/errutil"
	"sql-query/internal/exporter"
	"sql-query/internal/parser"
)

var (
	sqlFile    string
	outputFile string
	excelFlag  bool
	htmlFlag   bool
	workers    int
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "执行 SQL 查询并导出结果",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate output format
		formatCount := 0
		if excelFlag {
			formatCount++
		}
		if htmlFlag {
			formatCount++
		}
		if jsonFlag {
			formatCount++
		}
		if formatCount == 0 {
			errutil.Exit(errutil.ExitGenericError, "invalid_argument",
				"请指定输出格式：--excel、--html 或 --json", jsonFlag)
		}
		if formatCount > 1 {
			errutil.Exit(errutil.ExitGenericError, "invalid_argument",
				"只能选择一种输出格式：--excel、--html 或 --json", jsonFlag)
		}

		// Set default output file if needed
		if outputFile == "" {
			if excelFlag {
				outputFile = "output.xlsx"
			} else if htmlFlag {
				outputFile = "output.html"
			}
			// JSON with no -o: write to stdout (outputFile stays "")
		}

		// Read SQL
		sqlContent, err := readSQL()
		if err != nil {
			errutil.Exit(errutil.ExitGenericError, "file_error",
				fmt.Sprintf("读取 SQL 失败: %s", err), jsonFlag)
		}
		if sqlContent == "" {
			errutil.Exit(errutil.ExitGenericError, "invalid_argument",
				"未提供 SQL 内容，请使用 -f 指定文件或通过 stdin 输入", jsonFlag)
		}

		// Execute SQL
		fmt.Fprintln(os.Stderr, "执行 SQL 查询...")
		queryStart := time.Now()
		columns, data, err := db.Execute(database, sqlContent, cfg.QueryTimeout)
		if err != nil {
			errutil.Exit(errutil.ExitGenericError, "sql_syntax_error",
				fmt.Sprintf("执行 SQL 失败: %s", err), jsonFlag)
		}
		fmt.Fprintf(os.Stderr, "查询完成，共 %d 列 %d 行 (耗时 %v)\n",
			len(columns), len(data), time.Since(queryStart).Round(time.Millisecond))

		// Parse metadata
		parsedColumns := parser.ParseColumns(columns)

		// Note: S3 presigning is Phase 2 — skipped for now.
		for _, col := range parsedColumns {
			if col.HasMeta("URL") {
				fmt.Fprintln(os.Stderr, "警告: 发现 [URL] 元数据但 S3 预签名尚未启用 (Phase 2)")
				break
			}
		}

		// Export
		var exp exporter.Exporter
		if excelFlag {
			fmt.Fprintf(os.Stderr, "导出 Excel -> %s\n", outputFile)
			exp = exporter.NewExcelExporter(outputFile)
		} else if htmlFlag {
			fmt.Fprintf(os.Stderr, "导出 HTML -> %s\n", outputFile)
			exp = exporter.NewHTMLExporter(outputFile)
		} else {
			if outputFile != "" {
				fmt.Fprintf(os.Stderr, "导出 JSON -> %s\n", outputFile)
			}
			exp = exporter.NewJSONExporter(outputFile)
		}

		if err := exp.Export(parsedColumns, data); err != nil {
			errutil.Exit(errutil.ExitGenericError, "file_error",
				fmt.Sprintf("导出失败: %s", err), jsonFlag)
		}

		fmt.Fprintln(os.Stderr, "完成")
		return nil
	},
}

func init() {
	queryCmd.Flags().StringVarP(&sqlFile, "file", "f", "", "SQL 文件路径（不指定则读取 stdin）")
	queryCmd.Flags().StringVarP(&outputFile, "output", "o", "", "输出文件路径")
	queryCmd.Flags().BoolVar(&excelFlag, "excel", false, "导出为 Excel 格式")
	queryCmd.Flags().BoolVar(&htmlFlag, "html", false, "导出为 HTML 格式")
	queryCmd.Flags().IntVarP(&workers, "workers", "w", runtime.NumCPU(), "并发处理数")
	rootCmd.AddCommand(queryCmd)
}

func readSQL() (string, error) {
	if sqlFile != "" {
		content, err := os.ReadFile(sqlFile)
		if err != nil {
			return "", err
		}
		return string(content), nil
	}

	// Check if stdin has data
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return "", nil
	}

	reader := bufio.NewReader(os.Stdin)
	var content []byte
	for {
		line, err := reader.ReadBytes('\n')
		content = append(content, line...)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return string(content), nil
}
