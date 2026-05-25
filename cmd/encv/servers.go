package main

import (
	"fmt"
	"log"

	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/pkg/encv"
	"github.com/spf13/cobra"
)

func addServersCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts ENCV server and keeps it running in the foreground",
	Run: func(cmd *cobra.Command, args []string) {
		encv.Init(rootCtx)
		s := encv.NewServer(rootCtx, configPath)
		backendAddr, err := s.Start(Version)
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}

		utils.PrintSection("Server Started")
		utils.PrintKV("Serving files", cfg.Server.Dir)
		utils.PrintKV("API endpoint", fmt.Sprintf("http://%s", backendAddr))
		utils.PrintKV("WebDAV endpoint", fmt.Sprintf("http://%s/%s", backendAddr, cfg.Webdav.Root))
		if len(cfg.Proxy.Sites) > 0 {
			utils.PrintKV("OpenList Sites", fmt.Sprintf("http://%s/openlist/sites", backendAddr))
		}
		utils.PrintSection("How to Play")
		utils.PrintInfo("Press Ctrl+C to stop the server")

		select {}
	},
}
