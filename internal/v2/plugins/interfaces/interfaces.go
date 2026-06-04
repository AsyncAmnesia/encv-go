package interfaces

import (
	"io"

	"github.com/Soltus/encv-go/internal/v2/types"
)

// MetadataExtractor 定义了如何从文件中提取元数据的策略
type MetadataExtractor interface {
	// ExtractMetadata 从给定的文件路径提取元数据
	ExtractMetadata(inputPath string) (types.Index, error)
}

// ContentPreprocessor 定义了如何预处理文件内容的策略
// 例如，视频需要 FFmpeg 转码，而图片或文档可能不需要任何处理
type ContentPreprocessor interface {
	// Preprocess 接收原始文件路径，返回一个处理后的内容读取器
	// 调用者负责关闭返回的 io.ReadCloser
	Preprocess(inputPath string) (io.ReadCloser, error)
}

// VerifyWarning 表示验证过程中产生的非致命警告信息
type VerifyWarning struct {
	CheckName string `json:"check_name"`
	Message   string `json:"message"`
	Severity  string `json:"severity"`
}

// VerifyOptions 定义 Verify 方法的可选行为参数
type VerifyOptions struct {
	SkipSizeCheck   bool // 跳过精确文件大小比对（用于重编码/转码模式，此时原始文件与解密文件大小天然不同）
	SkipStructCheck bool // 跳过结构完整性检查（用于重编码输出，此时 MP4 结构可能完全不同）
	SkipDeepCheck   bool // 跳过深度完整性检查 L4（用于 PostEncryptProcessor，v4 容器加密后 MP4 结构必然改变）
	CollectWarnings bool // 收集 warnings 而非忽略（默认为 false，warnings 被丢弃）
}

// ContentVerifier 定义了插件校验解密内容完整性的能力。
// 这是一个可选接口，插件应根据自身特性（如是否支持随机访问、文件大小）来实现。
type ContentVerifier interface {
	// Verify 校验解密后的文件与原始文件是否一致。
	// originalPath: 原始输入文件路径。
	// decryptedPath: 经过加密再解密后的文件路径（通常位于临时目录）。
	// opts: 可选的验证选项，不传时使用默认严格模式。
	// 返回 error 表示校验失败，返回的 warnings 列表包含非致命警告信息（仅在 CollectWarnings=true 时有效）。
	Verify(originalPath, decryptedPath string, opts ...*VerifyOptions) (error, []*VerifyWarning)
}

// FragmentBuilder 定义了自定义逻辑分片策略的接口（如视频 GOP 对齐）
type FragmentBuilder interface {
	// BuildFragments 根据逻辑文件大小生成分片元数据
	BuildFragments(logicalFileSize int64) ([]types.Fragment, error)
}

// PasswordStrategy 声明插件的密码使用策略
type PasswordStrategy string

const (
	PasswordGlobal      PasswordStrategy = "global"      // 使用全局密码（video 等大多数插件）
	PasswordIndependent PasswordStrategy = "independent" // 使用插件独立密码，不用全局密码（alist_encrypt）
	PasswordNone        PasswordStrategy = "none"        // 不需要密码
)

// TaskOptions 返回该插件在创建加解密任务时需要的选项声明
// 前端根据此声明动态渲染表单字段，无需硬编码插件特定逻辑
type TaskOptions struct {
	PasswordStrategy     PasswordStrategy `json:"passwordStrategy"`
	SupportVersionSelect bool             `json:"supportVersionSelect"`
	SupportedVersions    []int            `json:"supportedVersions,omitempty"`
	DefaultVersion       int              `json:"defaultVersion"`
	ExtraFields          []TaskField      `json:"extraFields,omitempty"`
}

// TaskField 声明任务创建时的额外输入字段
type TaskField struct {
	Key          string            `json:"key"`
	Label        string            `json:"label"`
	Type         string            `json:"type"`
	Required     bool              `json:"required"`
	DefaultValue string            `json:"defaultValue"`
	Help         string            `json:"help"`
	Options      []string          `json:"options,omitempty"`
	OptionLabels map[string]string `json:"optionLabels,omitempty"`
	Condition    string            `json:"condition,omitempty"`
}

// TaskPasswordResolver 定义插件自定义主密码解析能力
// 插件根据 ExtraFields 和策略返回主密码（L0 或 L1）
// L2 二级密码不在此接口处理，由 TaskManager 单独传递
type TaskPasswordResolver interface {
	ResolveTaskPassword(taskPassword string, extraFields map[string]string) string
}

// TaskExtraFieldsSetter 定义插件接收任务级别 ExtraFields 的能力
// 声明式架构：插件通过 GetTaskOptions() 声明 ExtraFields，
// 前端渲染对应输入，后端在执行前通过此接口将用户输入注入插件实例
type TaskExtraFieldsSetter interface {
	SetTaskExtraFields(fields map[string]string)
}

type TaskStateResetter interface {
	ResetTaskState()
}
