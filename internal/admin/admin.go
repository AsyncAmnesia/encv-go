// internal/admin/admin.go
package admin

import (
	"context"
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
	"github.com/Soltus/encv-go/internal/admin/injector"
	"github.com/Soltus/encv-go/internal/admin/logic/auth"
	"github.com/Soltus/encv-go/internal/admin/logic/openlist"
	"github.com/Soltus/encv-go/internal/admin/middleware"
	"github.com/Soltus/encv-go/internal/admin/routes"
	"github.com/Soltus/encv-go/internal/config"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// SetupAdminServer 配置并返回一个准备好启动的 GoFrame 管理服务器实例。
// 它不负责启动，只负责配置。
func SetupAdminServer(backendAddr string, ctx context.Context) (*ghttp.Server, string) {
	cfg := config.FromContext(ctx)
	// 这会告诉 GoFrame 在初始化 "admin" 服务器时，从我们指定的位置加载配置
	g.Cfg("internal/admin/manifest/config")
	proxyServer := g.Server("Admin Server")
	proxyInstanceID := fmt.Sprintf("admin-%x", time.Now().UnixNano())

	// 创建用户注入器
	userInjector := injector.NewUserInjector(routes.Logout)

	// 1. 初始化JWT认证管理器
	var jwtManager *auth.JWTManager
	loginRequired := cfg.Admin.Password != ""
	if loginRequired {
		// 创建JWT管理器，7天有效期
		jwtManager = auth.NewJWTManager(cfg.Admin.Password, 7*24*time.Hour)
		log.Println("-> Admin service requires login.")
	} else {
		log.Println("-> Admin service is running without authentication (password is empty).")
	}
	proxyServer.Group("", func(group *ghttp.RouterGroup) {
		// 登录/登出路由
		group.GET(routes.Login, func(r *ghttp.Request) { handleLogin(r, jwtManager, cfg.Admin.Password, "GET") })
		group.POST(routes.Login, func(r *ghttp.Request) { handleLogin(r, jwtManager, cfg.Admin.Password, "POST") })
		group.ALL(routes.Logout, func(r *ghttp.Request) {
			auth.ClearAuthCookie(r.Response.Writer)
			r.Response.RedirectTo(routes.Login)
		})

	})
	// 全局 CORS
	proxyServer.BindMiddleware("/*", middleware.CORS)

	// 2. 注册 Admin 路由 (这些路由通常也需要保护)
	adminGroup := proxyServer.Group(routes.Admin)
	if loginRequired {
		adminGroup.Middleware(middleware.AuthMiddleware(jwtManager))
	}
	adminGroup.Middleware(middleware.Response)
	adminGroup.Bind(hello.NewV1())
	adminGroup.Bind(file.NewV1(cfg.Server.Dir))

	// 3. 创建并设置 OpenList 多站点代理路由
	if len(cfg.Proxy.Sites) > 0 {
		multiSiteServer := openlist.NewMultiSiteServer(ctx)
		multiSiteServer.SetupRoutes(proxyServer, jwtManager)
		log.Printf("-> OpenList multi-site proxy is available at: http://localhost:%d%s", cfg.Admin.Port, routes.OpenListProxy)
	}

	// 4. 注册代理和认证路由
	proxyServer.Group(routes.FSProxy, func(group *ghttp.RouterGroup) {
		if loginRequired {
			group.Middleware(middleware.AuthMiddleware(jwtManager))
		}
		// 【关键】文件代理路由，现在包含响应修改逻辑
		u, err := url.Parse("http://" + backendAddr)
		if err != nil {
			log.Panicf("Failed to parse backend URL %s: %v", backendAddr, err)
		}
		proxy := httputil.NewSingleHostReverseProxy(u)
		proxy.Director = func(req *http.Request) {
			originalPath := req.URL.Path
			req.URL.Path = strings.TrimPrefix(originalPath, routes.FSProxy)
			req.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, routes.FSProxy)
			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
			req.Header.Set("X-Forwarded-Prefix", routes.FSProxy)
			req.Header.Set("X-Forwarded-Host", req.Host)
			req.Header.Set("X-Forwarded-Proto", "http")
			g.Log().Infof(req.Context(), "proxyServer called for path: %s", req.URL.Path)
		}

		// 【核心】注册 ModifyResponse 函数来注入内容
		proxy.ModifyResponse = createResponseModifier(userInjector, jwtManager)

		group.ALL("/*", func(r *ghttp.Request) {
			g.Log().Infof(r.Context(), "proxyServer called for path: %s", r.URL.Path)

			r.MakeBodyRepeatableRead(false)
			proxy.ServeHTTP(r.Response.Writer, r.Request)
		})
	})

	proxyServer.Group(routes.FSProxyAPI, func(group *ghttp.RouterGroup) {

		u, err := url.Parse("http://" + backendAddr)
		if err != nil {
			log.Panicf("Failed to parse backend URL for API %s: %v", backendAddr, err)
		}

		// 为API创建一个独立的代理实例
		apiProxy := httputil.NewSingleHostReverseProxy(u)
		// 【关键】重写路径：将 /p-api/... 替换为 /api/...
		apiProxy.Director = func(req *http.Request) {
			originalPath := req.URL.Path
			req.URL.Path = strings.TrimPrefix(originalPath, routes.FSProxyAPI)
			req.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, routes.FSProxyAPI)

			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
			req.Header.Set("X-Forwarded-Prefix", routes.FSProxyAPI)
			req.Header.Set("X-Forwarded-Host", req.Host)
			req.Header.Set("X-Forwarded-Proto", "http")
		}

		// 捕获所有 /p-api/... 的请求并转发
		group.ALL("/*", func(r *ghttp.Request) {
			r.MakeBodyRepeatableRead(false)
			apiProxy.ServeHTTP(r.Response.Writer, r.Request)
		})
	})

	// 使用 Hook 来注入内容
	proxyServer.BindHookHandler("/*", ghttp.HookAfterServe, func(r *ghttp.Request) {
		isLoggedIn := jwtManager.IsLoggedIn(r.Request.Header.Get("Authorization"))
		// 检查响应类型
		contentType := r.Response.Header().Get("Content-Type")
		// 只对HTML响应进行注入
		if contentType != "" && !strings.Contains(contentType, "text/html") {
			return
		}

		// 检查响应内容
		buffer := r.Response.Buffer()
		if buffer != nil {
			// log.Printf("[Hook] Response buffer length: %d", len(buffer))
		} else {
			// log.Printf("[Hook] Response buffer is nil")
			return
		}

		// 只对HTML响应进行注入
		if contentType != "" && !strings.Contains(contentType, "text/html") {
			return
		}

		content := string(buffer)

		// 检查是否已经包含我们的内容，避免重复注入
		if strings.Contains(content, `id="`+injector.InjectorID+`">`) &&
			!strings.Contains(content, `id="`+injector.InjectorID+`"></div>`) {
			return
		}

		// 生成工具栏HTML
		toolbarHTML := userInjector.GenerateFloatingToolbar(isLoggedIn)

		if toolbarHTML != "" {
			// 查找完整的自闭合div
			emptyDiv := `<div id="` + injector.InjectorID + `"></div>`
			if idx := strings.Index(content, emptyDiv); idx != -1 {
				// 直接替换整个自闭合div
				content = content[:idx] + `<div id="` + injector.InjectorID + `">` + toolbarHTML + `</div>` + content[idx+len(emptyDiv):]
			} else {
				// 如果没有找到注入点，尝试在</body>前插入
				bodyIndex := strings.LastIndex(strings.ToLower(content), "</body>")
				if bodyIndex != -1 {
					content = content[:bodyIndex] + toolbarHTML + "\n" + content[bodyIndex:]
				}
			}
		}

		// 【关键】使用 SetBuffer 更新响应内容
		r.Response.SetBuffer([]byte(content))
	})

	return proxyServer, proxyInstanceID
}

