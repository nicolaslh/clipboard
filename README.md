# Clipboard Manager

基于 Wails v3 构建的桌面剪切板管理工具。

## 功能

- 🔄 自动监听系统剪切板变化
- 📋 保存剪切板历史记录
- 🔍 搜索历史记录
- 📌 固定重要内容
- 🗑️ 删除/清除记录
- 💾 SQLite 本地持久化存储

## 技术栈

- **后端**: Go + Wails v3
- **前端**: Vanilla JS + Vite
- **存储**: SQLite
- **剪切板**: golang.design/x/clipboard

## 开发

### 前置要求

- Go 1.23+
- Node.js 18+
- [Wails v3 CLI](https://v3.wails.io/getting-started/installation/)
- [Task](https://taskfile.dev/)

### 运行开发模式

```bash
wails3 dev
```

### 构建

```bash
wails3 build
```

### 打包

```bash
wails3 package
```

## 快捷键

- `Cmd/Ctrl + F` - 聚焦搜索框
- `Escape` - 清除搜索并返回列表
- 点击条目 - 复制到剪切板

## 项目结构

```
.
├── main.go                 # 应用入口
├── clipboard_service.go    # 剪切板监听与管理服务
├── store.go               # SQLite 数据存储层
├── frontend/              # 前端代码
│   ├── index.html
│   ├── src/
│   │   ├── main.js       # 前端逻辑
│   │   └── style.css     # 样式
│   └── package.json
├── build/                 # 构建配置
├── Taskfile.yml          # 构建任务
└── go.mod
```
