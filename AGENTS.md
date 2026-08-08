# AGENTS.md — Claude Identity Injector v2

## 项目路径

```
C:\Users\Administrator\Desktop\claude-identity-injector-v2
```

## CPA 安装

```
CPA 根目录:    C:\Users\Administrator\Desktop\EasyCLIProxyAPI-v0.2.12-Windows-amd64
CPA 核心:      C:\Users\Administrator\Desktop\EasyCLIProxyAPI-v0.2.12-Windows-amd64\cpa-core
可执行文件:     ...\cpa-core\cli-proxy-api.exe
备份:          ...\cpa-core\cli-proxy-api.exe.bak-7.2.115
版本:          7.2.115 (core-version.txt)
插件目录:       ...\cpa-core\plugins\windows\amd64\
配置文件:       ...\cpa-core\config.yaml
EasyCLI 配置:  C:\Users\Administrator\Desktop\EasyCLIProxyAPI-v0.2.12-Windows-amd64\config.toml
```

## 管理 API

```
Base URL:   http://localhost:8317
管理密钥:    adkinsm123
鉴权方式:    X-Management-Key: adkinsm123  或   Authorization: Bearer adkinsm123

插件状态 HTML:  GET  /v0/resource/plugins/claude-identity-injector_v2/status
插件状态 JSON:  GET  /v0/resource/plugins/claude-identity-injector_v2/status.json
更新配置:       PATCH /v0/management/plugins/claude-identity-injector_v2/config
启停插件:       PATCH /v0/management/plugins/claude-identity-injector_v2/enabled
```

## 插件构建

```powershell
$env:CGO_ENABLED = "1"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CC = "C:\Users\Administrator\zig\zig-x86_64-windows-0.14.1\zig.exe cc"
go build -buildmode=c-shared -o claude-identity-injector_v2.dll .
```

| 工具 | 路径 |
|---|---|
| Go | 1.26.5 (系统 PATH) |
| Zig | `C:\Users\Administrator\zig\zig-x86_64-windows-0.14.1\zig.exe` |
| 唯一依赖 | `gopkg.in/yaml.v3` |

## 部署

```powershell
Copy-Item claude-identity-injector_v2.dll ...\plugins\windows\amd64\ -Force
Stop-Process -Name cli-proxy-api -Force
Start-Process -FilePath ...\cpa-core\cli-proxy-api.exe -WorkingDirectory ...\cpa-core
```

## Git

```
Remote:  git@github.com:Adkimsm/claude-identity-injector.git
GPG 密钥: E99E75D3D7E5C875FCF87EEEF6ACC9EF886E14F2
GPG 邮箱: adkinsm9277@gmail.com
```

提交风格：Linux kernel 风格（`<subsystem>: <summary>` + body + `Signed-off-by: Adkimsm <adkinsm9277@gmail.com>`）。

## 管理页面密钥解密

Management Center 的 localStorage 使用 `enc::v1::` 混淆：

```
key = SHA-256? 实际: XOR 密钥 = "cli-proxy-api-webui::secure-storage|{host}|{userAgent}"
```

页面 JS 读取 `localStorage['cli-proxy-auth']` 解密得到管理密钥。无密钥则降级只读。

## 相关文档

| 文档 | 链接 |
|---|---|
| CPA 用户手册 | https://help.router-for.me/cn/ |
| CPA 管理 API 文档 | https://help.router-for.me/cn/management/api |
| CPAMC (管理中心) | https://github.com/router-for-me/Cli-Proxy-API-Management-Center |
| CPA Usage Keeper | https://github.com/Willxup/cpa-usage-keeper |
| CPA-Manager-Plus | https://github.com/seakee/CPA-Manager-Plus |
| SDK 使用文档 | `docs/sdk-usage_CN.md` (CPA 目录内) |