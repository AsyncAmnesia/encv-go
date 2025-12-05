// internal/v2/plugins/generic/generic.go

package generic

// import (
// 	"context"
// 	"fmt"
// 	"io"
// 	"os"
// 	"path/filepath"
// 	"strings"

// 	"github.com/Soltus/encv-go/internal/config"
// 	"github.com/Soltus/encv-go/internal/v2/crypto"
// 	"github.com/Soltus/encv-go/internal/v2/types"
// 	"github.com/Soltus/encv-go/internal/v2/writer"
// )

// type GenericPlugin struct{}

// func (p *GenericPlugin) SupportedMimePrefixes() []string {
// 	return []string{"application/octet-stream"}
// }

// func (p *GenericPlugin) ShouldProcess(inputPath string) bool {
// 	return true
// }

// func (p *GenericPlugin) ProcessFile(inputPath string) (types.Index, io.ReadCloser, error) {
// 	file, err := os.Open(inputPath)
// 	if err != nil {
// 		return nil, nil, fmt.Errorf("failed to open file: %w", err)
// 	}
// 	index := &types.GenericIndex{Kind: types.IndexKindGeneric}
// 	return index, file, nil
// }

// func (p *GenericPlugin) GetContainerExtension() string {
// 	return ".sccgi"
// }

// // --- 加密逻辑 ---

// func (p *GenericPlugin) PreEncryptProcessor(ctx context.Context, index types.Index, inputPath, inputRootDir, outputDir string) error {
// 	// 通用插件无需预处理
// 	return nil
// }

// func (p *GenericPlugin) Encrypt(ctx context.Context, index types.Index, dataReader io.ReadCloser, inputPath, inputRootDir, outputDir string) error {
// 	if index == nil || dataReader == nil {
// 		var err error
// 		index, dataReader, err = p.ProcessFile(inputPath)
// 		if err != nil {
// 			return err
// 		}
// 		defer dataReader.Close()
// 	}
// 	gIndex, ok := index.(*types.GenericIndex)
// 	if !ok {
// 		return fmt.Errorf("generic plugin received a non-generic index")
// 	}

// 	cfg := config.FromContext(ctx)
// 	password := cfg.Password

// 	relPath, _ := filepath.Rel(inputRootDir, inputPath)
// 	outputFilePath := filepath.Join(outputDir, relPath+p.GetContainerExtension())
// 	if err := os.MkdirAll(filepath.Dir(outputFilePath), 0755); err != nil {
// 		return err
// 	}

// 	w, err := writer.NewSingleFileContainerWriter(outputFilePath)
// 	if err != nil {
// 		return err
// 	}
// 	defer w.Close()

// 	salt, _ := crypto.GenerateSalt_v2(types.SaltSize_v2)
// 	iv, _ := crypto.GenerateIV_v2(types.IVSize_v2)
// 	key := crypto.GenerateKey_v2(password, salt, types.KeySize_v2)

// 	kvi := types.KVI_v2{SaltBase64: crypto.Base64Encode_v2(salt), IVBase64: crypto.Base64Encode_v2(iv)}
// 	genericKVI := types.GenericKVI_v2{KVI_v2: kvi, GenericIndex: gIndex}

// 	encryptedData, err := crypto.EncryptReaderToBytes_v2(dataReader, key, iv)
// 	if err != nil {
// 		return fmt.Errorf("failed to encrypt data: %w", err)
// 	}

// 	frag := types.Fragment_v2{ID: "main_content", Type: "atomic_file", Filename: filepath.Base(inputPath), Length: uint64(len(encryptedData))}
// 	w.WriteFragment(&frag, encryptedData)

// 	manifest, err := types.NewManifest_v2(genericKVI, []types.Fragment_v2{frag})
// 	w.WriteManifest(manifest)

// 	fmt.Printf("✅ [GENERIC] Encrypted to: %s\n", outputFilePath)
// 	return nil
// }

// func (p *GenericPlugin) PostEncryptProcessor(ctx context.Context, index types.Index, outputDir string) error {
// 	// 通用插件无需后处理
// 	return nil
// }

// // --- 解密逻辑 ---

// func (p *GenericPlugin) CanDecrypt(containerPath string) bool {
// 	return strings.HasSuffix(containerPath, p.GetContainerExtension())
// }

// func (p *GenericPlugin) PreDecryptProcessor(ctx context.Context, containerPath, outputDir string) error {
// 	// 通用插件无需预处理
// 	return nil
// }

// func (p *GenericPlugin) Decrypt(ctx context.Context, containerPath, outputDir string) error {
// 	// ... (实现通用解密逻辑) ...
// 	fmt.Printf("-> [GENERIC] Decrypting %s\n", containerPath)
// 	return nil
// }

// func (p *GenericPlugin) PostDecryptProcessor(ctx context.Context, index types.Index, containerPath, outputDir string) error {
// 	// 通用插件无需后处理
// 	return nil
// }
