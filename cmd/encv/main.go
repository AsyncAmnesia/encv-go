package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/pkg/encv"
	"github.com/spf13/cobra"
)

// Version 会在编译时通过 -ldflags 注入，如果不是，则默认为 "dev"
// 注入方式：go build -ldflags="-X main.Version=v1.2.3" ./cmd/encv
var Version = "dev"

// 全局变量，由 PersistentPreRun 初始化，供所有子命令使用
var (
	cfg     *config.Config
	rootCtx context.Context
)

// --- init 函数：添加所有命令到根命令，并定义标志 ---
func init() {
	// 添加子命令
	rootCmd.AddCommand(analyzeV2Cmd)
	rootCmd.AddCommand(manifestV2Cmd)
	rootCmd.AddCommand(kviV2Cmd)
	rootCmd.AddCommand(decryptV2Cmd)
	rootCmd.AddCommand(encryptV2Cmd)
	rootCmd.AddCommand(registerProtocolCmd)
	rootCmd.AddCommand(unregisterProtocolCmd)
	rootCmd.AddCommand(playV2Cmd)
	rootCmd.AddCommand(startCmd)
	// rootCmd.AddCommand(webdavCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(openasCmd)
	rootCmd.AddCommand(openStreamCmd)
	rootCmd.AddCommand(registerCmd)
	rootCmd.AddCommand(unregisterCmd)

	// 为命令添加标志
	manifestV2Cmd.Flags().StringP("save", "s", "", "Save Manifest content to a specified JSON file.")
	kviV2Cmd.Flags().StringP("save", "s", "", "Save KVI content to a specified JSON file.")
	decryptV2Cmd.Flags().StringP("password", "p", "", "Password for decryption (overrides config)")
	decryptV2Cmd.Flags().StringP("output", "o", "", "Output directory for decrypted files")
	encryptV2Cmd.Flags().StringP("password", "p", "", "Password for encryption (overrides config)")
	encryptV2Cmd.Flags().StringP("output", "o", "", "Output directory for encrypted files (overrides config)")
	// play-v2 的标志，包含 OS 相关的默认值
	defaultPlayer := "mpv"
	if runtime.GOOS == "windows" {
		defaultPlayer = "mpv.exe"
	}
	playV2Cmd.Flags().StringP("player", "r", defaultPlayer, "Media player to use (e.g., mpv, vlc)")

	// webdav 的标志
	// webdavCmd.Flags().StringP("password", "p", "", "Password for server to decrypt (overrides config)")
	// webdavCmd.Flags().StringP("dir", "d", "", "Directory to serve (overrides config)")
	// webdavCmd.Flags().IntP("port", "P", 0, "Port for WebDAV server (overrides config)")
}

// --- main 函数：入口点，变得非常简洁 ---
func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Command execution failed: %v", err)
	}
}

// --- 根命令 ---
var rootCmd = &cobra.Command{
	Use:   "encv",
	Short: "ENCV is a tool for encrypting and decrypting files.",
	Long:  `ENCV is a powerful command-line tool for encrypting and decrypting files and directories using a custom container format.`,
	// PersistentPreRun 会在每个子命令运行前执行，非常适合用来做通用的初始化工作
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// 【新增】在程序最开始就设置日志
		logFilePath := utils.SetupLogging("encv.log")
		log.Printf("Received Args: %v\n", os.Args)
		fmt.Printf("-> Log file is at: %s\n", logFilePath)

		// 1. 获取可执行文件自身的路径
		exePath, err := os.Executable()
		if err != nil {
			log.Fatalf("Error: Could not determine executable path: %v", err)
		}
		// 2. 获取可执行文件所在的目录
		exeDir := filepath.Dir(exePath)
		// 3. 构建配置文件的完整路径
		configPath := filepath.Join(exeDir, "config.user.json")

		log.Printf("-> Loading config from: %s\n", configPath)
		// 加载基础配置（默认值 + 配置文件）
		cfg, err = config.Load(configPath)
		if err != nil {
			log.Fatalf("Failed to load base config: %v", err)
		}
		rootCtx = config.NewContext(context.Background(), cfg)
	},
}

