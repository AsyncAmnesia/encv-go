package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Soltus/encv-go/internal/admin"
	"github.com/Soltus/encv-go/internal/register"
	"github.com/Soltus/encv-go/pkg/encv"
	"github.com/spf13/cobra"
)

func addServersCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(startCmd)
	// rootCmd.AddCommand(webdavCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(adminCmd)
}

// --- start 命令 ---
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the ENCV server and keeps it running in the foreground",
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
		adminServer, proxyInstanceID := admin.SetupAdminServer(backendAddr, cfg)
		adminAddr, err := register.StartGfServerWithRetry(adminServer, cfg.Admin.Port, proxyInstanceID, Version)
		if err != nil {
			log.Fatalf("[%s] Failed to start admin service: %v", proxyInstanceID, err)
		}

		// 5. 打印信息
		log.Printf("\n✅ Server started successfully!\n")
		log.Printf("   Serving files from: %s\n", cfg.Server.Dir)
		log.Printf("   Backend is at: http://%s\n", backendAddr)
		log.Printf("   Admin panel is at: http://%s/admin/\n", adminAddr)
		log.Printf("   Proxy service is at: http://%s%s/\n", adminAddr, admin.AdminProxyPath)
		log.Println("\n--- How to Play ---")
		log.Printf("   mpv --no-config http://%s/p/<video_name_without_extension>\n", adminAddr)
		log.Println("\n(Press Ctrl+C in this terminal to stop the server)")

		select {} // Keep the main goroutine alive
	},
}

// --- server 命令 ---
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Starts the ENCV server as a background service",
	Run: func(cmd *cobra.Command, args []string) {
		encv.Init(rootCtx)
		s := encv.NewServer(rootCtx)
		_, err := s.Start(Version)
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
		log.Println("✅ ENCV Server is running.")
		log.Printf("-> You can now double-click files to open them.")
		log.Printf("-> To stop the server, run: encv.exe stop-server")
		log.Println("-> Press Ctrl+C to stop the server manually.")

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit
		log.Println("-> Shutting down server...")
		s.Stop()
	},
}

// --- admin 命令 ---
var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Starts the ENCV admin service only",
	Long: `Starts only the admin/proxy service, assuming the main backend server
is already running. This is useful for managing an existing server instance.`,
	Run: func(cmd *cobra.Command, args []string) {
		// 1. 【关键】查找已运行的后端服务
		backendAddr, _, err := encv.FindServer(cfg.Server.Port, 20)
		if err != nil {
			log.Fatalf("Failed to find a running ENCV backend server: %v", err)
		}
		log.Printf("-> Found backend server at %s", backendAddr)

		// 2. 【关键】检查是否已有管理服务在运行
		if _, err := register.FindAdminServer(cfg.Admin.Port, 20); err == nil {
			log.Fatalf("Admin service is already running. Please stop it first.")
		}

		// 3. 启动新的管理服务
		adminServer, proxyInstanceID := admin.SetupAdminServer(backendAddr, cfg)
		adminAddr, err := register.StartGfServerWithRetry(adminServer, cfg.Admin.Port, proxyInstanceID, Version)
		if err != nil {
			log.Fatalf("Failed to start admin service: %v", err)
		}

		// 4. 打印信息
		log.Printf("\n✅ Admin service started successfully!\n")
		log.Printf("   Backend is at: http://%s\n", backendAddr)
		log.Printf("   Admin panel is at: http://%s/admin/\n", adminAddr)
		log.Printf("   Proxy service is at: http://%s/p/\n", adminAddr)
		log.Println("\n(Press Ctrl+C in this terminal to stop the admin service)")

		select {}
	},
}
