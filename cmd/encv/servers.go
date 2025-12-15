package main

import (
	"log"

	"github.com/Soltus/encv-go/internal/admin"
	"github.com/Soltus/encv-go/internal/admin/routes"
	"github.com/Soltus/encv-go/internal/register"
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

		// 5. 打印信息
		log.Printf("\n✅ Server started successfully!\n")
		log.Printf("   Serving files from: %s\n", cfg.Server.Dir)
		log.Printf("   FS is at: http://%s\n", backendAddr)
		log.Printf("   Webdav is at: http://%s/%s\n", backendAddr, cfg.Webdav.Root)
		log.Printf("   FS Proxy is at: http://%s%s/\n", adminAddr, routes.FSProxy)
		// 【修改】多站点代理信息（现在集成在管理服务中）
		if len(cfg.Proxy.Sites) > 0 {
			log.Printf("   OpenList Sites Management: http://%s%s/sites\n", adminAddr, routes.OpenListProxy)
		}
		log.Println("\n--- How to Play ---")
		log.Printf("   mpv --no-config http://%s/p/<video_name_without_extension>\n", adminAddr)
		log.Println("\n(Press Ctrl+C in this terminal to stop the server)")

		select {} // Keep the main goroutine alive
	},
}
