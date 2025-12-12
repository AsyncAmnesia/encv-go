// internal/admin/logic/openlist/multi_site_server.go
package openlist

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Soltus/encv-go/internal/admin/injector"
	"github.com/Soltus/encv-go/internal/admin/logic/auth"
	"github.com/Soltus/encv-go/internal/admin/middleware"
	"github.com/Soltus/encv-go/internal/admin/routes"
	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/v2/types"
	"github.com/Soltus/encv-go/internal/web"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// MultiSiteServer 多站点代理路由设置器（不是独立服务器）
type MultiSiteServer struct {
	ctx          context.Context
	cfg          *config.Config
	tokenManager *TokenManager
	proxyGhttp   *ProxyGhttp
}

// NewMultiSiteServer 创建多站点代理路由设置器实例
func NewMultiSiteServer(ctx context.Context) *MultiSiteServer {
	cfg := config.FromContext(ctx)
	tokenManager := NewTokenManager(ctx)
	proxyGhttp := NewProxyGhttp(ctx)

	return &MultiSiteServer{
		ctx:          ctx,
		cfg:          cfg,
		tokenManager: tokenManager,
		proxyGhttp:   proxyGhttp,
	}
}

// SetupRoutes 在给定的服务器上设置多站点代理路由
func (m *MultiSiteServer) SetupRoutes(server *ghttp.Server, jwtManager *auth.JWTManager) *ghttp.RouterGroup {
	loginRequired := m.cfg.Admin.Password != ""
	server.BindMiddleware("/*", middleware.CORS)
	return server.Group(routes.OpenListProxy, func(group *ghttp.RouterGroup) {

		// 多站点代理路由
		group.Group("/sites/{siteId}", func(group *ghttp.RouterGroup) {
			// 验证站点 token
			group.Middleware(func(r *ghttp.Request) {
				siteId := r.GetRouter("siteId").String()

				// 验证站点是否存在
				siteConfig, exists := m.cfg.Proxy.Sites[siteId]
				if !exists {
					r.Response.WriteStatusExit(http.StatusNotFound, "Site not found")
					return
				}

				// 获取 token
				token, hasToken := m.tokenManager.GetToken(siteId)
				if !hasToken {
					r.Response.WriteStatusExit(http.StatusUnauthorized, "Token required. Please set token first.")
					return
				}

				// 设置站点配置到上下文，供 ProxyGhttp 使用
				r.SetCtxVar("siteHost", siteConfig.Host)
				r.SetCtxVar("siteId", siteId)
				r.SetCtxVar("siteToken", token)

				// 【关键】动态构建并存储路径前缀
				pathPrefix := "/openlist/sites/" + siteId
				r.SetCtxVar("pathPrefix", pathPrefix)

				// 【新增】如果是解密请求，预处理文件路径
				if strings.HasSuffix(r.URL.Path, "/decrypt") {
					fileURL := r.URL.Query().Get("file")
					if fileURL != "" {
						// 解析文件URL
						if parsedURL, err := url.Parse(fileURL); err == nil {
							// 移除路径前缀
							if strings.HasPrefix(parsedURL.Path, pathPrefix) {
								cleanPath := strings.TrimPrefix(parsedURL.Path, pathPrefix)
								if cleanPath == "" {
									cleanPath = "/"
								}
								// 重新构建文件URL
								parsedURL.Path = cleanPath
								query := r.URL.Query()
								query.Set("file", parsedURL.String())
								r.URL.RawQuery = query.Encode()
							}
						}
					}
				}

				r.Middleware.Next()
			})
			// iframe 预览文件
			group.ALL("/_preview/*", func(r *ghttp.Request) {
				siteId := r.GetRouter("siteId").String()
				handler := http.StripPrefix("/openlist/sites/"+siteId+"/_preview/", web.PreviewHandler())
				handler.ServeHTTP(r.Response.Writer, r.Request)
			})

			group.POST("/decrypt", func(r *ghttp.Request) {
				m.proxyGhttp.HandleRequest(r)
			})
			// 直接使用 ProxyGhttp 处理所有请求
			group.ALL("/*", m.proxyGhttp.HandleRequest)
		})

		// 在 sites/{siteId} 之后，避免验证代理
		if loginRequired {
			group.Middleware(middleware.AuthMiddleware(jwtManager))
		}
		// 站点选择页面
		group.GET("/sites", func(r *ghttp.Request) {
			handleSiteSelection(r, m.cfg, m.tokenManager)
		})

		// 设置站点 token
		group.POST("/set-token", func(r *ghttp.Request) {
			handleSetSiteToken(r, m.cfg, m.tokenManager)
		})

		// 删除 token
		group.POST("/delete-token", func(r *ghttp.Request) {
			m.handleDeleteToken(r)
		})

		// 设置 token 有效期
		group.POST("/set-expiry", func(r *ghttp.Request) {
			m.handleSetExpiry(r)
		})
	})

}

