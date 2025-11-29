package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/service"
	"github.com/Soltus/encv-go/pkg/encv"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	// 1. 加载基础配置（默认值 + 配置文件），后期修改为可自定义配置文件路径
	cfg, err := config.Load("config.user.json")
	if err != nil {
		log.Fatalf("Failed to load base config: %v", err)
	}
	rootCtx := config.NewContext(context.Background(), cfg)

	switch os.Args[1] {
	case "encrypt":
		encryptCmd := flag.NewFlagSet("encrypt", flag.ExitOnError)
		// 直接传入 cfg 字段的地址，flag.Parse() 后会自动更新
		encryptCmd.StringVar(&cfg.Password, "p", cfg.Password, "Password for encryption, overrides config file")
		encryptCmd.StringVar(&cfg.OutputPath, "o", cfg.OutputPath, "Output directory, overrides config file")

		err := encryptCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("Error parsing 'encrypt' flags: %v", err)
		}

		inputPath := encryptCmd.Arg(0)
		if inputPath == "" {
			log.Fatal("Error: Please provide the path to the file or directory to encrypt.")
		}

		finalCtx := config.NewContext(context.Background(), cfg)
		encv.Init(finalCtx)
		encrypter := encv.NewEncrypter()
		if err := encrypter.Encrypt(finalCtx, inputPath); err != nil {
			log.Fatalf("Encryption failed: %v", err)
		}
		log.Printf("✅ Encryption complete. Output in: %s\n", cfg.OutputPath)

	case "decrypt":
		decryptCmd := flag.NewFlagSet("decrypt", flag.ExitOnError)
		// 直接传入 cfg 字段的地址，flag.Parse() 后会自动更新
		decryptCmd.StringVar(&cfg.Password, "p", cfg.Password, "Password for decryption, overrides config file")
		decryptCmd.BoolVar(&cfg.Recover, "r", cfg.Recover, "Force overwrite existing output files, overrides config file")
		outputPtr := decryptCmd.String("o", "./decrypted", "Output directory for decrypted files")
		err := decryptCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("Error parsing flags: %v", err)
		}

		inputPath := decryptCmd.Arg(0)
		if inputPath == "" {
			// 【修正】更新提示信息
			log.Fatal("Error: Please provide the path to the ENCV container file to decrypt.")
		}

		finalCtx := config.NewContext(context.Background(), cfg)
		encv.Init(finalCtx)
		opts := service.DecryptOptions{
			OutputDir: *outputPtr,
		}
		decrypter := encv.NewDecrypter()
		if err := decrypter.Decrypt(finalCtx, inputPath, opts); err != nil {
			log.Fatalf("Decryption failed: %v", err)
		}
		log.Printf("✅ Decryption complete. Output in: %s\n", opts.OutputDir)

	case "kvi":
		kviCmd := flag.NewFlagSet("kvi", flag.ExitOnError)
		savePathPtr := kviCmd.String("s", "", "Save KVI content to a specified JSON file.")
		err := kviCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("Error parsing flags: %v", err)
		}

		containerPath := kviCmd.Arg(0)
		if containerPath == "" {
			log.Fatal("Error: Please provide the path to the ENCV container file.")
		}
		finalCtx := config.NewContext(context.Background(), cfg)
		encv.Init(finalCtx)

		// 调用新函数获取 KVI 数据
		kviData, err := encv.ExtractKVI(rootCtx, containerPath)
		if err != nil {
			log.Fatalf("Failed to extract KVI from '%s': %v", containerPath, err)
		}

		// 根据 -s 标志决定输出方式
		if *savePathPtr == "" {
			// 打印到控制台，并进行格式化
			fmt.Println("--- KVI Content ---")
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

	case "server":
		serverCmd := flag.NewFlagSet("server", flag.ExitOnError)
		// 直接传入 cfg 字段的地址，flag.Parse() 后会自动更新
		serverCmd.StringVar(&cfg.Password, "p", cfg.Password, "Password for server to decrypt, overrides config file")
		serverCmd.StringVar(&cfg.Server.Dir, "d", cfg.Server.Dir, "Directory to serve from, overrides config file")
		serverCmd.IntVar(&cfg.Server.Port, "port", cfg.Server.Port, "Port to run the server on, overrides config file")

		err := serverCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("Error parsing flags: %v", err)
		}

		finalCtx := config.NewContext(context.Background(), cfg)
		encv.Init(finalCtx)
		player := encv.NewPlayer(finalCtx)

		addr, err := player.Start(cfg.Server.Port)
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
		webdavCmd := flag.NewFlagSet("webdav", flag.ExitOnError)
		// 直接传入 cfg 字段的地址，flag.Parse() 后会自动更新
		webdavCmd.StringVar(&cfg.Password, "p", cfg.Password, "Password for server to decrypt, overrides config file")
		webdavCmd.StringVar(&cfg.Webdav.Dir, "d", cfg.Webdav.Dir, "Directory to serve, overrides config file")
		webdavCmd.IntVar(&cfg.Webdav.Port, "port", cfg.Webdav.Port, "Port for WebDAV server, overrides config file")
		webdavCmd.Parse(os.Args[2:])

		if cfg.Password == "" {
			log.Fatalf("WebDAV requires a password. Please set it in config.user.json.")
		}
		finalCtx := config.NewContext(context.Background(), cfg)
		encv.Init(finalCtx)
		addr, webdavPath, err := encv.StartWebdav(finalCtx)
		if err != nil {
			log.Fatalf("Failed to start WebDAV server: %v", err)
		}

		log.Printf("\n✅ WebDAV server started successfully!\n")
		log.Printf("   Serving files from: %s\n", cfg.Webdav.Dir)
		log.Printf("   Access it at: http://localhost%s%s\n", addr, webdavPath)
		log.Println("\n--- How to Connect ---")
		log.Printf("   Windows: \\\\localhost@%s%s\n", strings.TrimPrefix(addr, ":"), webdavPath)
		log.Printf("   macOS:   http://localhost%s%s\n", addr, webdavPath)
		log.Println("\n(Press Ctrl+C in this terminal to stop the server)")

		select {} // Keep server running

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
  serve   <directory>   Start a server to stream encrypted videos from a directory.

Flags:
  -p, --password <pwd>  Password for encryption/decryption.
  -o, --output <path>   Output directory.
  -s <file.json>        (for 'kvi' command) Save KVI to a file.
  --port <number>       Port to run the server on (for 'serve' command).

Examples:
  ./encv encrypt -o ./my_encrypted_videos ./my_videos
  ./encv decrypt -p mypassword -o ./my_decrypted_movie/ ./output/movie.4pm.sccgv
  ./encv kvi ./output/movie.4pm.sccgv
  ./encv kvi -s kvi.json ./output/movie.4pm.sccgv
  ./encv serve -p mypassword -o ./my_videos --port 8080
`)
}
