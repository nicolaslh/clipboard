# 📋 Clipboard Manager

一款基于 Wails v3 构建的 macOS 剪切板管理工具，轻量、快速、本地优先。

## 功能特性

### 核心功能
- 🔄 **自动监听** — 实时捕获系统剪切板变化，自动保存历史
- 🔍 **全文搜索** — 模糊匹配内容，快速定位历史记录
- 📅 **日期筛选** — 日期选择器精确查看某天的记录
- 📌 **固定重要内容** — 固定项不会被清除或过期删除
- 🗑️ **批量清除** — 一键清除所有未固定记录（带确认）

### 智能分类
- 自动识别内容类型：**文本**、**链接**、**代码**、**文件路径**、**邮箱**
- 彩色标签直观展示
- 按类型快速筛选

### 片段库
- 将常用内容加入命名分组（如「代码模板」「常用地址」）
- 片段库独立查看，方便管理高频使用的文本

### 自动过期清理
- 默认保留 30 天历史，超期自动删除
- 启动时和每小时自动执行清理
- 可在设置中自定义保留天数（1-365 天）
- 固定项永不过期

### 大文本预览
- 超过 200 字符的记录自动截断显示
- 点击「展开预览」查看完整内容
- 预览弹窗支持滚动和一键复制

## 技术栈

| 层级 | 技术 |
|------|------|
| 框架 | [Wails v3](https://v3.wails.io/) |
| 后端 | Go 1.23 |
| 前端 | Vanilla JS + Vite |
| 存储 | SQLite (WAL 模式) |
| 剪切板 | [golang.design/x/clipboard](https://pkg.go.dev/golang.design/x/clipboard) |
| 构建 | [Task](https://taskfile.dev/) |

## 快速开始

### 前置要求

- Go 1.23+
- Node.js 18+
- [Wails v3 CLI](https://v3.wails.io/getting-started/installation/)
- [Task](https://taskfile.dev/)

### 安装 Wails CLI

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

### 开发模式

```bash
wails3 dev
```

前端热重载 + 后端自动重编译。

### 构建

```bash
wails3 task build
```

产物在 `bin/clipboard`。

### 打包为 .app

```bash
wails3 task darwin:package
```

产物在 `bin/clipboard.app`，可直接运行或拖入 Applications。

## 快捷键

| 快捷键 | 功能 |
|--------|------|
| `Cmd+Shift+V` | 显示/隐藏窗口 |
| `Cmd+F` | 聚焦搜索框 |
| `Escape` | 清除搜索/关闭弹窗 |
| 点击条目 | 复制到剪切板 |

## 项目结构

```
.
├── main.go                  # 应用入口，窗口配置，快捷键绑定
├── clipboard_service.go     # 剪切板监听、分类、片段管理、过期清理
├── store.go                 # SQLite 数据层，迁移，时间/分类查询
├── frontend/
│   ├── index.html           # 主页面
│   ├── src/
│   │   ├── main.js          # 前端逻辑（搜索、筛选、片段、设置）
│   │   └── style.css        # 扁平化 UI 样式
│   ├── bindings/            # Wails 自动生成的绑定（gitignore）
│   ├── vite.config.js
│   └── package.json
├── build/
│   ├── config.yml           # Wails 构建 & dev_mode 配置
│   ├── appicon.png          # 应用图标 1024x1024
│   └── darwin/
│       ├── Info.plist        # macOS 应用元数据
│       └── icons.icns        # macOS 图标
├── Taskfile.yml             # 构建任务定义
├── go.mod
└── go.sum
```

## 设计决策

- **轮询而非 Watch** — `golang.design/x/clipboard` 的 Watch 在 macOS 上需要主线程 Cocoa 事件循环，与 Wails WebView 冲突，改为 500ms 轮询
- **SQLite WAL 模式** — 读写并发不阻塞，适合频繁写入场景
- **去重逻辑** — 连续相同内容不重复保存；自身写入剪切板时跳过捕获
- **自动迁移** — 旧数据库自动添加新列，无需手动操作
- **扁平化 UI** — 跟随系统亮/暗模式，无多余装饰

## 许可证

MIT
