package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/pkg/encv"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cfg, err := config.LoadUserConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	switch os.Args[1] {
	case "encrypt":
		// ... (encrypt 逻辑保持不变) ...
		encryptCmd := flag.NewFlagSet("encrypt", flag.ExitOnError)
		passwordPtr := encryptCmd.String("p", cfg.Password, "Password for encryption")
		outputPtr := encryptCmd.String("o", cfg.OutputPath, "Output directory")
		err := encryptCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("Error parsing flags: %v", err)
		}

		inputPath := encryptCmd.Arg(0)
		if inputPath == "" {
			log.Fatal("Error: Please provide the path to the video file or directory to encrypt.")
		}

		opts := encv.EncryptOptions{
			Password:        *passwordPtr,
			OutputDir:       *outputPtr,
			TrackExtensions: cfg.TrackExtensions,
		}
		if err := encv.Encrypt(inputPath, opts); err != nil {
			log.Fatalf("Encryption failed: %v", err)
		}
		log.Printf("✅ Encryption complete. Output in: %s\n", opts.OutputDir)

	case "decrypt":
		decryptCmd := flag.NewFlagSet("decrypt", flag.ExitOnError)
		passwordPtr := decryptCmd.String("p", cfg.Password, "Password for decryption")
		outputPtr := decryptCmd.String("o", "./decrypted", "Output directory for decrypted files")
		force := decryptCmd.Bool("f", false, "Force overwrite existing output files")
		err := decryptCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("Error parsing flags: %v", err)
		}

		inputPath := decryptCmd.Arg(0)
		if inputPath == "" {
			// 【修正】更新提示信息
			log.Fatal("Error: Please provide the path to the ENCV container file to decrypt.")
		}

		opts := encv.DecryptOptions{
			Password:  *passwordPtr,
			OutputDir: *outputPtr,
			Force:     *force,
		}
		if err := encv.Decrypt(inputPath, opts); err != nil {
			log.Fatalf("Decryption failed: %v", err)
		}
		log.Printf("✅ Decryption complete. Output in: %s\n", opts.OutputDir)

	// 【新增】kvi 命令
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

		// 调用新函数获取 KVI 数据
		kviData, err := encv.ExtractKVI(containerPath)
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

	case "serve":
		// ... (serve 逻辑保持不变) ...
		serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
		passwordPtr := serveCmd.String("p", cfg.Password, "Password for server")
		outputPtr := serveCmd.String("o", cfg.OutputPath, "Directory to serve from")
		portPtr := serveCmd.Int("port", cfg.Port, "Port to run the server on")
		err := serveCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("Error parsing flags: %v", err)
		}

		dirFromFlag := *outputPtr
		dirFromArg := serveCmd.Arg(0)
		finalDir := dirFromFlag
		if finalDir == "" {
			finalDir = dirFromArg
		}
		if finalDir == "" {
			finalDir = cfg.OutputPath
		}
		if finalDir == "" {
			finalDir = "./output"
		}

		player := encv.NewPlayer(finalDir, *passwordPtr)
		addr, err := player.Start(*portPtr)
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}

		log.Printf("\n✅ Server started successfully!\n")
		log.Printf("   Serving files from: %s\n", finalDir)
		log.Printf("   Access it at: http://localhost%s\n", addr)
		log.Println("\n--- How to Play ---")
		log.Printf("   mpv --no-config http://localhost%s/<video_name_without_extension>\n", addr)
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
