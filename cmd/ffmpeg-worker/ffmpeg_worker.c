// cmd/ffmpeg-worker/ffmpeg_worker.c
// 完整可编译，无任何外部依赖，兼容 Go 1.25+ 所有问题
#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <dlfcn.h>
#include <signal.h>
#include <time.h>
#include <sys/time.h>
#include <fcntl.h>

// ========== 配置 ==========
#define MAX_ARGS        128
#define MAX_ARG_LEN     4096
#define MAX_JSON_LEN    65536
#define MAX_LIB_PATH    4096

// ========== FFmpeg 函数指针 ==========
typedef int (*run_fn_t)(int, char**);
typedef void (*reset_fn_t)(void);

// ========== 全局状态 ==========
static volatile int g_timeout_triggered = 0;
static char g_lib_dir[MAX_LIB_PATH] = {0};
static void* g_ffmpeg_handle = NULL;

// ========== 超时处理（硬超时） ==========
static void timeout_sig_handler(int sig) {
    g_timeout_triggered = 1;
    fprintf(stderr, "[ffmpeg-worker] timeout exceeded; self-exit\n");
    _exit(124);
}

static void setup_timeout(int timeout_ms) {
    if (timeout_ms <= 0) return;
    struct sigaction sa = {.sa_handler = timeout_sig_handler};
    sigaction(SIGALRM, &sa, NULL);
    ualarm((unsigned int)timeout_ms * 1000, 0);
}

// ========== 极简 JSON 解析（无依赖，专门针对我们的协议） ==========
// 协议格式：{"args":["-i","a.mp4"], "lib_dir":"/path", "timeout_ms":30000}
static int json_find_string(const char* json, const char* key, char* out, int out_len) {
    char search[256];
    snprintf(search, sizeof(search), "\"%s\":\"", key);
    const char* p = strstr(json, search);
    if (!p) return -1;
    p += strlen(search);
    const char* end = strchr(p, '"');
    if (!end) return -1;
    int len = (int)(end - p);
    if (len >= out_len) len = out_len - 1;
    memcpy(out, p, (size_t)len);
    out[len] = 0;
    return 0;
}

static int json_find_int(const char* json, const char* key, int* out) {
    char search[256];
    snprintf(search, sizeof(search), "\"%s\":", key);
    const char* p = strstr(json, search);
    if (!p) return -1;
    p += strlen(search);
    *out = atoi(p);
    return 0;
}

static int json_parse_args(const char* json, char*** out_args, int* out_argc) {
    const char* p = strstr(json, "\"args\":");
    if (!p) return -1;
    p = strchr(p, '[');
    if (!p) return -1;
    p++;
    
    char** args = calloc(MAX_ARGS, sizeof(char*));
    int argc = 0;
    args[argc++] = strdup("ffmpeg");  // argv[0]
    
    while (*p && *p != ']' && argc < MAX_ARGS) {
        while (*p == ' ' || *p == ',' || *p == '\n' || *p == '\t') p++;
        if (*p == ']') break;
        if (*p != '"') { p++; continue; }
        p++;
        const char* start = p;
        while (*p && *p != '"') {
            if (*p == '\\') p++;
            p++;
        }
        int len = (int)(p - start);
        char* arg = malloc((size_t)len + 1);
        memcpy(arg, start, (size_t)len);
        arg[len] = 0;
        args[argc++] = arg;
        if (*p == '"') p++;
    }
    
    *out_args = args;
    *out_argc = argc;
    return 0;
}

// ========== 重定向 stdout/stderr 到临时文件 ==========
static int redirect_output(const char* stderr_file) {
    if (stderr_file) {
        fflush(stderr);
        int saved_stderr = dup(STDERR_FILENO);
        int fd = open(stderr_file, O_WRONLY | O_CREAT | O_TRUNC, 0644);
        if (fd >= 0) {
            dup2(fd, STDERR_FILENO);
            close(fd);
        }
        return saved_stderr;
    }
    return -1;
}

static void restore_stderr(int saved) {
    if (saved >= 0) {
        fflush(stderr);
        dup2(saved, STDERR_FILENO);
        close(saved);
    }
}