// 站点选择和 token 输入页面
func handleSiteSelection(r *ghttp.Request, cfg *config.Config, tokenManager *TokenManager) {
	// 生成站点列表页面
	html := generateTokenInputPage(cfg.Proxy.Sites, tokenManager)
	r.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	r.Response.Write(html)
}

func generateTokenInputPage(sites map[string]types.ProxySiteConfig, tokenManager *TokenManager) string {
	var siteCards string
	for siteId, config := range sites {
		_, hasToken := tokenManager.GetToken(siteId)
		status := "❌ No Token"
		expiryInfo := ""

		if hasToken {
			status = "✅ Configured"
			// 获取token详细信息包括过期时间
			if siteToken := tokenManager.GetSiteToken(siteId); siteToken != nil {
				expiryInfo = fmt.Sprintf("Expires: %s", siteToken.ExpiresAt.Format("2006-01-02 15:04:05"))
			}
		}

		accessLinkStyle := "style='display:none;'"
		accessLink := ""
		if hasToken {
			accessLinkStyle = ""
			// 访问链接指向站点原始host，而不是代理路径
			accessLink = fmt.Sprintf(`<a href="%s" target="_blank" class="access-link" %s>Visit Site</a>`,
				config.Host, accessLinkStyle)
		}

		// 【修正】使用 Go 的条件判断
		expiryButtonDisabled := ""
		deleteButtonDisabled := ""
		if !hasToken {
			expiryButtonDisabled = "disabled"
			deleteButtonDisabled = "disabled"
		}

		siteCards += fmt.Sprintf(`
    <div class="site-card">
        <h3>%s</h3>
        <p>%s</p>
        <p class="status">Status: %s</p>
        <p class="expiry">%s</p>
        <form method="post" action="%s/set-token" class="token-form">
            <input type="hidden" name="siteId" value="%s">
            <input type="password" name="token" placeholder="Enter token for this site" required>
            <button type="submit">Save Token</button>
        </form>
        <div class="action-buttons">
            %s
            <button onclick="showExpiryDialog('%s')" class="expiry-btn" %s>Set Expiry</button>
            <button onclick="deleteToken('%s')" class="delete-btn" %s>Delete Token</button>
        </div>
    </div>
`, siteId, config.Description, status, expiryInfo,
			routes.OpenListProxy, siteId,
			accessLink,
			siteId, expiryButtonDisabled,
			siteId, deleteButtonDisabled)
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ENCV - OpenList Sites</title>
    <style>
        /* --- CSS 变量定义 --- */
        :root {
            --bg-color: #f4f4f9;
            --text-color: #333;
            --muted-text-color: #666;
            --card-bg-color: #f9f9f9;
            --border-color: #ddd;
            --header-text-color: #333;
            --status-text-color: #333;
            --expiry-text-color: #888;
            --input-bg-color: #fff;
            --input-border-color: #ddd;
            --input-text-color: #333;
            --btn-primary-bg: #007bff;
            --btn-primary-text: #fff;
            --btn-success-bg: #28a745;
            --btn-success-text: #fff;
            --btn-success-hover-bg: #218838;
            --btn-warning-bg: #ffc107;
            --btn-warning-text: #212529;
            --btn-danger-bg: #dc3545;
            --btn-danger-text: #fff;
            --btn-disabled-opacity: 0.5;
            --modal-bg-color: #fefefe;
            --modal-overlay-bg: rgba(0,0,0,0.4);
            --link-color: #007bff;
            --link-hover-color: #0056b3;
            --selection-bg: rgba(46, 170, 220, 0.3);
        }

        body.hope-ui-dark {
            --bg-color: #1a1a1a;
            --text-color: #e6edf3;
            --muted-text-color: #8b949e;
            --card-bg-color: #161b22;
            --border-color: #30363d;
            --header-text-color: #e6edf3;
            --status-text-color: #e6edf3;
            --expiry-text-color: #8b949e;
            --input-bg-color: #0d1117;
            --input-border-color: #30363d;
            --input-text-color: #e6edf3;
            --btn-primary-bg: #238636;
            --btn-primary-text: #fff;
            --btn-success-bg: #238636;
            --btn-success-text: #fff;
            --btn-success-hover-bg: #2ea043;
            --btn-warning-bg: #9e6a03;
            --btn-warning-text: #fff;
            --btn-danger-bg: #da3633;
            --btn-danger-text: #fff;
            --btn-disabled-opacity: 0.3;
            --modal-bg-color: #161b22;
            --modal-overlay-bg: rgba(0,0,0,0.8);
            --link-color: #58a6ff;
            --link-hover-color: #79c0ff;
            --selection-bg: rgba(46, 170, 220, 0.4);
        }

        /* --- 全局样式 --- */
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
            max-width: 1200px; 
            margin: 20px auto; 
            padding: 20px; 
            background-color: var(--bg-color);
            color: var(--text-color);
            transition: background-color 0.3s ease, color 0.3s ease;
        }
        
        .header { 
            text-align: center; 
            margin-bottom: 30px; 
        }
        .header h1 {
            color: var(--muted-text-color)
            margin-bottom: 10px;
        }
        .header p {
            color: var(--muted-text-color);
        }
        
        .sites-grid { 
            display: grid; 
            grid-template-columns: repeat(auto-fit, minmax(350px, 1fr)); 
            gap: 20px; 
        }
        
        .site-card { 
            border: 1px solid var(--border-color); 
            padding: 20px; 
            border-radius: 8px; 
            background-color: var(--card-bg-color);
            transition: transform 0.2s ease, box-shadow 0.2s ease;
        }
        .site-card:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(0,0,0,0.1);
        }
        
        .site-card h3 { 
            margin: 0 0 10px 0; 
            color: var(--text-color);
        }
        .site-card p { 
            margin: 5px 0; 
            color: var(--muted-text-color);
        }
        .status { 
            font-weight: bold; 
            margin: 10px 0; 
            color: var(--status-text-color);
        }
        .expiry { 
            font-size: 0.9em; 
            color: var(--expiry-text-color); 
            margin: 5px 0; 
        }
        
        .token-form { 
            display: flex; 
            gap: 10px; 
            margin: 15px 0; 
        }
        .token-form input { 
            flex: 1; 
            padding: 8px; 
            border: 1px solid var(--input-border-color); 
            border-radius: 4px; 
            background-color: var(--input-bg-color);
            color: var(--input-text-color);
        }
        .token-form input:focus {
            outline: none;
            border-color: var(--btn-primary-bg);
        }
        .token-form button { 
            padding: 8px 16px; 
            background: var(--btn-primary-bg); 
            color: var(--btn-primary-text); 
            border: none; 
            border-radius: 4px; 
            cursor: pointer;
            transition: background-color 0.2s ease;
        }
        .token-form button:hover {
            opacity: 0.9;
        }
        
        .action-buttons { 
            display: flex; 
            gap: 10px; 
            margin-top: 10px; 
            flex-wrap: wrap; 
        }
        
        .access-link { 
            display: inline-block; 
            padding: 8px 16px; 
            background: var(--btn-success-bg); 
            color: var(--btn-success-text); 
            text-decoration: none; 
            border-radius: 4px;
            transition: background-color 0.2s ease;
        }
        .access-link:hover { 
            background: var(--btn-success-hover-bg); 
        }
        
        .expiry-btn, .delete-btn { 
            padding: 6px 12px; 
            border: none; 
            border-radius: 4px; 
            cursor: pointer; 
            font-size: 0.9em;
            transition: all 0.2s ease;
        }
        .expiry-btn { 
            background: var(--btn-warning-bg); 
            color: var(--btn-warning-text); 
        }
        .delete-btn { 
            background: var(--btn-danger-bg); 
            color: var(--btn-danger-text); 
        }
        .expiry-btn:hover, .delete-btn:hover {
            opacity: 0.9;
            transform: translateY(-1px);
        }
        .expiry-btn:disabled, .delete-btn:disabled { 
            opacity: var(--btn-disabled-opacity); 
            cursor: not-allowed; 
            transform: none;
        }
        
        .modal { 
            display: none; 
            position: fixed; 
            z-index: 1000; 
            left: 0; 
            top: 0; 
            width: 100%%; 
            height: 100%%; 
            background-color: var(--modal-overlay-bg);
        }
        .modal-content { 
            background-color: var(--modal-bg-color); 
            margin: 15%% auto; 
            padding: 20px; 
            border: 1px solid var(--border-color);
            border-radius: 8px; 
            width: 300px;
            box-shadow: 0 4px 12px rgba(0,0,0,0.2);
        }
        .modal-content h3 { 
            margin-top: 0;
            color: var(--text-color);
        }
        .modal-content label {
            display: block;
            margin-bottom: 5px;
            color: var(--muted-text-color);
        }
        .modal-content input { 
            width: 100%%; 
            padding: 8px; 
            margin: 10px 0; 
            border: 1px solid var(--input-border-color); 
            border-radius: 4px;
            background-color: var(--input-bg-color);
            color: var(--input-text-color);
        }
        .modal-content input:focus {
            outline: none;
            border-color: var(--btn-primary-bg);
        }
        .modal-content button { 
            padding: 8px 16px; 
            margin-right: 10px; 
            border: none; 
            border-radius: 4px; 
            cursor: pointer;
            transition: all 0.2s ease;
        }
        .confirm-btn { 
            background: var(--btn-primary-bg); 
            color: var(--btn-primary-text);
        }
        .confirm-btn:hover {
            opacity: 0.9;
        }
        .cancel-btn { 
            background: var(--muted-text-color); 
            color: var(--btn-primary-text);
        }
        .cancel-btn:hover {
            opacity: 0.8;
        }
    </style>
    <script>
        function deleteToken(siteId) {
            if (confirm('Are you sure you want to delete the token for ' + siteId + '?')) {
                fetch('%s/delete-token', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({siteId: siteId})
                }).then(() => location.reload());
            }
        }
        
        function showExpiryDialog(siteId) {
            document.getElementById('expirySiteId').value = siteId;
            document.getElementById('expiryModal').style.display = 'block';
        }
        
        function hideExpiryDialog() {
            document.getElementById('expiryModal').style.display = 'none';
        }
        
        function setExpiry() {
    const siteId = document.getElementById('expirySiteId').value;
    const daysInput = document.getElementById('expiryDays');
    const days = parseInt(daysInput.value, 10); // 使用 parseInt 并指定进制

    // 【新增】客户端验证
    if (isNaN(days) || days < 1 || days > 365) {
        alert('Error: Days must be a number between 1 and 365.');
        return; // 阻止请求发送
    }

    // 【新增】显示加载状态（可选，但推荐）
    const confirmBtn = document.querySelector('.modal-content .confirm-btn');
    const originalText = confirmBtn.innerText;
    confirmBtn.innerText = 'Setting...';
    confirmBtn.disabled = true;

    fetch('%s/set-expiry', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({siteId: siteId, days: days})
    })
    .then(response => {
        if (!response.ok) {
            return response.json().then(errData => {
                throw new Error(errData.message || "Server error: "+response.statusText);
            });
        }
        return response.json();
    })
    .then(data => {
        console.log('Success:', data.message);
        location.reload();
    })
    .catch(error => {
        console.error('Set expiry failed:', error);
        alert('Failed to set expiry: ' + error.message);
        confirmBtn.innerText = originalText;
        confirmBtn.disabled = false;
    });
}

        // 点击模态框外部关闭
        window.onclick = function(event) {
            const modal = document.getElementById('expiryModal');
            if (event.target == modal) {
                modal.style.display = 'none';
            }
        }
    </script>
