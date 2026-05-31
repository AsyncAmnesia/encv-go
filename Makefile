.PHONY: encv copy-files build-all build-artifacts run clean dev-backend dev-mobile

OUTPUT_DIR ?= dist

# 清理编译产物
clean:
	@echo "Cleaning up..."
	rm -rf $(OUTPUT_DIR)/

# 编译 encv
encv:
	@echo "Building encv..."
	go build -o $(OUTPUT_DIR)/encv ./cmd/encv

# 复制配置和文档文件
copy-files:
	@echo "Copying necessary files..."
	@mkdir -p $(OUTPUT_DIR)
	@cp config.user.json $(OUTPUT_DIR)/
	@cp README.md $(OUTPUT_DIR)/

# 编译所有程序并复制文件
build-all: encv copy-files
	@echo "All targets and files built successfully in ./$(OUTPUT_DIR)/"

# 启动后端（桌面端模式，server.dir 使用 config 中的原始值）
dev-backend:
	@echo "Starting backend (desktop mode)..."
	go run ./cmd/encv start

# 启动后端（移动端预览模式，自动将 mobile.server_dir 覆盖到 server.dir）
# 等价于真机 Android 端 EncvGoService.kt 的行为
dev-mobile:
	@echo "Starting backend (mobile preview mode)..."
	ENCV_MOBILE=1 go run ./cmd/encv start
