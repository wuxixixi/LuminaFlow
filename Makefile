# LuminaFlow 构建脚本

# 版本号
VERSION = 1.2.0

# Windows 构建 (CLI)
build-windows:
	go build -o LuminaFlow.exe .

# Windows GUI 构建
build-windows-gui:
	go build -tags gui -ldflags -H=windowsgui -o LuminaFlow_gui.exe .

# macOS 构建 (需要 macOS 或交叉编译工具链)
build-macos:
	GOOS=darwin GOARCH=amd64 go build -o LuminaFlow .

# 清理构建产物
clean:
	rm -f LuminaFlow.exe
	rm -f LuminaFlow
	rmdir /s /q output 2>nul || true

# 下载依赖
deps:
	go mod download
	go mod tidy

# 格式化代码
fmt:
	go fmt ./...

# 运行静态分析
vet:
	go vet ./...

# 查看依赖
list-deps:
	go list -m all

# 打包 (需要 fyne 工具链)
package-windows:
	fyne package -os windows -name LuminaFlow

package-macos:
	fyne package -os darwin -name LuminaFlow

# 交叉编译 macOS (需要 fyne-cross)
cross-macos:
	fyne-cross darwin -arch amd64,arm64 -name LuminaFlow

# 交叉编译 Windows (需要 fyne-cross)
cross-windows:
	fyne-cross windows -arch amd64 -name LuminaFlow