// --- analyze-v2 命令 ---
var analyzeV2Cmd = &cobra.Command{
	Use:   "analyze-v2 [path to container]",
	Short: "Analyzes a v2 ENCV container file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		encv.Init(rootCtx)
		containerPath := args[0]
		if err := encv.AnalyzeContainerV2(rootCtx, containerPath); err != nil {
			log.Fatalf("Analysis failed for '%s': %v", containerPath, err)
		}
	},
}

// --- manifest-v2 命令 ---
var manifestV2Cmd = &cobra.Command{
	Use:   "manifest-v2 [path to container]",
	Short: "Extracts and prints the manifest from a v2 container",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		encv.Init(rootCtx)
		containerPath := args[0]
		savePath, _ := cmd.Flags().GetString("save")

		manifestData, err := encv.ExtractManifest_v2(containerPath)
		if err != nil {
			log.Fatalf("Failed to extract Manifest from '%s': %v", containerPath, err)
		}

		if savePath == "" {
			fmt.Println("--- Manifest Content (v2) ---")
			var prettyJSON interface{}
			if err := json.Unmarshal(manifestData, &prettyJSON); err != nil {
				fmt.Printf("%s\n", string(manifestData))
			} else {
				indentedJSON, _ := json.MarshalIndent(prettyJSON, "", "  ")
				fmt.Printf("%s\n", string(indentedJSON))
			}
		} else {
			if err := os.WriteFile(savePath, manifestData, 0644); err != nil {
				log.Fatalf("Failed to save Manifest to '%s': %v", savePath, err)
			}
			log.Printf("✅ Manifest content successfully saved to: %s\n", savePath)
		}
	},
}

// --- kvi-v2 命令 ---
var kviV2Cmd = &cobra.Command{
	Use:   "kvi-v2 [path to container]",
	Short: "Extracts and prints the KVI from a v2 container",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		encv.Init(rootCtx)
		containerPath := args[0]
		savePath, _ := cmd.Flags().GetString("save")

		kviData, err := encv.ExtractKVI_v2(containerPath)
		if err != nil {
			log.Fatalf("Failed to extract KVI from '%s': %v", containerPath, err)
		}

		if savePath == "" {
			fmt.Println("--- KVI Content (v2) ---")
			var prettyJSON interface{}
			if err := json.Unmarshal(kviData, &prettyJSON); err != nil {
				fmt.Printf("%s\n", string(kviData))
			} else {
				indentedJSON, _ := json.MarshalIndent(prettyJSON, "", "  ")
				fmt.Printf("%s\n", string(indentedJSON))
			}
		} else {
			if err := os.WriteFile(savePath, kviData, 0644); err != nil {
				log.Fatalf("Failed to save KVI to '%s': %v", savePath, err)
			}
			log.Printf("✅ KVI content successfully saved to: %s\n", savePath)
		}
	},
}

