.PHONY: encv encv-proxy copy-files build-all build-artifacts run clean

OUTPUT_DIR ?= dist

# 清理编译产物
clean:
	@echo "Cleaning up..."
	rm -rf $(OUTPUT_DIR)/
# 编译 encv
encv:
	@echo "Building encv..."
	go build -o $(OUTPUT_DIR)/encv ./cmd/encv

# 编译 encv-proxy
encv-proxy:
	@echo "Building encv-proxy..."
	go build -o $(OUTPUT_DIR)/encv-proxy ./cmd/encv-proxy

# 复制配置和文档文件
copy-files:
	@echo "Copying necessary files..."
	@mkdir -p $(OUTPUT_DIR)
	@cp config.user.json $(OUTPUT_DIR)/
	@cp README.md $(OUTPUT_DIR)/

# 编译所有程序并复制文件
build-all: encv encv-proxy copy-files
	@echo "All targets and files built successfully in ./$(OUTPUT_DIR)/"
