//go:build android

package utils

/*
#cgo LDFLAGS: -ldl

#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <pthread.h>

typedef int (*run_func_t)(int argc, char **argv);
typedef void (*reset_func_t)(void);

static pthread_mutex_t g_mutex = PTHREAD_MUTEX_INITIALIZER;
static void *g_ffmpeg_handle = NULL;
static void *g_ffprobe_handle = NULL;

static int call_native_run(
    const char *lib_path,
    const char *run_sym,
    const char *reset_sym,
    int argc,
    char **argv,
    const char *stdout_file,
    const char *stderr_file
) {
    pthread_mutex_lock(&g_mutex);

    void *handle = dlopen(lib_path, RTLD_NOW);
    if (!handle) {
        pthread_mutex_unlock(&g_mutex);
        return -1;
    }

    reset_func_t reset_fn = (reset_func_t)dlsym(handle, reset_sym);
    if (reset_fn) reset_fn();

    run_func_t run_fn = (run_func_t)dlsym(handle, run_sym);
    if (!run_fn) {
        dlclose(handle);
        pthread_mutex_unlock(&g_mutex);
        return -2;
    }

    int saved_stdout = -1, saved_stderr = -1;

    if (stdout_file) {
        fflush(stdout);
        saved_stdout = dup(STDOUT_FILENO);
        int fd = open(stdout_file, O_WRONLY | O_CREAT | O_TRUNC, 0644);
        if (fd >= 0) {
            dup2(fd, STDOUT_FILENO);
            close(fd);
        }
    }
    if (stderr_file) {
        fflush(stderr);
        saved_stderr = dup(STDERR_FILENO);
        int fd = open(stderr_file, O_WRONLY | O_CREAT | O_TRUNC, 0644);
        if (fd >= 0) {
            dup2(fd, STDERR_FILENO);
            close(fd);
        }
    }

    int ret = run_fn(argc, argv);

    if (saved_stdout >= 0) {
        fflush(stdout);
        dup2(saved_stdout, STDOUT_FILENO);
        close(saved_stdout);
    }
    if (saved_stderr >= 0) {
        fflush(stderr);
        dup2(saved_stderr, STDERR_FILENO);
        close(saved_stderr);
    }

    dlclose(handle);
    pthread_mutex_unlock(&g_mutex);
    return ret;
}
*/
import "C"

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"
)

var (
	libDirOnce  sync.Once
	libDirCache string
)

func getLibDir() string {
	libDirOnce.Do(func() {
		libDirCache = os.Getenv("ENCV_LIB_DIR")
	})
	return libDirCache
}

type nativeResult struct {
	exitCode int
	stdout   string
	stderr   string
}

func callFFmpegNative(args []string) (*nativeResult, error) {
	libDir := getLibDir()
	if libDir == "" {
		return nil, fmt.Errorf("ENCV_LIB_DIR not set")
	}

	libPath := filepath.Join(libDir, "libffmpeg.so")

	cLibPath := C.CString(libPath)
	defer C.free(unsafe.Pointer(cLibPath))

	cRunSym := C.CString("ffmpeg_run")
	defer C.free(unsafe.Pointer(cRunSym))

	cResetSym := C.CString("ffmpeg_reset")
	defer C.free(unsafe.Pointer(cResetSym))

	argc := C.int(len(args))
	argv := make([]*C.char, len(args)+1)
	for i, arg := range args {
		argv[i] = C.CString(arg)
		defer C.free(unsafe.Pointer(argv[i]))
	}
	argv[len(args)] = nil

	stderrFile, err := os.CreateTemp("", "ffmpeg_stderr_*.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	stderrPath := stderrFile.Name()
	stderrFile.Close()
	defer os.Remove(stderrPath)

	cStderrPath := C.CString(stderrPath)
	defer C.free(unsafe.Pointer(cStderrPath))

	ret := C.call_native_run(cLibPath, cRunSym, cResetSym, argc, &argv[0], nil, cStderrPath)

	stderrData, _ := os.ReadFile(stderrPath)

	result := &nativeResult{
		exitCode: int(ret),
		stderr:   string(stderrData),
	}

	if ret == -1 {
		return result, fmt.Errorf("failed to load %s", libPath)
	}
	if ret == -2 {
		return result, fmt.Errorf("ffmpeg_run symbol not found in %s", libPath)
	}

	return result, nil
}

func callFFprobeNative(args []string) (*nativeResult, error) {
	libDir := getLibDir()
	if libDir == "" {
		return nil, fmt.Errorf("ENCV_LIB_DIR not set")
	}

	libPath := filepath.Join(libDir, "libffprobe.so")

	cLibPath := C.CString(libPath)
	defer C.free(unsafe.Pointer(cLibPath))

	cRunSym := C.CString("ffprobe_run")
	defer C.free(unsafe.Pointer(cRunSym))

	cResetSym := C.CString("ffprobe_reset")
	defer C.free(unsafe.Pointer(cResetSym))

	argc := C.int(len(args))
	argv := make([]*C.char, len(args)+1)
	for i, arg := range args {
		argv[i] = C.CString(arg)
		defer C.free(unsafe.Pointer(argv[i]))
	}
	argv[len(args)] = nil

	stdoutFile, err := os.CreateTemp("", "ffprobe_stdout_*.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	stdoutPath := stdoutFile.Name()
	stdoutFile.Close()
	defer os.Remove(stdoutPath)

	cStdoutPath := C.CString(stdoutPath)
	defer C.free(unsafe.Pointer(cStdoutPath))

	ret := C.call_native_run(cLibPath, cRunSym, cResetSym, argc, &argv[0], cStdoutPath, nil)

	stdoutData, _ := os.ReadFile(stdoutPath)

	result := &nativeResult{
		exitCode: int(ret),
		stdout:   string(stdoutData),
	}

	if ret == -1 {
		return result, fmt.Errorf("failed to load %s", libPath)
	}
	if ret == -2 {
		return result, fmt.Errorf("ffprobe_run symbol not found in %s", libPath)
	}

	return result, nil
}