// --- decrypt-v2 命令 ---
var decryptV2Cmd = &cobra.Command{
	Use:   "decrypt-v2 [path to file/dir]",
	Short: "Decrypts a v2 ENCV container or a directory of them",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]

		// 【关键修正】从标志获取值，并直接覆盖 cfg 中的值
		passwordFlag, _ := cmd.Flags().GetString("password")
		if passwordFlag != "" {
			cfg.Password = passwordFlag
		}

		outputDirFlag, _ := cmd.Flags().GetString("output")
		finalOutputDir := outputDirFlag
		if finalOutputDir == "" {
			finalOutputDir = "./_decrypted_v2"
		}

		// 如果此时 cfg.Password 仍然为空，则提示用户输入
		if cfg.Password == "" {
			fmt.Print("Enter password: ")
			// 注意：这里需要您自行处理密码输入，例如使用 term.ReadPassword
			// bytePassword, _ := term.ReadPassword(int(syscall.Stdin))
			// cfg.Password = string(bytePassword)
			// fmt.Println()
			// 为了简化示例，这里暂时省略
			log.Fatal("Password is required.")
		}

		if err := os.MkdirAll(finalOutputDir, 0755); err != nil {
			log.Fatalf("Failed to create output directory: %v", err)
		}

		// 【关键修正】调用函数时不再传递密码，让它从 cfg 中读取
		if err := encv.DecryptPathV2(rootCtx, inputPath, finalOutputDir); err != nil {
			log.Fatalf("Decryption process failed: %v", err)
		}
		log.Printf("✅ All decryption tasks complete. Output in: %s\n", finalOutputDir)
	},
}

// --- encrypt-v2 命令 ---
var encryptV2Cmd = &cobra.Command{
	Use:   "encrypt-v2 [path to file/dir]",
	Short: "Encrypts a file or directory into a v2 ENCV container",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		// 这些标志会覆盖配置文件中的值
		passwordFlag, _ := cmd.Flags().GetString("password")
		outputPathFlag, _ := cmd.Flags().GetString("output")

		if passwordFlag != "" {
			cfg.Password = passwordFlag
		}
		if outputPathFlag != "" {
			cfg.OutputPath = outputPathFlag
		}

		encv.InitV2(rootCtx)
		if err := os.MkdirAll(cfg.OutputPath, 0755); err != nil {
			log.Fatalf("Failed to create output directory: %v", err)
		}
		if err := encv.EncryptPathV2(rootCtx, inputPath, cfg.OutputPath); err != nil {
			log.Fatalf("Encryption process failed: %v", err)
		}
	},
}

// --- 协议相关命令 ---
var registerProtocolCmd = &cobra.Command{
	Use:   "register-protocol",
	Short: "在Windows中注册 encv:// 自定义协议",
	Run: func(cmd *cobra.Command, args []string) {
		if err := RegisterProtocol(cfg); err != nil {
			log.Fatalf("注册协议失败: %v", err)
		}
	},
}

var unregisterProtocolCmd = &cobra.Command{
	Use:   "unregister-protocol",
	Short: "在Windows中取消注册 encv:// 自定义协议",
	Run: func(cmd *cobra.Command, args []string) {
		if err := UnregisterProtocol(); err != nil {
			log.Fatalf("取消注册协议失败: %v", err)
		}
	},
}

// --- play-v2 命令 ---
var playV2Cmd = &cobra.Command{
	Use:   "play-v2 [path to container]",
	Short: "Decrypts and plays a media file from a v2 container",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		player, _ := cmd.Flags().GetString("player")

		encv.Init(rootCtx)

		if cfg.Password == "" {
			fmt.Print("-> Please enter the password for decryption: ")
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				cfg.Password = scanner.Text()
			}
			if err := scanner.Err(); err != nil {
				log.Fatalf("Failed to read password from input: %v", err)
			}
			if cfg.Password == "" {
				log.Fatal("Error: Password cannot be empty.")
			}
		}

		log.Printf("-> Starting playback for '%s' with player '%s'\n", inputPath, player)
		if err := encv.PlayV2(rootCtx, inputPath, player); err != nil {
			log.Fatalf("Playback failed: %v", err)
		}
	},
}

// --- start 命令 ---
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the ENCV server and keeps it running in the foreground",
	Run: func(cmd *cobra.Command, args []string) {
		encv.Init(rootCtx)
		s := encv.NewServer(rootCtx)
		addr, err := s.Start(cfg.Server.Port, Version)
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}

		log.Printf("\n✅ Server started successfully!\n")
		log.Printf("   Serving files from: %s\n", cfg.Server.Dir)
		log.Printf("   Access it at: http://localhost%s\n", addr)
		log.Println("\n--- How to Play ---")
		log.Printf("   mpv --no-config http://localhost%s/<video_name_without_extension>\n", addr)
		log.Println("\n(Press Ctrl+C in this terminal to stop the server)")

		select {} // Keep server running
	},
}

