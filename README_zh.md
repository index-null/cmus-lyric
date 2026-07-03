<p align="center">
  <img src="./images/banner.png" alt="cmus-lyric banner" width="600" />
</p>

<p align="center">
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go"></a>
  <a href="https://github.com/charmbracelet/bubbletea"><img src="https://img.shields.io/badge/Bubble_Tea-TUI-ff69b4?style=flat" alt="Bubble Tea"></a>
  <a href="https://github.com/charmbracelet/lipgloss"><img src="https://img.shields.io/badge/Lipgloss-Styling-7D56F4?style=flat" alt="Lipgloss"></a>
  <a href="https://github.com/charmbracelet/bubbles"><img src="https://img.shields.io/badge/Bubbles-Components-AD8EE6?style=flat" alt="Bubbles"></a>
  <a href="https://lrclib.net"><img src="https://img.shields.io/badge/LRCLIB-Lyrics_API-4CAF50?style=flat" alt="LRCLIB"></a>
  <a href="https://github.com/index-null/cmus-lyric/releases/latest"><img src="https://img.shields.io/github/v/release/index-null/cmus-lyric?style=flat&color=blue" alt="Release"></a>
  <a href="https://github.com/index-null/cmus-lyric/blob/master/LICENSE"><img src="https://img.shields.io/github/license/index-null/cmus-lyric?style=flat" alt="License"></a>
</p>

# cmus-lyric

[English](README.md) | 中文

基于 [Bubble Tea](https://github.com/charmbracelet/bubbletea) 构建的终端歌词同步查看器，专为 [cmus](https://cmus.github.io/) 设计。

> 灵感来自 [pekrockstar/cmus-lyric](https://github.com/pekrockstar/cmus-lyric)，使用现代 Go 技术栈从零重写。

<p align="center">
  <img src="./images/demo.png" alt="cmus-lyric demo" width="600" />
</p>

## 简介

`cmus-lyric` 通过 Unix socket（自动回退到 `cmus-remote`）连接正在运行的 cmus 实例，读取当前播放曲目，并在终端中实时显示时间同步歌词。歌词来源包括音频内嵌标签、本地 `.lrc` 文件、磁盘缓存和在线 API，所有网络获取均异步执行，不阻塞 UI。

**功能特性：**

- 实时歌词滚动高亮
- 自动从 LRCLIB 和网易云音乐获取歌词（非阻塞）
- 翻译歌词支持（`.t.lrc` / `.t.lyric` 对照显示）
- 从音频文件提取内嵌歌词（ID3/Vorbis Comment）
- 专辑封面显示（`lyrics cover`）
- 歌词缓存系统（路径由 `os.UserCacheDir()` 确定，如 macOS 通常为 `~/Library/Caches/cmus-lyric/`，Linux 通常为 `~/.cache/cmus-lyric/`），支持离线和只读目录
- 时长容差匹配（±2秒），提升歌词匹配准确度
- Unix socket IPC，低开销 cmus 通信
- GBK/UTF-8 编码自动检测
- 进度条与播放状态
- 调试模式（`d` 键）查看曲目元数据和歌词来源
- 简洁无干扰的界面

## 安装

### Homebrew（macOS / Linux）

```bash
brew install index-null/tap/lyrics
```

### 一键脚本

```bash
curl -fsSL https://raw.githubusercontent.com/index-null/cmus-lyric/master/install.sh | bash
```

自定义安装目录：

```bash
INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/index-null/cmus-lyric/master/install.sh | bash
```

### Go

```bash
go install github.com/index-null/cmus-lyric/cmd/lyrics@latest
```

### 手动下载

从 [Releases](https://github.com/index-null/cmus-lyric/releases/latest) 页面下载对应平台的压缩包：

```bash
tar xzf cmus-lyric_*_darwin_arm64.tar.gz
sudo install -m 755 lyrics /usr/local/bin/lyrics
```

### 从源码构建

```bash
git clone https://github.com/index-null/cmus-lyric.git
cd cmus-lyric
task install   # 或: go build -o lyrics ./cmd/lyrics && sudo mv lyrics /usr/local/bin/
```

## 前置条件

- [cmus](https://cmus.github.io/) 音乐播放器（需正在运行）

## 使用

启动 cmus 并播放歌曲，然后在另一个终端中运行：

```bash
lyrics
```

| 按键         | 功能 |
| ------------ | ---- |
| `q` `Ctrl+C` | 退出 |
| `?`          | 帮助 |
| `d`          | 调试 |
| `r`          | 重新获取歌词 |

### 歌词解析逻辑

1. 从音频文件提取内嵌歌词（ID3 USLT / Vorbis Comment）
2. 在音频文件同目录下查找 `<文件名>.lrc` 或 `<文件名>.lyric`
3. 若存在 `.t.lrc` / `.t.lyric` 文件，翻译歌词会显示在每行下方
4. 查找本地缓存（路径由 `os.UserCacheDir()` 确定，如 macOS 通常为 `~/Library/Caches/cmus-lyric/`，Linux 通常为 `~/.cache/cmus-lyric/`）
5. 若均未找到，依次从 LRCLIB、网易云音乐获取，保存为 `.lrc` 并缓存
   - **时长容差**：匹配曲目时允许 ±2 秒的时长偏差

### 专辑封面

显示当前播放曲目的专辑封面：

```bash
lyrics cover
```

封面获取来源：
1. 音频文件内嵌专辑封面
2. 本地缓存（路径由 `os.UserCacheDir()` 确定，如 macOS 通常为 `~/Library/Caches/cmus-lyric/`，Linux 通常为 `~/.cache/cmus-lyric/`）
3. 网易云音乐 API（自动保存到缓存）

## 项目结构

```
cmus-lyric/
├── cmd/lyrics/           # 应用入口
├── internal/
│   ├── cmus/             # cmus IPC（Unix socket + exec 回退）
│   ├── cover/            # 专辑封面显示
│   ├── lyric/            # 歌词加载、解析、获取、缓存
│   └── player/           # Bubble Tea 模型、视图、样式
├── .github/workflows/    # CI/CD（tag 触发自动发布）
├── Taskfile.yml          # 构建任务
├── .golangci.yml         # Linter 配置（v2）
├── lefthook.yml          # Git hooks（fmt + lint + build + test）
├── .goreleaser.yaml      # 发布配置
├── install.sh            # 一键安装脚本
└── go.mod
```

## 开发

```bash
task build          # 构建到 bin/
task run            # 构建并运行
task lint           # 运行 golangci-lint
task test           # 运行测试
task check          # 完整质量检查（tidy + lint + test）
```

> [!NOTE]
> Lint 需要安装 [golangci-lint](https://golangci-lint.run/)：`brew install golangci-lint`

## 发布

通过 GitHub Actions 全自动发布：

```bash
git tag v0.1.0
git push origin v0.1.0
```

自动完成：

1. 构建 linux/darwin x amd64/arm64 四平台二进制
2. 创建 GitHub Release 并附带校验和
3. 更新 Homebrew tap

> [!TIP]
> 本地预览发布：`goreleaser release --snapshot --clean`
