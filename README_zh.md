# cmus-lyric

[English](README.md) | 中文

基于 [Bubble Tea](https://github.com/charmbracelet/bubbletea) 构建的终端歌词同步查看器，专为 [cmus](https://cmus.github.io/) 设计。

> 灵感来自 [rockagen/cmus-lyric](https://github.com/rockagen/cmus-lyric)，使用现代 Go 技术栈从零重写。

## 简介

`cmus-lyric` 通过 `cmus-remote -Q` 连接正在运行的 cmus 实例，读取当前播放曲目，并在终端中实时显示时间同步歌词。若本地没有 `.lrc` 文件，会自动从 [LRCLIB](https://lrclib.net) 或网易云音乐获取。

**功能特性：**

- 实时歌词滚动高亮
- 自动从 LRCLIB 和网易云音乐获取歌词
- 翻译歌词支持（`.tlrc` / `.tlyric` 对照显示）
- GBK/UTF-8 编码自动检测
- 进度条与播放状态
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

| 按键          | 功能     |
| ------------- | -------- |
| `q` `Ctrl+C`  | 退出     |
| `?`           | 帮助     |

### 歌词解析逻辑

1. 在音频文件同目录下查找 `<文件名>.lrc` 或 `<文件名>.lyric`
2. 若存在 `.tlrc` / `.tlyric` 文件，翻译歌词会显示在每行下方
3. 若本地无歌词文件，依次从 LRCLIB、网易云音乐获取，并保存为 `.lrc`

## 项目结构

```
cmus-lyric/
├── cmd/lyrics/           # 应用入口
├── internal/
│   ├── player/           # Bubble Tea 模型、cmus IPC、歌词渲染
│   └── lyric/            # 歌词获取（LRCLIB、网易云）
├── .github/workflows/    # CI/CD（tag 触发自动发布）
├── Taskfile.yml          # 构建任务
├── .golangci.yml         # Linter 配置
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
