package main

import (
	"flag"
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
		err := decryptCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("Error parsing flags: %v", err)
		}

		inputPath := decryptCmd.Arg(0)
		if inputPath == "" {
			log.Fatal("Error: Please provide the path to the .enc file to decrypt.")
		}

		opts := encv.DecryptOptions{
			Password:  *passwordPtr,
			OutputDir: *outputPtr,
		}
		if err := encv.Decrypt(inputPath, opts); err != nil {
			log.Fatalf("Decryption failed: %v", err)
		}
		log.Printf("✅ Decryption complete. Output in: %s\n", opts.OutputDir)

	case "serve":
		serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
		passwordPtr := serveCmd.String("p", cfg.Password, "Password for server")
		outputPtr := serveCmd.String("o", cfg.OutputPath, "Directory to serve from")
		portPtr := serveCmd.Int("port", cfg.Port, "Port to run the server on")
		err := serveCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("Error parsing flags: %v", err)
		}

		// 确定最终的目录路径: -o flag > path > config > default
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
		log.Printf("   mpv --no-config http://localhost%s/video/<video_name_without_extension>\n", addr)
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
  decrypt <input_path>  Decrypt a single .enc file and its associated tracks.
  serve   <directory>   Start a server to stream encrypted videos from a directory.

Flags:
  -p, --password <pwd>  Password for encryption/decryption.
  -o, --output <path>   Output directory.
  --port <number>       Port to run the server on (for 'serve' command).

Examples:
  ./encv encrypt -o ./my_encrypted_videos ./my_videos
  ./encv decrypt -p mypassword -o ./my_decrypted_movie/ ./output/movie.mkv.enc
  ./encv serve -p mypassword -o ./my_videos --port 8080
`)
}
