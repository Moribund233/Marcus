# Marcus

> 跨平台工具箱 — 基于 Wails + React + Go

## 简介

Marcus 是一个跨平台桌面工具箱，用于集中管理和快速启动各类开发工具。支持自动发现已安装的工具（Python uv、Bun、本地二进制）、沙箱执行进程、监控运行时状态。

## 功能

- **工具自动发现** — 扫描 `uv tool`、`bun` 全局包及本地 `bin/` 目录中的工具
- **多类型工具** — Web 服务、终端命令、文件处理工具
- **沙箱执行** — 带资源限制（CPU、内存、超时）的进程隔离
- **运行时检测** — 自动检测 Python/uv、Bun、Node.js 等运行时
- **手动添加** — 支持手动注册自定义命令
- **包安装** — 直接安装 `.whl`（Python）和 `.tgz`（npm）包
- **日志记录** — 工具运行历史与输出追踪
- **深色主题** — 默认深色 UI

## 技术栈

| 层        | 技术                                |
| --------- | ----------------------------------- |
| 后端      | Go + Wails v2                       |
| 前端      | React 18 + TypeScript + Vite        |
| 样式      | Tailwind CSS + Radix UI + Lucide    |
| 数据库    | SQLite (modernc.org/sqlite)         |
| 许可证    | MIT                                 |

## 快速开始

```bash
# 开发模式
wails dev

# 构建生产包
wails build

# 构建含 NSIS 安装器 (Windows)
wails build --windows/amd64 --nsis
```

## 配置文件

默认路径：`~/.marcus/config.json`

| 字段               | 说明             | 默认值 |
| ------------------ | ---------------- | ------ |
| `db_path`          | SQLite 数据库路径 | `~/.marcus/marcus.db` |
| `tools_dir`        | 本地二进制工具目录 | `~/.marcus/bin` |
| `auto_scan`        | 启动时自动扫描    | `true` |
| `terminate_on_exit`| 退出时终止工具    | `true` |
| `theme`            | 主题 (`dark`)     | `dark` |
| `language`         | 界面语言          | `zh-CN` |

## 目录结构

```
Marcus/
├── main.go                  # 入口
├── app.go                   # Wails 绑定
├── internal/
│   ├── app/                 # 核心逻辑
│   ├── config/              # 配置管理
│   ├── model/               # 数据模型
│   ├── runtime/             # 运行时检测
│   ├── sandbox/             # 进程沙箱
│   └── tools/               # 工具扫描与注册
├── frontend/                # React 前端
│   └── src/                 # UI 源码
├── build/                   # 构建脚本 & 安装器
└── wails.json               # Wails 项目配置
```

## 许可证

[MIT](LICENSE)
