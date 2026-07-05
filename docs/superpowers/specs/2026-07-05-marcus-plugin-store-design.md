# Marcus 插件商店设计文档

## 概述

基于 GitHub Releases + Action 的插件注册中心，为 Marcus 工具箱提供工具发现、搜索、安装和更新能力。零服务器运维成本，全部基础设施由 GitHub 提供。

## 架构

```
┌─────────────────────────────────────────────────────┐
│ GitHub Repo (marcus-plugins)                        │
│                                                     │
│  GitHub Releases ──→ .marcus-plugin 包              │
│       │                                             │
│       ▼                                             │
│  GitHub Action ──→ index.json (自动维护)            │
│                                                     │
│  raw.githubusercontent.com ← 客户端拉取 index.json │
└─────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────┐
│ Marcus 客户端 (internal/store/)                     │
│                                                     │
│  store.Client ──→ 拉取 index → 缓存到本地 SQLite    │
│  store.Installer ──→ 下载 .marcus-plugin → 解压安装 │
│  store.Updater ──→ 检查版本 → UI 通知更新            │
└─────────────────────────────────────────────────────┘
```

## 远程仓库 (marcus-plugins)

### 发布机制

- 插件通过 `gh release create` 发布
- Release tag 格式：`<plugin-id>@v<semver>`
- Release Asset：`.marcus-plugin` 包（ZIP 格式）
- 每次 Release 的 body 作为发布说明

### 索引同步

`sync-index.yml` Action 在以下事件触发：
- `release: published` — 新插件或新版本
- `release: deleted` — 下架
- `release: edited` — 更新发布说明（弃用标记）

### 弃用策略

Release notes 含 "最终版本" 或 `[deprecated]` 标记时，Action 自动在 index 中标记 `deprecated: true`。

## 插件包格式 (.marcus-plugin)

ZIP 包，内容：
- `marcus-plugin.json` — 清单文件（必选，符合 JSON Schema）
- `icon.png` — 图标（推荐，128x128）
- `dist/` — 工具源码或二进制
- `docs/index.html` — 详情页（可选）

## Marcus 客户端模块 (internal/store/)

### 组件

| 组件 | 职责 |
|------|------|
| `client.go` | HTTP 客户端，拉取远程 index.json，带缓存 TTL |
| `installer.go` | 下载 .marcus-plugin，解压，校验 manifest，调用 Registry.UpsertTool |
| `updater.go` | 对比本地版本与远程 latest_version，生成更新列表 |
| `cache.go` | 本地 SQLite 缓存表，缓存 index 数据 |

### 数据流

1. Marcus 启动 / 用户打开商店页 → `client.Sync()` → 拉取 index.json → 存入 SQLite 缓存
2. 商店 UI → 从本地缓存读取 → 分类、搜索、列表渲染
3. 用户点击安装 → `installer.Install(pluginID, version)` → 下载 Release Asset → 解压 → 注册到 Registry
4. 更新检查 → `updater.CheckUpdates()` → 返回 `[{pluginID, currentVersion, latestVersion}]`

### 本地缓存表

```sql
CREATE TABLE store_cache (
    id          TEXT PRIMARY KEY,
    display_name TEXT,
    description TEXT,
    categories  TEXT,       -- JSON array
    latest_version TEXT,
    versions    TEXT,       -- JSON object
    deprecated  INTEGER DEFAULT 0,
    deprecation_message TEXT,
    synced_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE store_installed (
    plugin_id    TEXT PRIMARY KEY,
    version      TEXT NOT NULL,
    installed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (plugin_id) REFERENCES store_cache(id)
);
```

## UI 集成

Marcus 前端新增 "商店" 页面：
- 分类过滤（按 categories）
- 搜索（按 display_name / description 本地过滤）
- 插件卡片：icon + 名称 + 描述 + 版本 + 安装按钮
- 详情弹窗：描述、版本历史、截图、安装/更新
- 状态栏：更新可用通知

## 开放问题

- 是否需要 GitHub Token 提升 API 频率限制
- 插件包的 checksum 校验（目前通过 Release 的 integrity）
