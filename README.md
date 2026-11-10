# Hoard

Hoard 是一个本地优先的个人内容库，帮助你整理、浏览和回看收藏的内容。

当前版本支持抖音和哔哩哔哩账号登录、内容同步、多账号隔离、网格浏览、搜索与筛选、应用内播放，以及探索页的作品星图。

Hoard 是独立的社区项目，与任何内容平台没有官方关联。使用时请遵守相关平台的服务条款、适用法律和内容授权要求。

## 当前功能

- 登录并保存多个抖音、哔哩哔哩账号的独立会话。
- 同步并浏览喜欢、收藏、稍后再看和音乐。
- 使用网格、搜索、类型和时间筛选浏览内容。
- 在应用内查看和播放已同步内容。
- 在探索页以星图浏览作品及其内容关联，并按发布时间筛选。
- 同步过程中显示进度，并在本地保存已同步数据。

项目仍在持续开发中，部分同步能力和 Linux 桌面体验尚未完成完整的实机验证。发布前请阅读 [开源发布检查清单](OPEN_SOURCE_CHECKLIST.md)。

## 开始使用

### 环境要求

- Go 版本见 [`go.mod`](go.mod)。
- Bun。
- macOS、Windows 或 Linux 桌面环境。
- Windows 需要 WebView2 运行环境。
- Linux 需要 GTK 3、WebKitGTK 4.1 和 GStreamer 运行库。

### 开发运行

在项目根目录执行：

```sh
bun install --frozen-lockfile
bun run generate
bun run dev
```

产品官网位于 `website/`，使用 Next.js，与桌面应用共用仓库和 Bun 锁文件。启动官网：

```sh
bun run dev:site
```

打开 http://127.0.0.1:3000。静态构建与部署说明见 [website/README.md](website/README.md)。

只启动前端开发服务器：

```sh
bun run dev:web
```

### 检查与构建

```sh
bun run generate
bun run typecheck
go test ./...
go vet ./...
bun run build:web
```

PR 和 `main` 更新会触发 `Checks`，执行格式、测试、类型与前端构建检查。推送 `v*` Tag 会另行触发 `Release`，生成各平台安装包和 GitHub Release 草稿。

跨平台发布构建目前在 macOS 上执行：

```sh
bun run build
```

该命令将可运行的发布文件整理到 `release/`。`release/` 和 `build/` 已被 Git 忽略，不应提交到仓库。

## 发布文件

macOS 构建会生成以下目标：

- `Hoard-macOS-arm64.app`：Apple Silicon Mac 应用。
- `Hoard-Windows-x64.exe`：Windows x64 程序，需要 WebView2。
- `Hoard-Linux-x64`：Linux x86-64 程序。
- `Hoard-Linux-arm64`：Linux ARM64 程序。

macOS 的 `.app` 是系统标准应用包，分发时需要保留整个包目录。Windows 需要安装 WebView2 运行环境。

### Linux

Linux 使用 GTK 3、WebKitGTK 4.1 和 GStreamer，建议使用 Debian 12 或 Ubuntu 24.04 及更新版本。构建 Linux 目标：

```sh
bun install --frozen-lockfile
bun run build:linux
```

产物位于 `build/linux-amd64/` 和 `build/linux-arm64/`，其中包含可执行文件、`.deb`、`.tar.gz` 和安装脚本。Debian / Ubuntu 可以在产物目录安装：

```sh
sudo apt install ./hoard_0.1.0_amd64.deb
hoard
```

ARM64 机器将文件名中的 `amd64` 改为 `arm64`。也可以先安装运行依赖，再直接运行对应架构的可执行文件：

```sh
sudo apt install libwebkit2gtk-4.1-0 gstreamer1.0-libav gstreamer1.0-plugins-good gstreamer1.0-plugins-bad
chmod +x Hoard-Linux-x64
./Hoard-Linux-x64
```

使用构建目录中的压缩包安装到用户目录：

```sh
sh install.sh hoard-0.1.0-linux-amd64.tar.gz
~/.local/bin/hoard
```

当前已验证两种 Linux 架构的交叉构建和安装包结构；桌面上的登录、同步、音视频播放和关窗退出仍需实机验证。

## 数据与隐私

Hoard 面向个人使用，数据默认保存在本机应用数据目录中，包括本地数据库、缓存和按账号隔离的登录会话。

- 数据库包含账号信息和同步内容，登录会话由系统 WebView 保存。分享诊断材料前，请先脱敏。
- 请不要把真实账号数据、日志、数据库文件或 `.env` 文件提交到 Git。
- 同步和播放需要访问相应内容来源的网络服务。
- 删除账号会清理该账号的本地会话和关联数据；操作前请确认你不再需要这些本地数据。

## 贡献

欢迎通过 Issue 反馈问题、提出建议或提交改进。开始前请阅读 [CONTRIBUTING.md](CONTRIBUTING.md)。

这个项目受益于开源社区，也希望把成果继续分享出来。如果 Hoard 对你有所帮助，欢迎反馈问题、贡献改进，或公开你基于它所做的工作，让更多人从中受益。

## 许可证

Hoard 采用 [MIT License](LICENSE)。你可以自由使用、复制、修改、合并、发布、分发、再许可和销售本软件，但需要在副本或实质性部分中保留版权声明和许可证声明。

第三方依赖、图标和其他资源遵循各自的许可证。当前图标来源记录见 [`web/LICENSES.md`](web/LICENSES.md)，依赖的许可条款请参阅各依赖随附的许可证。
