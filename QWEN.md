## Qwen Added Memories
- 用户希望代码修改完成后自动进行编译测试
- 编译GUI版本命令: cd /d D:\LuminaFlow && set PATH=C:\Program Files\Go\bin;C:\msys64\mingw64\bin;%PATH% && set GOPROXY=https://goproxy.cn,direct && set CGO_ENABLED=1 && go build -tags gui -ldflags "-H windowsgui" -o LuminaFlow_gui.exe .