// --- webdav 命令 ---
// var webdavCmd = &cobra.Command{
// 	Use:   "webdav",
// 	Short: "Starts the ENCV WebDAV server",
// 	Run: func(cmd *cobra.Command, args []string) {
// 		if err := encv.CheckForExistingService(cfg.Webdav.Port); err != nil {
// 			os.Exit(1)
// 		}

// 		// Flags override config
// 		if pwd, _ := cmd.Flags().GetString("password"); pwd != "" {
// 			cfg.Password = pwd
// 		}
// 		if dir, _ := cmd.Flags().GetString("dir"); dir != "" {
// 			cfg.Webdav.Dir = dir
// 		}
// 		if port, _ := cmd.Flags().GetInt("port"); port != 0 {
// 			cfg.Webdav.Port = port
// 		}

// 		if cfg.Password == "" {
// 			log.Fatalf("WebDAV requires a password. Please set it in config.user.json or with the -p flag.")
// 		}
// 		encv.Init(rootCtx)
// 		addr, webdavPath, err := encv.StartWebdav(rootCtx)
// 		if err != nil {
// 			log.Fatalf("Failed to start WebDAV server: %v", err)
// 		}

// 		log.Printf("\n✅ WebDAV server started successfully!\n")
// 		log.Printf("   Serving files from: %s\n", cfg.Webdav.Dir)
// 		log.Printf("   Access it at: http://%s%s\n" , addr, webdavPath)
// 		log.Println("\n--- How to Connect ---")
// 		log.Printf("   Windows: \\\\localhost@%s%s\n", strings.TrimPrefix(addr, ":"), webdavPath)
// 		log.Printf("   macOS:   http://%s%s\n" , addr, webdavPath)
// 		log.Println("\n(Press Ctrl+C in this terminal to stop the server)")

// 		select {} // Keep server running
// 	},
// }

