# 运行时环境策略

日期: 2026-07-05
状态: 定稿

---

## 1. 概述

Marcus 本质是可视化启动器，**不管理也不持有运行时**。Python 工具的环境隔离由 uv 负责，JS 工具由 bun 负责。Marcus 只负责发现和启动。

### 核心原则

| 原则 | 说明 |
|------|------|
| **不造轮子** | 环境管理委托给 uv/bun，Marcus 不重复实现 |
| **不持有环境** | Marcus 不创建/管理 venv，不存储依赖 |
| **统一发现接口** | 扫描器读取 manifest，决定如何启动 |
| **渐进式增强** | 先保障核心流程跑通，后续按需优化 |

---

## 2. Python 工具 — uv

### 现状

`uv tool install <pkg>` 的行为：
- 每工具独立 venv：`~/.local/share/uv/tools/<name>/`
- 全局包缓存：`~/.cache/uv/`（内容寻址，依赖去重）
- 工具命令加入 PATH 的 uv bin 目录

磁盘效率：50 个工具共享同一份 `pydantic` 不重复占用，因为 uv 使用硬链接/缓存。

### Marcus 策略

| 操作 | 方式 |
|------|------|
| 发现 | `uv tool list --json` → 读取 entry point `marcus` |
| 读取 manifest | `uv run --with <toolname> python -c "from <module> import marcus; import json; print(json.dumps(marcus()))"` |
| 启动工具 | 直接调用命令（PATH 包含 uv bin 目录），或 `uv tool run <name>`（更可靠） |

---

## 3. JS 工具 — bun

### 现状

`bun install -g <pkg>` 的行为：
- 全局安装到 `~/.bun/install/global/`
- 命令加入 PATH 的 bun bin 目录
- **无隔离机制**（bun 设计如此，依赖在 package.json 中声明）

### Marcus 策略

| 操作 | 方式 |
|------|------|
| 发现 | `bun pm ls -g` + 读取 `package.json` 中 `marcus` 字段 |
| 读取 manifest | 从 `package.json` 的 `marcus` 字段直接解析 |
| 启动工具 | `bunx <name>`（按需运行，不持久安装），或直接调用命令 |

### bunx 作为默认启动方式

`bunx` 的优势：
- 按需下载/运行，不占用磁盘
- 自动处理依赖
- 无需 `bun install -g` 前置步骤

策略：terminal/web 模式的 JS 工具默认用 `bunx <name>` 启动。

---

## 4. 扫描器改进

### Python 工具 manifest 读取

当前 `ScanUV()` 只读取工具名称和版本，**未读取 manifest**。改进后流程：

```
uv tool list --json
  └─ 遍历每个工具
       └─ 检查 entry point 是否包含 "marcus"
            └─ 执行 uv run --with <name> python -c "from <module> import marcus; print(json.dumps(marcus()))"
                 └─ 解析 JSON → 填充 ToolInfo 字段
                      └─ UpsertTool
```

### Go 代码变化

在 `internal/tools/scanner.go` 中新增 `readUvManifest(name, module string) (*model.ToolManifest, error)`：

```go
func (s *Scanner) readUvManifest(name, module string) (*model.ToolManifest, error) {
    // uv run --with <name> python -c "from <module> import marcus; import json; print(json.dumps(marcus()))"
    // 解析 stdout JSON → ToolManifest
}
```

### 安全考虑

- 从子进程 stdout 读取 JSON，需设置超时（默认 10s）
- 解析失败则跳过该工具（非致命）
- 不信任工具输出，做基本类型校验

---

## 5. 沙箱启动 — 初始无需改动

`uv tool install <pkg>` 将工具命令添加到 PATH。Wails 子进程继承用户环境，因此 `exec.Command("marcus-textstats")` 可直接执行。

如需更可靠的启动方式（例如用户通过 Marcus UI 安装工具后 PATH 未刷新），后续可改用 `uv tool run <name>`，但当前阶段**不做沙箱改动**。

---

## 6. 验证方法

1. 构建 whl：`temp/marcus-textstats/dist/marcus_textstats-0.1.0-py3-none-any.whl`
2. 安装：`uv tool install <whl路径>`
3. Marcus 扫描 → 应显示"文本统计器"，contribution 为 terminal
4. 点击启动 → 选择文件 → 输出行/词/字符统计

---

## 7. 后续优化（非本次范围）

- 自定义 uv/bun 存储目录（环境变量 `UV_TOOL_DIR`、`UV_TOOL_BIN_DIR`）
- 自定义 Python 版本映射（每个工具指定 `requires-python`）
- 工具环境 GC（清理未使用的工具缓存）
- 离线安装（`uv tool install --offline`）
