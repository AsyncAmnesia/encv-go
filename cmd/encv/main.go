package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
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
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/pkg/encv"
)

// Version 会在编译时通过 -ldflags 注入，如果不是，则默认为 "dev"
// 注入方式：go build -ldflags="-X main.Version=v1.2.3" ./cmd/encv
var Version = "dev"

func main() {
	// 【新增】在程序最开始就设置日志
	logFilePath := utils.SetupLogging("encv.log")

	// %TEMP%/encv.log
	log.Printf("Received Args: %v\n", os.Args)
	// 也打印到控制台，方便在 PowerShell 中查看
	fmt.Printf("-> Log file is at: %s\n", logFilePath)

	if len(os.Args) < 2 {
		printUsage()
		return
	}
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
	// 1. 加载基础配置（默认值 + 配置文件），后期修改为可自定义配置文件路径
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load base config: %v", err)
	}
	rootCtx := config.NewContext(context.Background(), cfg)

	switch os.Args[1] {
	case "analyze-v2":
		analyzeV2Cmd := flag.NewFlagSet("analyze-v2", flag.ExitOnError)
		err := analyzeV2Cmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("Error parsing 'analyze-v2' flags: %v", err)
		}

		containerPath := analyzeV2Cmd.Arg(0)
		if containerPath == "" {
			log.Fatal("Error: Please provide the path to the v2 ENCV container file.")
		}

		encv.Init(rootCtx)

		// 调用分析函数
		if err := encv.AnalyzeContainerV2(rootCtx, containerPath); err != nil {
			log.Fatalf("Analysis failed for '%s': %v", containerPath, err)
		}
	case "manifest-v2":
		manifestV2Cmd := flag.NewFlagSet("manifest-v2", flag.ExitOnError)
		savePathPtr := manifestV2Cmd.String("s", "", "Save Manifest content to a specified JSON file.")
		err := manifestV2Cmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("Error parsing 'manifest-v2' flags: %v", err)
		}

		containerPath := manifestV2Cmd.Arg(0)
		if containerPath == "" {
			log.Fatal("Error: Please provide the path to the v2 ENCV container file.")
		}

		encv.Init(rootCtx)

		// 调用新函数获取 Manifest 数据
		manifestData, err := encv.ExtractManifest_v2(containerPath)
		if err != nil {
			log.Fatalf("Failed to extract Manifest from '%s': %v", containerPath, err)
		}

		// 根据 -s 标志决定输出方式
		if *savePathPtr == "" {
			fmt.Println("--- Manifest Content (v2) ---")
			var prettyJSON interface{}
			if err := json.Unmarshal(manifestData, &prettyJSON); err != nil {
				fmt.Printf("%s\n", string(manifestData))
			} else {
				indentedJSON, _ := json.MarshalIndent(prettyJSON, "", "  ")
				fmt.Printf("%s\n", string(indentedJSON))
			}
		} else {
			if err := os.WriteFile(*savePathPtr, manifestData, 0644); err != nil {
				log.Fatalf("Failed to save Manifest to '%s': %v", *savePathPtr, err)
			}
			log.Printf("✅ Manifest content successfully saved to: %s\n", *savePathPtr)
		}
	case "kvi-v2":
		kviV2Cmd := flag.NewFlagSet("kvi-v2", flag.ExitOnError)
		savePathPtr := kviV2Cmd.String("s", "", "Save KVI content to a specified JSON file.")
		err := kviV2Cmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("Error parsing 'kvi-v2' flags: %v", err)
		}

		containerPath := kviV2Cmd.Arg(0)
		if containerPath == "" {
			log.Fatal("Error: Please provide the path to the v2 ENCV container file.")
		}
		encv.Init(rootCtx)

		// 调用新的 v2 函数获取 KVI 数据
		kviData, err := encv.ExtractKVI_v2(containerPath)
		if err != nil {
			log.Fatalf("Failed to extract KVI from '%s': %v", containerPath, err)
		}

		// 根据 -s 标志决定输出方式
		if *savePathPtr == "" {
			// 打印到控制台，并进行格式化
			fmt.Println("--- KVI Content (v2) ---")
			var prettyJSON interface{}
			if err := json.Unmarshal(kviData, &prettyJSON); err != nil {
				// 如果无法解析为 JSON，就打印原始字符串
				fmt.Printf("%s\n", string(kviData))
			} else {
				// 格式化打印
				indentedJSON, _ := json.MarshalIndent(prettyJSON, "", "  ")
				fmt.Printf("%s\n", string(indentedJSON))
			}
		} else {
			// 保存到文件
			if err := os.WriteFile(*savePathPtr, kviData, 0644); err != nil {
				log.Fatalf("Failed to save KVI to '%s': %v", *savePathPtr, err)
			}
			log.Printf("✅ KVI content successfully saved to: %s\n", *savePathPtr)
		}
	case "decrypt-v2":
		decryptV2Cmd := flag.NewFlagSet("decrypt-v2", flag.ExitOnError)
		var passwordFlag, outputDirFlag string
		decryptV2Cmd.StringVar(&passwordFlag, "p", cfg.Password, "Password for decryption, overrides config file")
		decryptV2Cmd.StringVar(&outputDirFlag, "o", "", "Output directory for decrypted files.")

		if err := decryptV2Cmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("Error parsing 'decrypt-v2' flags: %v", err)
		}

		inputPath := decryptV2Cmd.Arg(0)
		if inputPath == "" {
			log.Fatal("Error: Please provide the path to the ENCV container file or directory to decrypt.")
		}

		if passwordFlag != "" {
			cfg.Password = passwordFlag
		}
		// ... (密码提示逻辑) ...

		finalOutputDir := outputDirFlag
		if finalOutputDir == "" {
			finalOutputDir = "./_decrypted_v2" // 默认输出目录
		}

		if err := os.MkdirAll(finalOutputDir, 0755); err != nil {
			log.Fatalf("Failed to create output directory: %v", err)
		}
		// 【关键改动】调用新的、支持文件夹的高级 API
		if err := encv.DecryptPathV2(rootCtx, inputPath, finalOutputDir); err != nil {
			log.Fatalf("Decryption process failed: %v", err)
		}
		log.Printf("✅ All decryption tasks complete. Output in: %s\n", finalOutputDir)

	case "encrypt-v2":
		encryptV2Cmd := flag.NewFlagSet("encrypt-v2", flag.ExitOnError)
		encryptV2Cmd.StringVar(&cfg.Password, "p", cfg.Password, "Password for encryption, overrides config file")
		encryptV2Cmd.StringVar(&cfg.OutputPath, "o", cfg.OutputPath, "Output directory for encrypted files.")
		// 解析子命令后的参数
		if err := encryptV2Cmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("Error parsing 'encrypt-v2' flags: %v", err)
		}
		inputPath := encryptV2Cmd.Arg(0)
		if inputPath == "" {
			log.Fatal("Error: Please provide the path to the file or directory to encrypt.")
		}
		rootCtx := config.NewContext(context.Background(), cfg)
		encv.InitV2(rootCtx) // 此时 Init 得到的是包含命令行参数的最终配置
		if err := os.MkdirAll(cfg.OutputPath, 0755); err != nil {
			log.Fatalf("Failed to create output directory: %v", err)
		}
		if err := encv.EncryptPathV2(rootCtx, inputPath, cfg.OutputPath); err != nil {
			log.Fatalf("Encryption process failed: %v", err)
		}
	case "play-v2":
		playV2Cmd := flag.NewFlagSet("play-v2", flag.ExitOnError)
		playV2Cmd.StringVar(&cfg.Password, "p", cfg.Password, "Password for decryption, overrides config file")

		// 添加一个新参数来指定播放器
		defaultPlayer := "mpv"
		if runtime.GOOS == "windows" {
			// Windows 下可能需要指定完整路径或使用其他播放器
			// 这里我们先假设 mpv.exe 在 PATH 中
			defaultPlayer = "mpv.exe"
		}
		playerPtr := playV2Cmd.String("player", defaultPlayer, "Media player to use (e.g., mpv, vlc)")

		err := playV2Cmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("Error parsing 'play-v2' flags: %v", err)
		}

		inputPath := playV2Cmd.Arg(0)
		if inputPath == "" {
			log.Fatal("Error: Please provide the path to the ENCV container file to play.")
		}

		// 【新增】如果密码为空，则提示用户输入 (复用 decrypt 的逻辑)
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

		finalCtx := config.NewContext(context.Background(), cfg)
		encv.Init(finalCtx)

		log.Printf("-> Starting playback for '%s' with player '%s'\n", inputPath, *playerPtr)
		if err := encv.PlayV2(finalCtx, inputPath, *playerPtr); err != nil {
			log.Fatalf("Playback failed: %v", err)
		}
	// case "encrypt":
	// 	encryptCmd := flag.NewFlagSet("encrypt", flag.ExitOnError)
	// 	// 直接传入 cfg 字段的地址，flag.Parse() 后会自动更新
	// 	encryptCmd.StringVar(&cfg.Password, "p", cfg.Password, "Password for encryption, overrides config file")
	// 	encryptCmd.StringVar(&cfg.OutputPath, "o", cfg.OutputPath, "Output directory, overrides config file")

	// 	err := encryptCmd.Parse(os.Args[2:])
	// 	if err != nil {
	// 		log.Fatalf("Error parsing 'encrypt' flags: %v", err)
	// 	}

	// 	inputPath := encryptCmd.Arg(0)
	// 	if inputPath == "" {
	// 		log.Fatal("Error: Please provide the path to the file or directory to encrypt.")
	// 	}

	// 	finalCtx := config.NewContext(context.Background(), cfg)
	// 	encv.Init(finalCtx)
	// 	encrypter := encv.NewEncrypter()
	// 	if err := encrypter.Encrypt(finalCtx, inputPath); err != nil {
	// 		log.Fatalf("Encryption failed: %v", err)
	// 	}
	// 	log.Printf("✅ Encryption complete. Output in: %s\n", cfg.OutputPath)

	case "decrypt":
		decryptCmd := flag.NewFlagSet("decrypt", flag.ExitOnError)
		// 直接传入 cfg 字段的地址，flag.Parse() 后会自动更新
		decryptCmd.StringVar(&cfg.Password, "p", cfg.Password, "Password for decryption, overrides config file")
		decryptCmd.BoolVar(&cfg.Recover, "r", cfg.Recover, "Force overwrite existing output files, overrides config file")
		// outputPtr := decryptCmd.String("o", "./decrypted", "Output directory for decrypted files")
		// modePtr := decryptCmd.String("mode", "to-folder", "Decryption mode: preview, to-folder, here, to-subfolder")
		// --- 【关键修改】从这里开始 ---
		// 1. 定义一个相对于可执行文件目录的默认输出路径
		defaultOutputDir := filepath.Join(exeDir, "decrypted")
		// 2. 将这个绝对路径作为 flag 的默认值
		outputPtr := decryptCmd.String("o", defaultOutputDir, "Output directory for decrypted files")
		// --- 【关键修改】结束 ---

		modePtr := decryptCmd.String("mode", "to-folder", "Decryption mode: preview, to-folder, here, to-subfolder")

		err := decryptCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("Error parsing flags: %v", err)
		}

		// 【新增】如果密码为空，则提示用户输入
		if cfg.Password == "" {
			fmt.Print("-> Please enter the password for decryption: ")
			// 使用 bufio.Scanner 来安全地读取一行输入（可以包含空格）
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

		inputPath := decryptCmd.Arg(0)
		if inputPath == "" {
			// 【修正】更新提示信息
			log.Fatal("Error: Please provide the path to the ENCV container file to decrypt.")
		}
		// --- 【关键修改】添加详细调试日志 ---
		parsedModeString := *modePtr
		log.Printf("-> [Debug] Parsed mode string from flag: '%s'", parsedModeString)

		var finalOutputDir string
		// 为了排除类型问题，我们直接比较字符串
		switch parsedModeString {
		case "to-subfolder":
			log.Println("-> [Debug] Entering 'to-subfolder' logic.")
			inputDir := filepath.Dir(inputPath)
			fileNameWithoutExt := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(filepath.Base(inputPath)))
			finalOutputDir = filepath.Join(inputDir, fileNameWithoutExt)
		case "here":
			log.Println("-> [Debug] Entering 'here' logic.")
			finalOutputDir = filepath.Dir(inputPath)
		case "to-folder":
			log.Println("-> [Debug] Entering 'to-folder' logic.")
			finalOutputDir = *outputPtr
		default: // preview 模式或其他未知模式
			log.Printf("-> [Debug] Entering 'default' logic for mode: '%s'", parsedModeString)
			finalOutputDir = *outputPtr
		}
		// --- 修改结束 ---

		log.Printf("-> [Debug] Final output directory calculated as: %s\n", finalOutputDir)

		finalCtx := config.NewContext(context.Background(), cfg)
		encv.Init(finalCtx)

		// 注意：这里我们还是需要把字符串转换回 service.DecryptMode 类型，因为解密服务需要它
		// mode := service.DecryptMode(parsedModeString)
		// opts := service.DecryptOptions{
		// 	Mode:      mode,
		// 	OutputDir: finalOutputDir,
		// }
		// decrypter := encv.NewDecrypter()                     // 暂时注释，需要修改，勿删
		// fmt.Printf("--- DEBUG ---\n")
		// fmt.Printf("Parsed Mode: '%s'\n", opts.Mode)
		// fmt.Printf("Input Path: '%s'\n", inputPath)
		// fmt.Printf("-------------\n")

		// if opts.Mode == service.ModePreview {
		// 	// 【新增】调试信息：确认进入了 Preview 分支
		// 	fmt.Println("--- DEBUG: Entering PREVIEW mode ---")
		// 	if err := decrypter.Preview(finalCtx, inputPath); err != nil {
		// 		log.Fatalf("Preview failed: %v", err)
		// 	}
		// 	fmt.Println("--- DEBUG: Preview function finished successfully ---")
		// } else {
		// 	// 【新增】调试信息：确认进入了 Decrypt 分支
		// 	fmt.Println("--- DEBUG: Entering STANDARD DECRYPT mode ---")
		// 	if err := decrypter.Decrypt(finalCtx, inputPath, opts); err != nil {
		// 		log.Fatalf("Decryption failed: %v", err)
		// 	}
		// 	log.Printf("✅ Decryption complete. Output in: %s\n", opts.OutputDir)
		// }

	case "start":
		serverCmd := flag.NewFlagSet("start", flag.ExitOnError)
		encv.ParseServerFlags(serverCmd, cfg, os.Args[2:])
		finalCtx := config.NewContext(context.Background(), cfg)
		encv.Init(finalCtx)
		s := encv.NewServer(finalCtx)
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

	case "webdav":
		// 暂时注释，需要修改，勿删
		//
		//
		// webdavCmd := flag.NewFlagSet("webdav", flag.ExitOnError)
		// // 单实例检查，如需多实例请使用 "start"
		// if err := encv.CheckForExistingService(cfg.Webdav.Port); err != nil {
		// 	os.Exit(1)
		// }
		// // 直接传入 cfg 字段的地址，flag.Parse() 后会自动更新
		// webdavCmd.StringVar(&cfg.Password, "p", cfg.Password, "Password for server to decrypt, overrides config file")
		// webdavCmd.StringVar(&cfg.Webdav.Dir, "d", cfg.Webdav.Dir, "Directory to serve, overrides config file")
		// webdavCmd.IntVar(&cfg.Webdav.Port, "port", cfg.Webdav.Port, "Port for WebDAV server, overrides config file")
		// webdavCmd.Parse(os.Args[2:])

		// if cfg.Password == "" {
		// 	log.Fatalf("WebDAV requires a password. Please set it in config.user.json.")
		// }
		// finalCtx := config.NewContext(context.Background(), cfg)
		// encv.Init(finalCtx)
		// addr, webdavPath, err := encv.StartWebdav(finalCtx)
		// if err != nil {
		// 	log.Fatalf("Failed to start WebDAV server: %v", err)
		// }

		// log.Printf("\n✅ WebDAV server started successfully!\n")
		// log.Printf("   Serving files from: %s\n", cfg.Webdav.Dir)
		// log.Printf("   Access it at: http://%s%s\n", addr, webdavPath)
		// log.Println("\n--- How to Connect ---")
		// log.Printf("   Windows: \\\\localhost@%s%s\n", strings.TrimPrefix(addr, ":"), webdavPath)
		// log.Printf("   macOS:   http://%s%s\n", addr, webdavPath)
		// log.Println("\n(Press Ctrl+C in this terminal to stop the server)")

		// select {} // Keep server running

	case "server":
		serverCmd := flag.NewFlagSet("server", flag.ExitOnError)
		// 单实例检查，如需多实例请使用 "start"
		if err := encv.CheckForExistingService(cfg.Server.Port); err != nil {
			os.Exit(1)
		}
		encv.ParseServerFlags(serverCmd, cfg, os.Args[2:])
		ctx := config.NewContext(context.Background(), cfg)
		encv.Init(ctx)
		s := encv.NewServer(ctx)
		_, err = s.Start(cfg.Server.Port, Version)
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
		log.Println("✅ ENCV Server is running.")
		log.Printf("-> You can now double-click files to open them.")
		log.Printf("-> To stop the server, run: encv.exe stop-server")
		log.Println("-> Press Ctrl+C to stop the server manually.")

		// 等待中断信号以优雅地关闭服务器
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit
		log.Println("-> Shutting down server...")
		s.Stop()

	case "openas":
		// 这个命令只在 Windows 上有意义
		if runtime.GOOS != "windows" {
			log.Fatal("Error: The 'openas' command is only available on Windows.")
		}

		// 调用我们之前定义的、专门处理“打开方式”的函数
		if err := handleOpenAsCommand(cfg); err != nil {
			log.Fatalf("Failed to register file associations: %v", err)
		}

		log.Println("✅ File associations for 'Open' action registered successfully!")
		log.Println("You can now double-click on an ENCV file to decrypt it.")

	// case "unopenas":
	// 	if runtime.GOOS != "windows" {
	// 		log.Fatal("Error: The 'unopenas' command is only available on Windows.")
	// 	}
	// 	if err := handleUnopenAsCommand(); err != nil {
	// 		log.Fatalf("Failed to unregister file associations: %v", err)
	// 	}
	// 	log.Println("✅ File associations for 'Open' action unregistered successfully!")

	case "open-stream":

		// 1. 【关键修改】从配置中确定发现起始端口
		discoveryStartPort := cfg.Server.Port
		if discoveryStartPort == 0 {
			// 如果配置文件中端口为0（表示任意端口），我们使用一个合理的默认值作为扫描起点
			discoveryStartPort = 1999
			log.Printf("INFO: Server port in config is 0 (any port). Starting discovery from default port %d.", discoveryStartPort)
		} else {
			log.Printf("INFO: Starting discovery from configured port %d.", discoveryStartPort)
		}

		const maxDiscoveryTries = 20 // 扫描 20 个端口
		serverAddr, err := encv.FindServer(discoveryStartPort, maxDiscoveryTries)
		if err != nil {
			log.Println("--------------------------------------------------")
			log.Println("🔴 ENCV Server is not running.")
			log.Printf("-> Please start it first by running: encv.exe start-server")
			log.Printf("-> Or check if it's running near the configured port: %d", discoveryStartPort)
			log.Println("--------------------------------------------------")
			os.Exit(1)
		}

		// 3. 获取输入文件
		inputPath := os.Args[2]
		if inputPath == "" {
			log.Fatal("Error: No input file specified for open-stream.")
		}
		// 4. 【新增】准备字幕文件
		subtitles, err := prepareSubtitles(inputPath, cfg)
		if err != nil {
			log.Printf("Warning: An error occurred while preparing subtitles: %v. Playing without subtitles.", err)
		}
		// 4. 使用动态发现的 serverAddr 构造流 URL
		encodedPath := url.QueryEscape(inputPath)
		streamURL := fmt.Sprintf("http://%s/stream?file=%s", serverAddr, encodedPath)

		// 构建 mpv 参数
		mpvArgs := []string{streamURL}
		for _, sub := range subtitles {
			// 【修复】使用 = 连接选项和参数，符合 mpv 的要求
			mpvArgs = append(mpvArgs, fmt.Sprintf("--sub-files=%s", sub.Path))
		}

		log.Printf("-> Starting mpv with arguments: %v", mpvArgs)
		// cmd := exec.Command("mpv", mpvArgs...)

		// 【修改】让 mpv 写入日志文件
		logFile := filepath.Join(os.TempDir(), "encv_mpv.log")
		cmd := exec.Command("mpv", append(mpvArgs, "--log-file="+logFile, "--msg-level=all=v")...)

		// 保留输出捕获，以防有其他问题
		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr

		err = cmd.Run()
		if err != nil {
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

	// case "open-temp":

	// 	// 2. 获取输入文件
	// 	inputPath := os.Args[2]
	// 	if inputPath == "" {
	// 		log.Fatal("Error: No input file specified for open-temp.")
	// 	}

	// 	// 3. 【关键】从容器中获取原始文件名和索引信息
	// 	index, packedData, err := getContainerInfo(inputPath, cfg)
	// 	if err != nil {
	// 		log.Fatalf("Failed to get container info: %v", err)
	// 	}
	// 	defer packedData.DataStream.Close()

	// 	originalFilename := index.GetOriginalFilename()
	// 	log.Printf("-> Original filename in container: %s", originalFilename)

	// 	// 4. 创建临时文件
	// 	tmpFile, err := os.CreateTemp("", "*"+filepath.Ext(originalFilename))
	// 	if err != nil {
	// 		log.Fatalf("Failed to create temp file: %v", err)
	// 	}
	// 	tmpPath := tmpFile.Name()
	// 	defer os.Remove(tmpPath) // 确保程序退出时删除
	// 	log.Printf("-> Decrypting to temporary file: %s", tmpPath)

	// 	// 5. 解密并写入临时文件
	// 	if err := decryptToFile(packedData, index, cfg, tmpFile); err != nil {
	// 		log.Fatalf("Failed to decrypt to temp file: %v", err)
	// 	}
	// 	tmpFile.Close() // 关闭文件，让其他程序可以访问

	// 	// 6. 使用默认程序打开
	// 	log.Printf("-> Opening with default application...")
	// 	var cmd *exec.Cmd
	// 	switch runtime.GOOS {
	// 	case "windows":
	// 		cmd = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", tmpPath)
	// 	case "darwin":
	// 		cmd = exec.Command("open", tmpPath)
	// 	default:
	// 		cmd = exec.Command("xdg-open", tmpPath)
	// 	}

	// 	if err := cmd.Start(); err != nil {
	// 		log.Fatalf("Failed to open file with default app: %v", err)
	// 	}
	// 	log.Printf("-> Temporary file opened. It will be deleted when this program exits.")

	// ... (其他 case) ...

	case "register":
		// 这个命令只在 Windows 上有意义
		if runtime.GOOS != "windows" {
			log.Fatal("Error: The 'register' command is only available on Windows.")
		}
		if err := RegisterFileAssociations(); err != nil {
			log.Fatalf("Failed to register file associations: %v", err)
		}

	case "unregister":
		// 这个命令只在 Windows 上有意义
		if runtime.GOOS != "windows" {
			log.Fatal("Error: The 'unregister' command is only available on Windows.")
		}
		if err := UnregisterFileAssociations(); err != nil {
			log.Fatalf("Failed to unregister file associations: %v", err)
		}

	case "debug":
		if len(os.Args) < 2 {
			fmt.Println("Usage: encv.exe debug <path_to_sub_chunk_file>")
			os.Exit(1)
		}
		subChunkPath := os.Args[2]
		// 我们需要子分片的魔法数字
		subMagic := container.MagicVideoSubChunk
		err := debugSubChunkHeader(subChunkPath, subMagic)
		if err != nil {
			log.Fatalf("Failed to debug sub-chunk header: %v", err)
		}

	default:
		printUsage()
	}
}