</head>
<body>
    <!-- 【新增】注入点 -->
<div id="`+injector.InjectorID+`"></div>
    
    <div class="header">
        <h1>OpenList Site Management</h1>
        <p>Configure tokens for your OpenList sites</p>
    </div>
    <div class="sites-grid">
        %s
    </div>
    
    <!-- 设置有效期的模态框 -->
    <div id="expiryModal" class="modal">
        <div class="modal-content">
            <h3>Set Token Expiry</h3>
            <input type="hidden" id="expirySiteId">
            <label for="expiryDays">Days until expiry:</label>
            <input type="number" id="expiryDays" min="1" max="365" value="30">
            <button class="confirm-btn" onclick="setExpiry()">Set</button>
            <button class="cancel-btn" onclick="hideExpiryDialog()">Cancel</button>
        </div>
    </div>
</body>
</html>
    `, routes.OpenListProxy, routes.OpenListProxy, siteCards)
}

func handleSetSiteToken(r *ghttp.Request, cfg *config.Config, tokenManager *TokenManager) {
	var req struct {
		SiteID string `json:"siteId"`
		Token  string `json:"token"`
	}

	if err := r.Parse(&req); err != nil {
		r.Response.WriteStatusExit(http.StatusBadRequest, "Invalid request")
		return
	}
	// 验证站点是否存在
	if _, exists := cfg.Proxy.Sites[req.SiteID]; !exists {
		r.Response.WriteStatusExit(http.StatusNotFound, "Site not found")
		return
	}

	// 设置 token（会自动保存到文件）
	tokenManager.SetToken(req.SiteID, req.Token)

	// 【修正】重定向回站点选择页面，而不是返回JSON
	r.Response.RedirectTo(routes.OpenListProxy + "/sites")
}

// 删除token的处理函数
func (m *MultiSiteServer) handleDeleteToken(r *ghttp.Request) {
	var req struct {
		SiteID string `json:"siteId"`
	}

	if err := r.Parse(&req); err != nil {
		r.Response.WriteStatusExit(http.StatusBadRequest, "Invalid request")
		return
	}

	// 删除token
	m.tokenManager.RemoveToken(req.SiteID)

	// 返回成功
	r.Response.WriteJsonExit(g.Map{
		"success": true,
		"message": "Token deleted successfully",
	})
}

// 设置有效期的处理函数
func (m *MultiSiteServer) handleSetExpiry(r *ghttp.Request) {
	var req struct {
		SiteID string `json:"siteId"`
		Days   int    `json:"days"`
	}

	if err := r.Parse(&req); err != nil {
		r.Response.WriteStatusExit(http.StatusBadRequest, "Invalid request")
		return
	}

	if req.Days < 1 || req.Days > 365 {
		r.Response.WriteStatusExit(http.StatusBadRequest, "Days must be between 1 and 365")
		return
	}

	// 设置有效期
	if err := m.tokenManager.SetTokenExpiry(req.SiteID, req.Days); err != nil {
		r.Response.WriteStatusExit(http.StatusInternalServerError, err.Error())
		return
	}

	// 返回成功
	r.Response.WriteJsonExit(g.Map{
		"success": true,
		"message": "Token expiry updated successfully",
	})
}