// handleLogin 处理登录逻辑
func handleLogin(r *ghttp.Request, jwtManager *auth.JWTManager, password, method string) {
	if r.URL.Path != routes.Login {
		r.Response.WriteStatus(http.StatusNotFound)
		return
	}
	if method == "GET" {
		// 检查是否已经登录
		token := auth.GetTokenFromCookie(r.Request)
		if token != "" {
			if claims, err := jwtManager.ValidateToken(token); err == nil {
				// 已登录，显示提示而不是重定向
				log.Printf("[handleLogin] User already logged in, session: %s", claims.SessionID)
				tmpl := template.Must(template.New("already_logged").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>Already Logged In</title>
    <style>
        body { font-family: sans-serif; text-align: center; padding: 2em; }
        .message { margin: 2em; padding: 1em; background: #f0f8ff; border-radius: 4px; }
        a { color: #007bff; margin: 0 1em; }
    </style>
</head>
<body>
<div id="` + injector.InjectorID + `"></div>
    <div class="message">
        <h2>You are already logged in</h2>
        <p>
            <a href="` + routes.FSProxy + `/">Go to Files</a>
            <a href="` + routes.OpenListProxy + `/sites">OpenList</a>
            <a href="` + routes.Logout + `">Logout</a>
        </p>
    </div>
</body>
</html>`))
				r.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
				tmpl.Execute(r.Response.Writer, nil)
				return
			} else {
				log.Printf("[handleLogin] Invalid token: %v", err)
				// 清除无效token
				auth.ClearAuthCookie(r.Response.Writer)
			}
		}

		// 保存原始请求URL
		redirectURL := r.Get("redirect_url").String()
		if redirectURL == "" {
			redirectURL = r.Referer()
		}

		// 验证并保存重定向URL
		var savedRedirectURL string
		if redirectURL != "" && !isLoginRelatedURL(redirectURL) {
			auth.SetRedirectCookie(r.Response.Writer, redirectURL)
			savedRedirectURL = redirectURL // 只保存有效的URL
		}

		// 显示登录页面
		tmpl, _ := template.New("login").Parse(auth.LoginPageTmpl)
		data := map[string]interface{}{
			"RedirectURL": savedRedirectURL, // 只传递有效的URL
		}
		r.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(r.Response.Writer, data)
		return
	}

	// POST 处理登录
	var req struct {
		Password    string `json:"password" form:"password"`
		RedirectURL string `json:"redirect_url" form:"redirect_url"`
	}

	if err := r.Parse(&req); err != nil {
		tmpl, _ := template.New("login").Parse(auth.LoginPageTmpl)
		data := map[string]interface{}{
			"Error":       "Invalid request",
			"RedirectURL": "", // 错误时不显示重定向信息
		}
		r.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(r.Response.Writer, data)
		return
	}

	// 验证密码
	if req.Password != password {
		tmpl, _ := template.New("login").Parse(auth.LoginPageTmpl)
		data := map[string]interface{}{
			"Error":       "Invalid password",
			"RedirectURL": "", // 错误时不显示重定向信息
		}
		r.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(r.Response.Writer, data)
		return
	}

	// 创建JWT token
	token, err := jwtManager.CreateToken()
	if err != nil {
		tmpl, _ := template.New("login").Parse(auth.LoginPageTmpl)
		data := map[string]interface{}{
			"Error":       "Failed to create token",
			"RedirectURL": "", // 错误时不显示重定向信息
		}
		r.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(r.Response.Writer, data)
		return
	}

	// 设置Cookie
	auth.SetAuthCookie(r.Response.Writer, token, 7*24*time.Hour)

	// 1. 优先使用表单中的重定向URL
	if req.RedirectURL != "" && !isLoginRelatedURL(req.RedirectURL) {
		auth.ClearRedirectCookie(r.Response.Writer)
		r.Response.RedirectTo(req.RedirectURL)
		return
	}

	// 2. 其次使用Cookie中的重定向URL
	if redirectURL := auth.GetRedirectCookie(r.Request); redirectURL != "" {
		auth.ClearRedirectCookie(r.Response.Writer)
		if !isLoginRelatedURL(redirectURL) {
			r.Response.RedirectTo(redirectURL)
			return
		}
	}

	// 3. 最后重定向
	r.Response.RedirectTo(routes.FSProxy + "/")
}

// isLoginRelatedURL 检查URL是否与登录相关
func isLoginRelatedURL(url string) bool {
	if url == "" {
		return false
	}

	// 检查是否包含登录相关的路径
	loginPaths := []string{
		routes.Login,
		routes.Logout,
	}

	for _, path := range loginPaths {
		if strings.Contains(url, path) {
			return true
		}
	}

	return false
}

// createResponseModifier 创建一个响应修改器，代理绕过了普通生命周期的注入器，需要传入普通路由的注入器合并注入
func createResponseModifier(injec *injector.UserInjector, jwtManager *auth.JWTManager) func(*http.Response) error {
	const contentInjectionPoint = `<div id="` + injector.InjectorID + `"></div>`
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
		currentPath := strings.TrimPrefix(resp.Request.URL.Path, routes.FSProxy)
		// 处理根路径的特殊情况
		if currentPath == "/" {
			currentPath = ""
		}

		isLoggedIn := jwtManager.IsLoggedIn(resp.Request.Header.Get("Authorization"))

		toolbarHTML := injec.GenerateFloatingToolbar(isLoggedIn)
		styleHTML := generateUserAssets(isLoggedIn, currentPath)

		if styleHTML == "" && toolbarHTML == "" {
			resp.Body = io.NopCloser(strings.NewReader(string(originalBody)))
			resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(originalBody)))
			return nil
		}

		newBody := string(originalBody)

		// 【合并注入】注入工具栏
		if toolbarHTML != "" {
			emptyDiv := `<div id="` + injector.InjectorID + `"></div>`
			if idx := strings.Index(newBody, emptyDiv); idx != -1 {
				newBody = newBody[:idx] + `<div id="` + injector.InjectorID + `">` + toolbarHTML + `</div>` + newBody[idx+len(emptyDiv):]
			}
		}

		// 注入样式+JS
		if styleHTML != "" {
			newBody = strings.ReplaceAll(newBody, headEndTag, styleHTML+headEndTag)
		}

		resp.Body = io.NopCloser(strings.NewReader(newBody))
		resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(newBody)))

		return nil
	}
}

// generateUserAssets 生成需要注入的表格内容
func generateUserAssets(isLoggedIn bool, currentPath string) (styleHTML string) {
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
		const currentPath = "` + template.JSEscapeString(currentPath) + `";
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
		  	let currentDirPath = currentPath;
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
				let currentDirPath = currentPath;
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

		return styleHTML
	}

	return ""
}
