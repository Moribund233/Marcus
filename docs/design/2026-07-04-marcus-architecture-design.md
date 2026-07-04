# Marcus 工具箱 — 架构设计文档

日期: 2026-07-04
状态: 定稿

---

## 1. 概述

Marcus 是一个桌面工具箱应用，本质是**可视化启动器**。工具通过标准包管理器（uv/bun）安装，Marcus 负责发现、启动、管理和展示它们。

### 核心技术栈

| 层级 | 技术 |
|------|------|
| 桌面外壳 | Wails v2 (Go) |
| 前端 | React + shadcn/ui |
| 运行时 | Python (via uv) / JavaScript (via bun) / 原生二进制 |
| 本地存储 | SQLite |
| 进程隔离 | Windows Job Object |

---

## 2. 贡献点（Contribution Points）

每个工具声明自己属于哪种交互类型，Marcus 据此决定呈现方式。

| 贡献点 | 交互方式 | 适用场景 | 用户界面 |
|--------|----------|----------|----------|
| `web` | 启动 HTTP 服务，Marcus 用 WebView 打开 | 有 Web 前端的工具（OCR 工具、正则测试器） | 全屏 WebView |
| `terminal` | 执行 CLI 命令，实时流式输出 | 已有 CLI（dupe-check、jq、ffmpeg） | 终端组件（实时 stdout/stderr） |
| `file` | 用户选文件/目录 → 传给工具 → 结果输出 | 文件批处理（图片压缩、格式转换） | 文件选择器 + 进度条 + 结果预览 |

三种共享同一套沙箱（Job Object、超时控制、日志采集），区别只在前端渲染和传参方式。

---

## 3. Manifest Schema

### 3.1 Python 工具 — pyproject.toml

```toml
[project.entry-points.marcus]
ocrtool = "ocrtool:marcus"
```

`ocrtool.marcus` 返回值:

```python
# web 模式 — 启动 HTTP 服务
def marcus() -> dict:
    return {
        "api_version": "v1",
        "display_name": "图片文字识别",
        "description": "基于 PaddleOCR 的高精度图片文字识别",
        "icon": "text_snippet",
        "category": "image",
        "contribution": "web",
        "web": {
            "start_command": "ocrtool",
            "port": 0,
            "health_check": "/api/health",
            "auto_open": True,
        },
        "ui": {
            "type": "form",
            "inputs": [],
            "actions": [],
            "output": {}
        }
    }
```

```python
# terminal 模式 — 执行 CLI，实时回显
def marcus() -> dict:
    return {
        "display_name": "磁盘分析器",
        "contribution": "terminal",
        "terminal": {
            "command": "du -sh {dir}",
            "args": [
                {"name": "dir", "label": "选择目录", "type": "directory"}
            ]
        }
    }
```

```python
# file 模式 — 文件批处理
def marcus() -> dict:
    return {
        "display_name": "图片压缩",
        "contribution": "file",
        "file": {
            "command": "compress.py --input {input} --output {output} --quality {quality}",
            "input_type": "file",        # file | files | directory
            "input_extensions": [".png", ".jpg", ".webp"],
            "output_type": "directory",
            "args": [
                {"name": "quality", "label": "压缩质量", "type": "number", "default": 80}
            ]
        }
    }
```

### 3.2 JS 工具 — package.json

```json
{
  "marcus": {
    "display_name": "JSON Formatter",
    "icon": "data_object",
    "category": "text",
    "contribution": "web",
    "web": {
      "start_command": "json-fmt",
      "port": 0
    }
  }
}
```

### 3.3 Binary 工具

放在 `~/.marcus/bin/` 下，工具自身支持 `--marcus-manifest` 参数输出 JSON。

### 3.4 兼容层：手动添加已有 CLI

工具无需任何修改，用户通过 Marcus UI 手动添加：

```
Marcus UI → "+ 添加工具"
  ├─ 名称: 磁盘分析器
  ├─ 命令: du -sh {path}
  └─ 参数类型: directory # Marcus 自动生成目录选择器
```

自动降级为 `terminal` 模式，显示实时输出。用户想进一步美化（加图标、加自动 UI）再写 manifest。

| 用户入口 | 是否需改工具代码 | 功能完整度 |
|----------|-----------------|-----------|
| 自动发现（entry point） | 需加 manifest | 全功能（图标、分类、UI Schema） |
| 手动添加 | 零修改 | 基础功能（命令行、输出回显） |

---

## 4. 运行时环境

Marcus 启动时检测以下环境，前端展示状态图标。

| 运行时 | 检测命令 | 状态 |
|--------|----------|------|
| Python 3 | `python3 --version` / `python --version` | available / missing |
| Node.js | `node --version` | available / missing |
| uv | `uv --version` | available / missing |
| bun | `bun --version` | available / missing |

---

## 5. 工具发现与注册表

### 发现流程

```
Marcus 启动 / 用户点击"刷新"
  ├─ uv: 执行 `uv tool list --json`，解析已安装包
  │    └─ 对每个包，读取 entry point `marcus`
  │
  ├─ bun: 执行 `bun pm ls -g`，解析全局包
  │    └─ 对每个包，读取 `package.json` 中的 `marcus` 字段
  │
  └─ binary: 扫描 `~/.marcus/bin/`
       └─ 对每个可执行文件，尝试 `--marcus-manifest`
```

### SQLite Schema

