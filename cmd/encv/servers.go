package main

import (
	"fmt"
	"log"

	"github.com/Soltus/encv-go/internal/admin"
	"github.com/Soltus/encv-go/internal/admin/routes"
	"github.com/Soltus/encv-go/internal/register"
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/pkg/encv"
	"github.com/spf13/cobra"
)

func addServersCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(startCmd)
	// rootCmd.AddCommand(serverCmd)
	// rootCmd.AddCommand(adminCmd)
	// rootCmd.AddCommand(proxyCmd)
}

// --- start 命令 ---
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts all ENCV servers and keeps them running in the foreground",
	Run: func(cmd *cobra.Command, args []string) {
		// 1. 初始化并启动后端服务
		encv.Init(rootCtx)
		s := encv.NewServer(rootCtx)
		backendAddr, err := s.Start(Version)
		if err != nil {
			log.Fatalf("Failed to start backend server: %v", err)
		}

		// 2. 【关键】检查是否已有管理服务在运行
		if _, err := register.FindAdminServer(cfg.Admin.Port, 20); err == nil {
			log.Fatalf("Admin service is already running. Please stop it first or use the existing one.")
		}

		// 3. 启动新的管理服务
		adminServer, proxyInstanceID := admin.SetupAdminServer(backendAddr, rootCtx)
		adminAddr, err := register.StartGfServerWithRetry(adminServer, cfg.Admin.Port, proxyInstanceID, Version)
		if err != nil {
			log.Fatalf("[%s] Failed to start admin service: %v", proxyInstanceID, err)
		}

		// 5. 打印启动信息（使用 pterm 美化输出）
		utils.PrintSection("Server Started")
		utils.PrintKV("Serving files", cfg.Server.Dir)
		utils.PrintKV("FS endpoint", fmt.Sprintf("http://%s", backendAddr))
		utils.PrintKV("WebDAV endpoint", fmt.Sprintf("http://%s/%s", backendAddr, cfg.Webdav.Root))
		utils.PrintKV("FS Proxy endpoint", fmt.Sprintf("http://%s%s/", adminAddr, routes.FSProxy))
		// 【修改】多站点代理信息（现在集成在管理服务中）
		if len(cfg.Proxy.Sites) > 0 {
			utils.PrintKV("OpenList Sites", fmt.Sprintf("http://%s%s/sites", adminAddr, routes.OpenListProxy))
		}
		utils.PrintSection("How to Play")
		utils.PrintInfo("mpv --no-config http://%s/p/<video_name_without_extension>", adminAddr)
		utils.PrintInfo("Press Ctrl+C to stop the server")

		select {} // Keep the main goroutine alive
	},
}