func printUsage() {
	log.Println(`
Usage: ./encv <command> [flags] [path]

IMPORTANT: Flags (like -o, -p) must be specified BEFORE the path argument.

Commands:
  encrypt <input_path>  Encrypt a video file or all videos in a directory.
  decrypt <input_path>  Decrypt a single ENCV container file and its associated tracks.
  kvi     <container_path> Extract and print or save the KVI from a container file.
  server  <directory>   Start a server to stream encrypted videos from a directory.
  openas                 (Windows only) Register ENCV as the default application for its file types.
	register              (Windows only) Register file associations and right-click menu.
  unregister            (Windows only) Unregister file associations.


Flags:
  -p, --password <pwd>  Password for encryption/decryption.
  -o, --output <path>   Output directory.
  -s <file.json>        (for 'kvi' command) Save KVI to a file.
  --port <number>       Port to run the server on (for 'serve' command).
  --mode <mode>         (for 'decrypt' command) Decryption mode: preview, to-folder, here, to-subfolder.


Examples:
  ./encv encrypt -o ./my_encrypted_videos ./my_videos
  ./encv decrypt -p mypassword -o ./my_decrypted_movie/ ./output/movie.4pm.sccgv
  ./encv kvi ./output/movie.4pm.sccgv
  ./encv kvi -s kvi.json ./output/movie.4pm.sccgv
  ./encv serve -p mypassword -o ./my_videos --port 8080
  ./encv openas
`)
	fmt.Println("  play-v2 <path>    Stream and play a video container using the new v2 architecture.")
	fmt.Println("                    -p <password>     Password for decryption.")
	fmt.Println("                    -player <path>    Media player to use (default: mpv).")
	fmt.Println()
	fmt.Println("  encrypt-v2 <path>  Encrypt a file or a directory (non-recursive) using the new v2 architecture.")
	fmt.Println("                     -p <password>     Password for encryption.")
	fmt.Println("                     -o <output_path>  Output container file path.")
	fmt.Println()
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
