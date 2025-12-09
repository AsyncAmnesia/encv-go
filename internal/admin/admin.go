// internal/admin/admin.go
package admin

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/Soltus/encv-go/internal/admin/controller/file"
	"github.com/Soltus/encv-go/internal/admin/controller/hello" // 导入主服务的配置
	"github.com/Soltus/encv-go/internal/admin/logic/auth"
	"github.com/Soltus/encv-go/internal/admin/middleware"
	"github.com/Soltus/encv-go/internal/config"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// SetupAdminServer 配置并返回一个准备好启动的 GoFrame 管理服务器实例。
// 它不负责启动，只负责配置。
func SetupAdminServer(backendAddr string, cfg *config.Config) (*ghttp.Server, string) {
	// 这会告诉 GoFrame 在初始化 "admin" 服务器时，从我们指定的位置加载配置
	g.Cfg("internal/admin/manifest/config")
	proxyServer := g.Server("admin")
	proxyInstanceID := fmt.Sprintf("admin-%x", time.Now().UnixNano())

	// 1. 初始化认证管理器
	var authManager *auth.Manager
	loginRequired := cfg.Admin.Password != ""
	if loginRequired {
		authManager = auth.NewManager(24 * time.Hour) // 24小时会话
		log.Println("-> Admin service requires login.")
	} else {
		log.Println("-> Admin service is running without authentication (password is empty).")
	}

	// 2. 注册 Admin 路由 (这些路由通常也需要保护)
	adminGroup := proxyServer.Group("/admin")
	if loginRequired {
		adminGroup.Middleware(authMiddleware(authManager))
	}
	adminGroup.Middleware(middleware.Response)
	adminGroup.Bind(hello.NewV1())
	adminGroup.Bind(file.NewV1(cfg.Server.Dir))

	// 3. 注册代理和认证路由
	proxyServer.Group("/p", func(group *ghttp.RouterGroup) {
		if loginRequired {
			group.Middleware(authMiddleware(authManager))
		}

		// 登录/登出路由 (保持不变)
		group.GET("/login", func(r *ghttp.Request) { handleLogin(r, authManager, cfg.Admin.Password, "GET") })
		group.POST("/login", func(r *ghttp.Request) { handleLogin(r, authManager, cfg.Admin.Password, "POST") })
		group.GET("/logout", func(r *ghttp.Request) {
			sessionID := r.Cookie.Get("encv_session_id")
			if sessionID != nil {
				authManager.DestroySession(sessionID.String())
			}
			auth.ClearSessionCookie(r.Response.Writer)
			r.Response.RedirectTo("/p/login")
		})

		// 【关键】文件代理路由，现在包含响应修改逻辑
		u, err := url.Parse("http://" + backendAddr)
		if err != nil {
			log.Panicf("Failed to parse backend URL %s: %v", backendAddr, err)
		}
		proxy := httputil.NewSingleHostReverseProxy(u)
		proxy.Director = func(req *http.Request) {
			originalPath := req.URL.Path
			req.URL.Path = strings.TrimPrefix(originalPath, "/p")
			req.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, "/p")
			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
			req.Header.Set("X-Forwarded-Prefix", "/p")
			req.Header.Set("X-Forwarded-Host", req.Host)
			req.Header.Set("X-Forwarded-Proto", "http")
		}

		// 【核心】注册 ModifyResponse 函数来注入内容
		proxy.ModifyResponse = createResponseModifier()

		proxyServer.Group("/p-api", func(group *ghttp.RouterGroup) {
			// 【重要】API 路由也需要认证保护
			if loginRequired {
				group.Middleware(authMiddleware(authManager))
			}

			u, err := url.Parse("http://" + backendAddr)
			if err != nil {
				log.Panicf("Failed to parse backend URL for API %s: %v", backendAddr, err)
			}

			// 为API创建一个独立的代理实例
			apiProxy := httputil.NewSingleHostReverseProxy(u)
			// 【关键】重写路径：将 /p-api/... 替换为 /api/...
			apiProxy.Director = func(req *http.Request) {
				originalPath := req.URL.Path
				req.URL.Path = strings.TrimPrefix(originalPath, "/p-api")
				req.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, "/p-api")

				req.URL.Scheme = u.Scheme
				req.URL.Host = u.Host
				req.Header.Set("X-Forwarded-Host", req.Host)
				req.Header.Set("X-Forwarded-Proto", "http")
			}

			// 捕获所有 /p-api/... 的请求并转发
			group.ALL("/*", func(r *ghttp.Request) {
				r.MakeBodyRepeatableRead(false)
				apiProxy.ServeHTTP(r.Response.Writer, r.Request)
			})
		})

		group.ALL("/*", func(r *ghttp.Request) {
			r.MakeBodyRepeatableRead(false)
			proxy.ServeHTTP(r.Response.Writer, r.Request)
		})
	})

	return proxyServer, proxyInstanceID
}

