package cmd

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"sql-query/internal/config"
	"sql-query/internal/db"
	"sql-query/internal/errutil"
)

var (
	envFile  string
	jsonFlag bool

	// Shared state set by PersistentPreRunE
	cfg      *config.Config
	database *gorm.DB
)

var rootCmd = &cobra.Command{
	Use:   "sql-query",
	Short: "MySQL 数据库结构查询与 SQL 导出工具",
	Long:  "sql-query 是一个 CLI 工具，提供 MySQL 数据库结构查询和 SQL 查询导出能力，专为 AI skills 设计。",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip DB connection for help and completion commands
		if cmd.Name() == "help" || cmd.Name() == "completion" ||
			cmd.Flags().Changed("help") {
			return nil
		}

		if envFile != "" {
			if err := godotenv.Load(envFile); err != nil {
				errutil.Exit(errutil.ExitGenericError, "file_error",
					fmt.Sprintf(".env 文件加载失败: %s", err), jsonFlag)
			}
		}

		var err error
		cfg, err = config.Load()
		if err != nil {
			errutil.Exit(errutil.ExitGenericError, "invalid_argument",
				err.Error(), jsonFlag)
		}

		fmt.Fprintln(os.Stderr, "连接数据库...")
		database, err = db.Connect(cfg.DBDSN)
		if err != nil {
			errutil.Exit(errutil.ExitConnectFailed, "connection_failed",
				fmt.Sprintf("数据库连接失败: %s", err), jsonFlag)
		}
		fmt.Fprintln(os.Stderr, "数据库连接成功")

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&envFile, "env", "e", "", ".env 配置文件路径")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "以 JSON 格式输出")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(errutil.ExitGenericError)
	}
}
