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
- **四源并行**抓取歌词：LRCLIB、网易云音乐、酷狗音乐、lyrics.ovh（非阻塞）
- 歌词选择器（`r`）：一次搜索所有来源，逐条预览后自己挑一条
- 歌曲信息面板（`i`）：无歌词时展示码率、格式、文件大小与标签元信息
- 纯音乐 / 占位歌词识别：`纯音乐，请欣赏` 之类的内容不会再被当成歌词显示
- 翻译歌词支持（`.t.lrc` / `.t.lyric` 对照显示）
- 从音频文件提取内嵌歌词（ID3/Vorbis Comment）
- 专辑封面显示（`lyrics cover`）
- 歌词缓存以「艺人 + 曲名 + **时长**」为键，同名不同版本（原曲 / 伴奏 / 现场版）不会串味；路径由 `os.UserCacheDir()` 确定，如 macOS 通常为 `~/Library/Caches/cmus-lyric/`，Linux 通常为 `~/.cache/cmus-lyric/`
- 候选打分：标题相似度 60% + 艺人相似度 25% + 时长接近度 15%（±30 秒容差）
- 全角括号（`［00:05.00］`）与 GBK/UTF-8 编码自动归一化
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

| 按键         | 功能                                 |
| ------------ | ------------------------------------ |
| `q` `Ctrl+C` | 退出                                 |
| `?`          | 帮助                                 |
| `i`          | 切换歌曲信息（码率 / 格式 / 标签）   |
| `d`          | 调试                                 |
| `r`          | 搜索全部歌词来源并选择               |

选择器内：

| 按键         | 功能                             |
| ------------ | -------------------------------- |
| `↑/↓` `j/k`  | 移动候选（焦点在预览区时滚动预览） |
| `Tab`        | 在候选列表与预览区之间切换焦点   |
| `Enter`      | 使用当前高亮的歌词               |
| `s`          | 使用并保存到音频文件同目录       |
| `Esc`        | 关闭选择器                       |

### 歌词解析逻辑

1. 从音频文件提取内嵌歌词（ID3 USLT / Vorbis Comment）
2. 在音频文件同目录下查找 `<文件名>.lrc` / `<文件名>.lyric`（以及 `<曲名>.lrc`）
3. 若存在 `.t.lrc` / `.t.lyric` 文件，翻译歌词会显示在每行下方
4. 查找本地缓存（键为「艺人 + 曲名 + 时长」）
5. 若均未找到，**并行**查询 LRCLIB、网易云音乐、酷狗音乐、lyrics.ovh，取得分最高的候选
6. 随时可按 `r` 手动重搜，并在选择器里自行挑选

**候选打分**：标题相似度 60% + 艺人相似度 25% + 时长接近度 15%（±30 秒容差）；带时间轴的歌词始终排在纯文本之前，伴奏 / remix / cover 版本会被降权。

**确实没有歌词？** 若所有来源都为空（或曲目被标记为纯音乐），播放器会展示歌曲信息面板：格式、码率、文件大小、年份、流派、曲目号、内嵌封面等，而不是显示一句假的「纯音乐，请欣赏」。

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
│   ├── lyric/            # 歌词解析、缓存、多来源（lrclib/网易云/酷狗/lyrics.ovh）、音频探测
│   └── player/           # Bubble Tea 模型、视图、歌词选择器、信息面板
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
# 首次使用：安装 git hooks 并校验工具链（仅需一次）
task setup

task build          # 构建到 bin/
task run            # 构建并运行
task lint           # 运行 golangci-lint
task test           # 运行测试
task check          # 完整质量检查（tidy + lint + test）
```

> [!NOTE]
> Lint 需要安装 [golangci-lint](https://golangci-lint.run/) v2.x：`brew install golangci-lint` 或参考[其他安装方式](https://golangci-lint.run/welcome/install/)。
>
> `task setup` 会自动安装 [lefthook](https://github.com/evilmartians/lefthook) git hooks（pre-commit: fmt+lint+build，pre-push: lint+test）。hooks 会拦截不符合 CI 要求的代码提交，check 内容与 GitHub CI 完全一致。

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
