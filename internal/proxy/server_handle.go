package proxy

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// serveDecryptedStreamFromReader 从一个 io.ReadCloser 解密并服务流
// useChunked 为 true 时，使用分块传输；为 false 时，使用定长传输
func serveDecryptedStreamFromReader(w http.ResponseWriter, encryptedReader io.ReadCloser, index types.Index, password string, useChunked bool) error {
	defer encryptedReader.Close()

	// 1. 从 KVI 获取解密所需信息
	salt, err := crypto.Base64Decode(index.GetEncryptionInfo().SaltBase64)
	if err != nil {
		log.Printf("Error decoding salt: %v", err)
		http.Error(w, "Invalid salt in index file", http.StatusInternalServerError)
		return err
	}
	key := crypto.GenerateKey(password, salt)

	// 2. 【关键】创建一个透明的解密流
	// 它会从 encryptedReader 读取加密数据，并吐出解密后的原始数据
	decryptedReader, err := crypto.GetDecryptReader(encryptedReader, key)
	if err != nil {
		log.Printf("Error creating decrypt reader: %v", err)
		http.Error(w, "Failed to initialize decryption", http.StatusInternalServerError)
		return err
	}

	// 3. 【关键】设置响应头，不设置 Content-Length
	// 这会让 Go 自动使用分块传输，最稳健的流式方案
	originalFilename := index.GetOriginalFilename()
	contentType := index.GetMimeType()

	log.Printf("-> [Proxy] Streaming content: %s (%s) using chunked encoding.", originalFilename, contentType)

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", originalFilename))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	// 不再设置 Content-Length 和 Accept-Ranges

	// 4. 【关键】将解密流直接泵送到 HTTP 响应体
	if _, err := io.Copy(w, decryptedReader); err != nil {
		// 客户端断开连接是正常行为，不应记录为错误
		if !utils.IsConnectionClosedError(err) {
			log.Printf("Error streaming decrypted content to client: %v", err)
		}
		return err
	}

	return nil
}