// authMiddleware 创建一个认证中间件
func authMiddleware(manager *auth.Manager) func(r *ghttp.Request) {
	return func(r *ghttp.Request) {
		// 如果是访问登录页面，则直接放行
		if strings.HasSuffix(r.URL.Path, "/login") {
			r.Middleware.Next()
			return
		}

		sessionID := r.Cookie.Get("encv_session_id")
		if !manager.ValidateSession(sessionID.String()) {
			// 未登录，重定向到登录页面
			r.Response.RedirectTo("/p/login")
			return
		}
		// 在请求头中添加标记，供下游使用
		// 这个标记会随着请求一起被代理到后端，并在 ModifyResponse 中可见
		r.Header.Set("X-ENCV-User-Authenticated", "true")

		// 已登录，继续处理请求
		r.Middleware.Next()
	}
}

// handleLogin 处理登录逻辑
func handleLogin(r *ghttp.Request, manager *auth.Manager, correctPassword string, method string) {
	if method == "GET" {
		tmpl, _ := template.New("login").Parse(auth.LoginPageTmpl)
		r.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(r.Response.Writer, nil)
		return
	}

	password := r.Get("password").String()
	if password == correctPassword {
		sessionID := manager.CreateSession()
		auth.SetSessionCookie(r.Response.Writer, sessionID)
		r.Response.RedirectTo("/p/")
	} else {
		tmpl, _ := template.New("login").Parse(auth.LoginPageTmpl)
		r.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(r.Response.Writer, map[string]string{"Error": "Invalid password"})
	}
}

// createResponseModifier 创建一个响应修改器
func createResponseModifier() func(*http.Response) error {
	const contentInjectionPoint = `<div id="encv-content-injection-point"></div>`
	const headEndTag = "</head>"

	return func(resp *http.Response) error {
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/html") {
			return nil
		}

		originalBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		resp.Body.Close()
		// 【关键】从请求中获取当前路径，并去掉代理前缀 /p
		currentPath := strings.TrimPrefix(resp.Request.URL.Path, "/p")
		// 处理根路径的特殊情况
		if currentPath == "/" {
			currentPath = ""
		}
		isLoggedIn := resp.Request.Header.Get("X-ENCV-User-Authenticated") == "true"

		styleHTML, contentHTML := generateUserAssets(isLoggedIn, currentPath)

		if styleHTML == "" && contentHTML == "" {
			resp.Body = io.NopCloser(strings.NewReader(string(originalBody)))
			resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(originalBody)))
			return nil
		}

		newBody := string(originalBody)

		// 注入样式
		if styleHTML != "" {
			newBody = strings.ReplaceAll(newBody, headEndTag, styleHTML+headEndTag)
		}

		// 【核心】替换注入点
		if contentHTML != "" {
			newBody = strings.ReplaceAll(newBody, contentInjectionPoint, contentHTML)
		}

		resp.Body = io.NopCloser(strings.NewReader(newBody))
		resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(newBody)))

		return nil
	}
}

