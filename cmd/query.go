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
	"sql-query/internal/log"
	"sql-query/internal/parser"
	"sql-query/internal/processor"
)

var (
	sqlFile    string
	outputFile string
	excelFlag  bool
	htmlFlag   bool
	workers    int
	maxRows    int
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

		// Validate SQL safety — code-level keyword check (defense in depth)
		if err := db.ValidateReadOnly(sqlContent); err != nil {
			errutil.Exit(errutil.ExitGenericError, "sql_rejected",
				fmt.Sprintf("SQL 安全校验失败: %s", err), jsonFlag)
		}

		// Execute SQL
		log.Info("执行 SQL 查询...")
		log.Debug("SQL: %s", sqlContent)
		queryStart := time.Now()
		columns, data, err := db.Execute(database, sqlContent, cfg.QueryTimeout, maxRows)
		if err != nil {
			errutil.Exit(errutil.ExitGenericError, "sql_syntax_error",
				fmt.Sprintf("执行 SQL 失败: %s", err), jsonFlag)
		}
		log.Info("查询完成，共 %d 列 %d 行 (耗时 %v)",
			len(columns), len(data), time.Since(queryStart).Round(time.Millisecond))

		// Parse metadata
		parsedColumns := parser.ParseColumns(columns)

		// S3 presign: replace bucket:key values with presigned URLs
		urlCount := 0
		for _, col := range parsedColumns {
			if col.HasMeta("URL") {
				urlCount++
			}
		}
		if urlCount > 0 {
			if !cfg.HasS3Config() {
				errutil.Exit(errutil.ExitGenericError, "invalid_argument",
					"存在 [URL] 元数据但未配置 S3：需要 S3_ACCESS_KEY, S3_SECRET_KEY, S3_REGION", jsonFlag)
			}
			log.Info("S3 预签名处理中 (workers: %d)...", workers)
			processStart := time.Now()
			if err := processor.Process(cfg, parsedColumns, data, workers); err != nil {
				errutil.Exit(errutil.ExitGenericError, "s3_presign_failed",
					fmt.Sprintf("S3 预签名失败: %s", err), jsonFlag)
			}
			log.Info("预签名处理完成 (耗时 %v)", time.Since(processStart).Round(time.Millisecond))
		}

		// Export
		var exp exporter.Exporter
		if excelFlag {
			log.Info("导出 Excel -> %s", outputFile)
			exp = exporter.NewExcelExporter(outputFile)
		} else if htmlFlag {
			log.Info("导出 HTML -> %s", outputFile)
			exp = exporter.NewHTMLExporter(outputFile)
		} else {
			if outputFile != "" {
				log.Info("导出 JSON -> %s", outputFile)
			}
			exp = exporter.NewJSONExporter(outputFile)
		}

		if err := exp.Export(parsedColumns, data); err != nil {
			errutil.Exit(errutil.ExitGenericError, "file_error",
				fmt.Sprintf("导出失败: %s", err), jsonFlag)
		}

		log.Info("完成")
		return nil
	},
}

func init() {
	queryCmd.Flags().StringVarP(&sqlFile, "file", "f", "", "SQL 文件路径（不指定则读取 stdin）")
	queryCmd.Flags().StringVarP(&outputFile, "output", "o", "", "输出文件路径")
	queryCmd.Flags().BoolVar(&excelFlag, "excel", false, "导出为 Excel 格式")
	queryCmd.Flags().BoolVar(&htmlFlag, "html", false, "导出为 HTML 格式")
	queryCmd.Flags().IntVarP(&workers, "workers", "w", runtime.NumCPU(), "并发处理数")
	queryCmd.Flags().IntVar(&maxRows, "max-rows", 0, "最大返回行数（0 表示不限制）")
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