// ========== 主函数 ==========
int main(void) {
    // 1. 完整读取 stdin（JSON 请求）
    char json_buf[MAX_JSON_LEN];
    ssize_t total = 0;
    while (1) {
        ssize_t n = read(STDIN_FILENO, json_buf + total, MAX_JSON_LEN - total - 1);
        if (n <= 0) break;
        total += n;
    }
    if (total <= 0) {
        printf("{\"error\":\"empty stdin\",\"exit_code\":-1}\n");
        return 1;
    }
    json_buf[total] = 0;
    
    // 2. 解析 JSON
    char** args = NULL;
    int argc = 0;
    int timeout_ms = 0;
    
    if (json_parse_args(json_buf, &args, &argc) != 0) {
        printf("{\"error\":\"parse args failed\",\"exit_code\":-1}\n");
        return 1;
    }
    
    json_find_string(json_buf, "lib_dir", g_lib_dir, sizeof(g_lib_dir));
    json_find_int(json_buf, "timeout_ms", &timeout_ms);
    
    // 3. 设置超时
    setup_timeout(timeout_ms);
    
    // 4. 计时开始
    struct timeval start, end;
    gettimeofday(&start, NULL);
    
    // 5. dlopen libffmpeg.so
    char ffmpeg_path[MAX_LIB_PATH];
    snprintf(ffmpeg_path, sizeof(ffmpeg_path), "%s/libffmpeg.so", g_lib_dir);
    
    dlerror();
    g_ffmpeg_handle = dlopen(ffmpeg_path, RTLD_NOW | RTLD_LOCAL);
    if (!g_ffmpeg_handle) {
        const char* err = dlerror();
        printf("{\"error\":\"[ENGINE_LOAD_FAILED] dlopen %s: %s\",\"exit_code\":-1}\n", ffmpeg_path, err ? err : "unknown");
        return 1;
    }
    
    // 6. 查找符号
    reset_fn_t ffmpeg_reset = (reset_fn_t)dlsym(g_ffmpeg_handle, "ffmpeg_reset");
    if (ffmpeg_reset) ffmpeg_reset();
    
    run_fn_t ffmpeg_run = (run_fn_t)dlsym(g_ffmpeg_handle, "ffmpeg_run");
    if (!ffmpeg_run) {
        const char* err = dlerror();
        printf("{\"error\":\"[ENGINE_SYMBOL_MISSING] ffmpeg_run: %s\",\"exit_code\":-2}\n", err ? err : "unknown");
        return 1;
    }
    
    // 7. 创建 stderr 临时文件 + 重定向
    char stderr_path[] = "/tmp/ffmpeg_stderr_XXXXXX";
    int stderr_fd = mkstemp(stderr_path);
    close(stderr_fd);
    
    int saved_stderr = redirect_output(stderr_path);
    
    // 8. 执行 FFmpeg（核心调用）
    int exit_code = ffmpeg_run(argc, args);
    
    // 9. 恢复 stderr，读取输出
    restore_stderr(saved_stderr);
    
    char stderr_buf[65536] = {0};
    FILE* f = fopen(stderr_path, "r");
    if (f) {
        fread(stderr_buf, 1, sizeof(stderr_buf)-1, f);
        fclose(f);
    }
    unlink(stderr_path);
    
    // 10. 计算耗时
    gettimeofday(&end, NULL);
    long duration_ms = (end.tv_sec - start.tv_sec) * 1000L + 
                       (end.tv_usec - start.tv_usec) / 1000L;
    
    // 11. 输出 JSON 响应（与 Go worker 完全兼容的格式）
    printf("{\"exit_code\":%d,\"duration_ms\":%ld,\"stderr\":\"", exit_code, duration_ms);
    
    // JSON 转义 stderr
    for (char* p = stderr_buf; *p; p++) {
        if (*p == '"' || *p == '\\') putchar('\\');
        if (*p >= ' ' && *p < 127) putchar(*p);
    }
    
    printf("\"}\n");
    fflush(stdout);
    
    // 12. 清理
    for (int i = 0; i < argc; i++) free(args[i]);
    free(args);
    if (g_ffmpeg_handle) dlclose(g_ffmpeg_handle);
    
    return exit_code;
}