// generateUserAssets 生成样式和内容
func generateUserAssets(isLoggedIn bool, currentPath string) (styleHTML string, contentHTML string) {

	if isLoggedIn {
		styleHTML = `
	<style>
		/* --- 操作列样式 --- */
		.actions-cell {
			text-align: center;
			width: 180px; /* 给操作列一个固定宽度 */
			white-space: nowrap; /* 防止按钮换行 */
		}

		/* --- 操作按钮样式 (采用透明背景版本) --- */
		.action-btn {
			padding: 4px 8px;
			font-size: 0.8em;
			color: var(--link-color);
			background-color: transparent;
			border: 1px solid var(--border-color);
			border-radius: 4px;
			cursor: pointer;
			transition: all 0.2s ease;
		}
		.action-btn:hover {
			background-color: var(--link-color);
			color: white;
		}

		/* --- 用户状态样式 --- */
		.user-status {
			display: inline-flex;
			align-items: center;
			margin-left: 0.5em;
			padding: 0.3em 0.8em;
			background-color: var(--toolbar-btn-bg);
			border-radius: 12px;
			border: 1px solid var(--border-color);
			box-shadow: 0 2px 8px rgba(0,0,0,0.1);
			font-size: 0.9em;
			white-space: nowrap;
		}
		.user-status .status-text {
			color: var(--muted-text-color);
			margin-right: 0.5em;
		}
		.user-status .logout-btn {
			color: var(--link-color);
			text-decoration: none;
			font-weight: bold;
		}
		.user-status .logout-btn:hover {
			text-decoration: underline;
		}

		/* --- 对话框样式 --- */
		.analyze-dialog {
			width: 80%;
			max-width: 900px;
			border: 1px solid var(--border-color, #ccc);
			border-radius: 8px;
			padding: 20px;
			background-color: var(--bg-color, #f9f9f9);
			color: var(--text-color, #333);
			box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
			position: fixed;
		}

		.dialog-close-btn {
			position: absolute;
			top: 15px;
			right: 15px;
			background: none;
			border: none;
			font-size: 24px;
			font-weight: bold;
			line-height: 1;
			color: #888;
			cursor: pointer;
			padding: 0;
			width: 30px;
			height: 30px;
			display: flex;
			align-items: center;
			justify-content: center;
			border-radius: 50%;
			transition: background-color 0.2s, color 0.2s;
		}

		.dialog-close-btn:hover {
			background-color: #e0e0e0;
			color: #333;
		}

		.dialog-content {
			background-color: #fff;
			border: 1px solid #ddd;
			padding: 15px;
			border-radius: 4px;
			max-height: 70vh;
			overflow-y: auto;
			margin-top: 10px;
		}
		.dialog-content pre {
			margin: 0;
			white-space: pre-wrap;
			word-wrap: break-word;
		}
		.dialog-content code {
			background-color: transparent;
			padding: 0;
			font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
		}

		/* --- 暗黑模式适配 --- */
		body.hope-ui-dark .action-btn {
			/* 暗黑模式下，悬停效果由CSS变量自动处理 */
		}
		body.hope-ui-dark .dialog-close-btn {
			color: #aaa;
		}
		body.hope-ui-dark .dialog-close-btn:hover {
			background-color: #444;
			color: #fff;
		}
		body.hope-ui-dark .dialog-content {
			background-color: #1e1e1e;
			border-color: #444;
		}
	</style>`

		jsScript := `
	<script>
		(function() {
			// ================== DOM 操作与事件绑定 ==================
			document.addEventListener('DOMContentLoaded', function() {
				const table = document.querySelector('table');
				if (!table) return;

				const thead = table.querySelector('thead tr');
				const tbody = table.querySelector('tbody');

				// 1. 注入 "Actions" 表头
				if (thead && !thead.querySelector('.actions-cell')) {
					const actionsTh = document.createElement('th');
					actionsTh.textContent = 'Actions';
					actionsTh.className = 'actions-cell';
					thead.appendChild(actionsTh);
				}

				// 2. 遍历所有文件行，注入操作按钮
				if (tbody) {
					const rows = tbody.querySelectorAll('tr');
					rows.forEach(row => {
						const firstCell = row.querySelector('td:first-child a');
						if (!firstCell || firstCell.textContent.trim() === '..') return;

						const fileName = firstCell.textContent.trim();
						const actionTd = document.createElement('td');
						actionTd.className = 'actions-cell';

						const renameBtn = document.createElement('button');
						renameBtn.textContent = 'Rename';
						renameBtn.className = 'action-btn';
						renameBtn.onclick = () => showRenameDialog(fileName);

						const analyzeBtn = document.createElement('button');
						analyzeBtn.textContent = 'Analyze';
						analyzeBtn.className = 'action-btn';
						analyzeBtn.style.marginLeft = '5px';
						analyzeBtn.onclick = () => showAnalyzeDialog(fileName, fileName);

						actionTd.appendChild(renameBtn);
						actionTd.appendChild(analyzeBtn);
						row.appendChild(actionTd);
					});
				}
			});

			// ================== 功能函数 ==================
			function showAnalyzeDialog(baseName, fullOldPath) {
				const currentPathElement = document.getElementById('encv-current-path');
				let currentDirPath = currentPathElement ? currentPathElement.getAttribute('data-path') : '';
				if (currentDirPath && !currentDirPath.endsWith('/')) {
					currentDirPath += '/';
				}
				const fullPathToSend = currentDirPath + fullOldPath;

				let dialog = document.getElementById('analyze-dialog');
				if (!dialog) {
					dialog = document.createElement('dialog');
					dialog.id = 'analyze-dialog';
					dialog.className = 'analyze-dialog';
					document.body.appendChild(dialog);
				}

				// 【优化】创建辅助函数来更新对话框内容，减少重复代码
				const updateDialogContent = (title, contentHtml) => {
					dialog.innerHTML = '<h2>' + title + '</h2>' + contentHtml;
					const closeBtn = document.createElement('button');
					closeBtn.innerHTML = '&times;';
					closeBtn.className = 'dialog-close-btn';
					closeBtn.onclick = () => dialog.close();
					dialog.appendChild(closeBtn);
				};

				updateDialogContent('Analyzing: ' + baseName, '<p>Please wait...</p><div class="dialog-content"></div>');
				dialog.showModal();

				fetch('/admin/file/analyze', {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ path: fullPathToSend })
				})
				.then(response => {
					if (!response.ok) throw new Error('HTTP error! status: ' + response.status);
					return response.json();
				})
				.then(data => {
					if (data.code === 0) {
						updateDialogContent('Analysis Result for: ' + baseName, '<div class="dialog-content">' + data.data.htmlContent + '</div>');
					} else {
						throw new Error(data.message || 'An unknown error occurred.');
					}
				})
				.catch(error => {
					console.error('Analyze error:', error);
					updateDialogContent('Analysis Failed', '<div class="dialog-content" style="color: red;">Error: ' + error.message + '</div>');
				});
			}

			function showRenameDialog(fileName) {
				const currentPathElement = document.getElementById('encv-current-path');
				let currentDirPath = currentPathElement ? currentPathElement.getAttribute('data-path') : '';
				if (currentDirPath && !currentDirPath.endsWith('/')) {
					currentDirPath += '/';
				}
				const fullOldPath = currentDirPath + fileName;

				const newName = prompt('Enter new name for "' + fileName + '":', fileName);
				if (newName && newName !== fileName) {
					const cleanedName = newName.replace(/[^\\w\\s.-]/g, '');
					if (!cleanedName) {
						alert('Error: The new name contains no valid characters.');
						return;
					}
					fetch('/admin/file/rename', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json','Accept': 'application/json'  },
						body: JSON.stringify({ oldPath: fullOldPath, newName: cleanedName })
					})
					.then(response => {
						if (!response.ok) throw new Error('HTTP error! status: ' + response.status);
						return response.json();
					})
					.then(data => {
						if (data.code === 0) {
							alert('Success: ' + data.message);
							location.reload();
						} else {
							throw new Error(data.message || 'An unknown error occurred.');
						}
					})
					.catch(error => {
						console.error('Rename error:', error);
						alert('Rename Failed: ' + error.message);
					});
				}
			}
		})();
	</script>`

		// 将JS追加到样式HTML中，一起注入
		styleHTML += jsScript

		// 注入一个 span，因为它在 flex 容器中与按钮表现一致
		// 在 contentHTML 中增加一个隐藏元素来存储路径
		contentHTML = `
		<span class="user-status"><span class="status-text">Logged In</span><a href="/p/logout" class="logout-btn">Logout</a></span>
		<div id="encv-current-path" data-path="` + currentPath + `"></div>
	`

		return styleHTML, contentHTML
	}

	return "", ""
}
