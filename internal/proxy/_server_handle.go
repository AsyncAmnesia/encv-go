package proxy

import (
	"fmt"
	"net/http"

	"github.com/Soltus/encv-go/internal/v2/openlist"
	"github.com/Soltus/encv-go/internal/v2/provider"
	"github.com/Soltus/encv-go/internal/v2/reader"
)

func (p *Proxy) serveEncryptedContainer(w http.ResponseWriter, r *http.Request, containerURL string, headers map[string][]string, originalPath string) {

	// 1. 【责任转移】proxy 负责创建 URLResolver
	urlResolver := openlist.NewOpenListURLResolver(p.cfg, originalPath)

	// 2. 【责任转移】proxy 负责创建远程工厂
	factory, err := reader.NewRemoteDecryptReaderFactory(containerURL, p.cfg.Password, headers, urlResolver)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create remote decrypt reader factory: %v", err), http.StatusInternalServerError)
		return
	}

	// 3. 【责任转移】proxy 负责创建解密器
	decryptReader, err := factory.NewDecryptReader(*p.cfg)
	if err != nil {
		factory.Close() // 创建 reader 失败，需要关闭 factory
		http.Error(w, fmt.Sprintf("failed to create remote decrypt reader: %v", err), http.StatusInternalServerError)
		return
	}

	// 4. 【使用重构后的构造函数】proxy 将创建好的零件组装成 provider
	prov, err := provider.NewRemoteFileProvider(factory, decryptReader)
	if err != nil {
		// 如果 provider 创建失败，factory 和 reader 的生命周期由 proxy 管理
		decryptReader.Close()
		factory.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. provider 的生命周期由 ContentHandler 的 defer 管理
	defer prov.Close()

	// 6. 使用统一的内容处理器来服务文件
	p.contentHandler.ServeFile(w, r, prov)
}