// --- server 命令 ---
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Starts the ENCV server as a background service",
	Run: func(cmd *cobra.Command, args []string) {
		if err := encv.CheckForExistingService(cfg.Server.Port); err != nil {
			os.Exit(1)
		}
		// Note: The original ParseServerFlags was here. If it needs to be re-implemented,
		// it should be done by adding flags to this command and overriding cfg values.
		encv.Init(rootCtx)
		s := encv.NewServer(rootCtx)
		_, err := s.Start(cfg.Server.Port, Version)
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

// --- openas 命令 ---
var openasCmd = &cobra.Command{
	Use:   "openas",
	Short: "Registers 'Open' action for ENCV files (Windows only)",
	RunE: func(cmd *cobra.Command, args []string) error { // Use RunE to return errors
		if runtime.GOOS != "windows" {
			return fmt.Errorf("the 'openas' command is only available on Windows")
		}
		if err := handleOpenAsCommand(cfg); err != nil {
			return fmt.Errorf("failed to register file associations: %w", err)
		}
		log.Println("✅ File associations for 'Open' action registered successfully!")
		log.Println("You can now double-click on an ENCV file to decrypt it.")
		return nil
	},
}

// --- open-stream 命令 ---
var openStreamCmd = &cobra.Command{
	Use:   "open-stream [path to container]",
	Short: "Streams a media file from a running ENCV server to mpv",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputPath := args[0]
		discoveryStartPort := cfg.Server.Port
		if discoveryStartPort == 0 {
			discoveryStartPort = 1999
			log.Printf("INFO: Server port in config is 0 (any port). Starting discovery from default port %d.", discoveryStartPort)
		} else {
			log.Printf("INFO: Starting discovery from configured port %d.", discoveryStartPort)
		}

		serverAddr, err := encv.FindServer(discoveryStartPort, 20)
		if err != nil {
			log.Println("--------------------------------------------------")
			log.Println("🔴 ENCV Server is not running.")
			log.Printf("-> Please start it first by running: encv.exe start-server")
			log.Printf("-> Or check if it's running near the configured port: %d", discoveryStartPort)
			log.Println("--------------------------------------------------")
			os.Exit(1)
		}

		// Assume prepareSubtitles exists and is defined elsewhere
		subtitles, err := prepareSubtitles(inputPath, cfg)
		if err != nil {
			log.Printf("Warning: An error occurred while preparing subtitles: %v. Playing without subtitles.", err)
		}

		encodedPath := url.QueryEscape(inputPath)
		streamURL := fmt.Sprintf("http://%s/stream?file=%s", serverAddr, encodedPath)

		mpvArgs := []string{streamURL}
		for _, sub := range subtitles {
			mpvArgs = append(mpvArgs, fmt.Sprintf("--sub-files=%s", sub.Path))
		}

		log.Printf("-> Starting mpv with arguments: %v", mpvArgs)
		logFile := filepath.Join(os.TempDir(), "encv_mpv.log")
		cmd2 := exec.Command("mpv", append(mpvArgs, "--log-file="+logFile, "--msg-level=all=v")...)

		var out, stderr bytes.Buffer
		cmd2.Stdout = &out
		cmd2.Stderr = &stderr

		if err := cmd2.Run(); err != nil {
			log.Println("--------------------------------------------------")
			log.Println("🔴 Failed to run mpv.")
			log.Printf("Error: %v", err)
			log.Println("--- MPV Stdout ---")
			log.Println(out.String())
			log.Println("--- MPV Stderr ---")
			log.Println(stderr.String())
			log.Println("--------------------------------------------------")
			log.Fatalf("Please check the MPV output above for details.")
		}
		log.Println("-> mpv closed.")
	},
}

// --- register / unregister 命令 ---
var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Registers file associations and context menu (Windows only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "windows" {
			return fmt.Errorf("the 'register' command is only available on Windows")
		}
		if err := RegisterFileAssociations(); err != nil {
			return fmt.Errorf("failed to register file associations: %w", err)
		}
		return nil
	},
}

var unregisterCmd = &cobra.Command{
	Use:   "unregister",
	Short: "Unregisters file associations and context menu (Windows only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "windows" {
			return fmt.Errorf("the 'unregister' command is only available on Windows")
		}
		if err := UnregisterFileAssociations(); err != nil {
			return fmt.Errorf("failed to unregister file associations: %w", err)
		}
		return nil
	},
}

// prepareSubtitles 查找与视频同名的字幕文件，并解密加密的字幕
func prepareSubtitles(videoPath string, cfg *config.Config) ([]SubtitleInfo, error) {
	var subtitles []SubtitleInfo

	// 1. 获取视频文件的目录和基础名
	videoDir := filepath.Dir(videoPath)
	videoBaseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))

	// 2. 定义要查找的字幕扩展名（包括加密和未加密的）
	subExts := []string{".srt", ".ass", ".vtt", cfg.BinExtGroup.Text}

	// 3. 查找文件
	for _, ext := range subExts {
		potentialSubPath := filepath.Join(videoDir, videoBaseName+ext)

		if _, err := os.Stat(potentialSubPath); err == nil {
			// 是普通字幕，直接使用
			log.Printf("-> Found standard subtitle: %s", potentialSubPath)
			subtitles = append(subtitles, SubtitleInfo{Path: potentialSubPath, IsTemp: false})
		}
	}

	return subtitles, nil
}

// SubtitleInfo 存储字幕文件信息
type SubtitleInfo struct {
	Path   string // 字幕文件的路径（原始或临时）
	IsTemp bool   // 是否是解密后的临时文件
}