```sql
CREATE TABLE tools (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description  TEXT,
    icon         TEXT,
    category     TEXT DEFAULT 'other',
    version      TEXT,
    source       TEXT NOT NULL,         -- "python:uv" | "js:bun" | "binary:marcus" | "manual"
    contribution TEXT NOT NULL,         -- "web" | "terminal" | "file"
    package_path TEXT,
    manifest     TEXT NOT NULL,         -- 完整 manifest json
    entry_point  TEXT,
    enabled      INTEGER DEFAULT 1,
    last_seen    DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tool_runtime_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    tool_id     TEXT REFERENCES tools(id),
    pid         INTEGER,
    status      TEXT,                   -- "running" | "exited" | "killed" | "crashed"
    started_at  DATETIME,
    stopped_at  DATETIME,
    exit_code   INTEGER,
    port        INTEGER,
    error_log   TEXT
);
```

---

## 6. 进程生命周期

```
IDLE → LAUNCHING → RUNNING → STOPPING → IDLE
                     ↓
                   CRASHED → IDLE
```

### web 模式

| 阶段 | 动作 |
|------|------|
| LAUNCHING | 分配随机端口 → 设置环境变量（`MARCUS_PORT`、`MARCUS_TOOL_ID`） → 启动子进程 → 附到 Job Object → 轮询 health check（15s 超时） |
| RUNNING | WebView 打开 `http://localhost:{port}` → 后台监控子进程 |
| STOPPING | CTRL_BREAK_EVENT → 等待 5s → TerminateProcess → 清理 → 写日志 |

### terminal / file 模式

| 阶段 | 动作 |
|------|------|
| LAUNCHING | 启动子进程 → 附到 Job Object → 准备 stdout/stderr 管道 |
| RUNNING | 实时流式输出到前端终端组件 / 进度条 |
| STOPPING | 用户手动关闭 → TerminateProcess → 写日志（或子进程自然退出） |

---

## 7. 沙箱（Windows Job Object）

```go
type Sandbox struct {
    job      windows.Handle          // Windows Job Object
    process  *exec.Cmd
    pid      uint32
    limits   ResourceLimits
}

type ResourceLimits struct {
    CPULimitPercent     int     // 0 = 不限
    MemoryLimitMB       int     // 0 = 不限
    TimeoutSeconds      int     // 0 = 不限
}
```

| 维度 | 策略 |
|------|------|
| 进程树 | Windows Job Object — 子进程生的孩子也归 Job 管 |
| 资源 | CPU 配额、内存上限（可选配置） |
| 网络 | `web` 模式只绑定 `127.0.0.1:随机端口` |
| 环境变量 | 白名单：`PATH`、`TEMP`、`HOME`、`USERPROFILE` |
| 日志 | stdout/stderr 全部重定向到统一日志系统 |
| 清理 | Marcus 退出时 TerminateJobObject 自动清理 |

---

## 8. Go 模块结构

```
internal/
├── app/              # App struct, startup, Wails Bind
│   └── app.go
├── tools/            # 工具注册中心
│   ├── registry.go   # 数据库读写、发现
│   ├── scanner.go    # uv/bun/binary 扫描器
│   └── manual.go     # 手动添加 CLI 工具
├── runtime/          # 运行时环境检测
│   └── checker.go
├── sandbox/          # 进程启动、Job Object、生命周期
│   ├── process.go    # 启动/停止/监控（统一接口）
│   └── sandbox_windows.go  # Windows Job Object 实现
├── config/           # 应用配置
│   └── config.go
└── model/            # 共享数据类型
    └── types.go      # ToolManifest, UISchema, ToolInfo 等

pkg/
└── utils/

cmd/marcus/
└── main.go           # 入口
```

---

## 9. 前端结构

```
frontend/src/
├── components/
│   ├── ui/                     # shadcn/ui 组件
│   ├── tools/
│   │   ├── ToolGrid.tsx        # 工具网格主页
│   │   ├── ToolCard.tsx        # 单个工具卡片
│   │   ├── ToolDetail.tsx      # 工具详情/启动
│   │   └── ToolAddManual.tsx   # 手动添加 CLI
│   ├── runtime/
│   │   └── EnvStatus.tsx       # 运行时状态展示
│   ├── terminal/
│   │   └── TerminalView.tsx    # 实时终端输出组件（xterm.js 或自定义）
│   ├── file/
│   │   ├── FileSelector.tsx    # 文件/目录选择
│   │   └── ProgressView.tsx    # 进度展示
│   └── renderer/
│       ├── FormRenderer.tsx    # 通用表单渲染器
│       ├── OutputRenderer.tsx  # 结果渲染器
│       └── types.ts            # UISchema TypeScript 类型
├── hooks/
│   ├── useTools.ts
│   └── useRuntime.ts
├── lib/
│   └── api.ts                  # Wails Bind 调用封装
└── App.tsx
```

---

## 10. 数据流

### web 模式

```
ToolGrid → 点击工具
  ├─ Go: sandbox.Start(manifest) → Job Object → uv run ocrtool
  ├─ Go: 轮询 health check → 返回 {port, pid}
  └─ 前端: WebView.Open("http://localhost:{port}")
```

### terminal 模式

```
用户选择参数 → 点击执行
  ├─ Go: sandbox.Start(manifest) → Job Object → exec.Command("du", "-sh", dir)
  ├─ Go: 实时流式 stdout/stderr → Wails Events.Emit("tool:output", chunk)
  └─ 前端: TerminalView 接收事件 → xterm.js 渲染
```

### file 模式

```
用户选文件/目录 → 设参数 → 点击执行
  ├─ Go: sandbox.Start(manifest) → 子进程处理
  ├─ Go: 进度推送到前端
  └─ 前端: ProgressView + 完成后 OutputRenderer 显示结果
```
