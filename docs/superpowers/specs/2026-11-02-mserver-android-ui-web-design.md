# Android 微型服务端：Web UI 视觉还原（方案 A）

## 目标

按设计稿 `D:\springfield\download\Page 1` 中的**方案 A（深色科技感）**，对 mserver Android 客户端做**像素级**视觉还原，并在第一版把整套壳接到现有能力：服务启停与通知、远程连接、资源同步、账号移交、日志。

实现路径：**按屏重建静态 Web UI**（HTML/CSS/JS + 字体）+ 全屏 `WebView` + Kotlin JS Bridge。不直接把 PAGX 导出的整张 3512×1116 长画板当作运行时页面。

本文件替换 `2026-10-17-mserver-page-design.md` 中关于**页面布局与导航壳**的说法。借出、交还、进度选择、持有者规则、账号列表接口与日志文件语义不改，除非下文显式改动。

## 决策摘要

| 项 | 选择 |
|---|---|
| 视觉风格 | 方案 A：近黑底 `#0B0D12`、青霓虹 `#22D3C5`、紫色星云渐变、毛玻璃卡片、等宽标签 |
| 范围 | Splash + 底栏四 Tab（服务 / 连接 / 同步 / 日志）+ 完整日志 + 同步 Tab 内账号移交 |
| 保真 | 像素级：布局、字号、间距、材质尽量贴稿；以稿中单机手机框为基准宽度 |
| 完成度 | 第一版可点可用，接到现有 Kotlin/Sm 逻辑 |
| 技术 | Web UI + Bridge（非 Compose 主 UI；非 Compose+Web 混排壳） |

## 架构

单宿主 `Activity`（沿用或收束为 `MainActivity`）内嵌全屏 `WebView`。

三层：

1. **Web UI** — `assets/web/` 下按屏拆分的静态资源；负责展示、路由与手势。
2. **JS Bridge** — Web ↔ Kotlin 的少量 JSON 命令与状态推送；Web 不直连业务网络。
3. **现有 Kotlin 域** — `ServerService`、端口/配置落盘、`SyncClient`、移交与日志读取等保持为逻辑源；Activity 负责绑定 Service 并把快照推给 Web。

系统返回键：子页（若有独立路由的完整日志深链等）先回到对应 Tab；再按现有退出/后台策略处理。通知栏「停止服务」仍走 `ServerService`，并触发 UI 快照刷新。

设计资产：从 Page 1 抽取方案 A 色板、圆角、字体与各手机框尺寸，重建可在该基准宽度下像素对齐的页面；实机用 viewport 缩放铺满，**不擅自改相对比例**。稿内 woff2 打进 `assets/web/fonts/`，避免系统字体回退破坏字重。

## 页面与信息架构

### 全局壳

- **Splash**（冷启动展示一次）→ 主壳。文案对齐稿：应用名 /「本地服务管家」/「正在初始化本地服务环境…」。
- **主壳**：中部页面 + 底栏四 Tab：**服务 / 连接 / 同步 / 日志**。
- 服务、连接等页可带 **LogBar** 摘要；点「展开完整日志」进入完整日志（与 Tab「日志」同一能力）。

### Tab 与能力映射

| 界面 | 稿内要点 | 现有逻辑 |
|---|---|---|
| 服务 | 端口、启停、常驻通知说明与预览 | `ServerService`、端口文件、前台通知 |
| 连接 | IP / PORT / TOKEN、连接/断开、测试心跳 | 现移交页的 `server-host` / `remote-port` / `sync-token`；探活/心跳经 Bridge |
| 同步 · 数据同步 | 进度、取消、上次摘要、未连接空态 | 经本机端口的现有同步路径（`SyncClient` 等） |
| 同步 · 账号移交 | 账号卡状态、归还/借出主按钮、未连接空态 | 现 `TransferActivity` 能力与账号列表规则 |
| 日志 | 完整列表、清空、分类计数 | `server.log` / 现日志展示语义 |

### 状态与前置条件

- **服务**：未启动 / 监测中 / 已启动（语义对齐现主页；视觉改用方案 A 状态卡）。已启动前本机端口可改；监测中与已启动时端口锁定。监测中不能停止（与现规则一致）。
- **连接**：未连接 / 已连接（展示心跳）。断网后自动重试并写日志——有现成重试则接上，否则第一版最小可用为失败提示 + 写日志，规格不要求新协议。
- **同步 / 移交**：未连接远程时只显示空态（稿 08/09），不静默请求。未起本地服务时，连接 Tab 可浏览，操作前提提示需先启动服务。
- **移交选中规则**：主按钮随选中在「归还到远程」与「借出至本地」间切换。底层 `server` / `mine` / `other` 与 409 进度选择规则不变。卡片视觉固定映射：`mine` → 白·可归还；`server` → 绿·可借出；`other` → 橙·不可归还（不可选操作）；未知 `state` → 灰·不可操作。

### 旧页处理

`TransferActivity`、`LogActivity` 的**界面退役**：逻辑迁入 Bridge。实现期删除或改为不可达；不得再作为主路径入口。

本规格**不**改变：账号列表 API、口令头、借出/交还/进度选择、系统伙伴过滤、日志追加语义。

## Bridge 与数据流

### 通道

- Web → Kotlin：`JavascriptInterface` 命令。
- Kotlin → Web：`evaluateJavascript`（或等价）推送事件，至少包括完整快照 `uiState`。

### 状态源

Kotlin 为唯一源。定时或事件后推送 `uiState`，字段分组：

- `server`：running / probing、port、uptime、listen 地址、error
- `connection`：host、port、token、connected、lastHeartbeatMs、error
- `sync`：phase（idle / running / done）、progress、lastSummary
- `transfer`：accounts[]（含展示态）、selection、busy、choice（409 时）
- `logs`：recent[]（LogBar）与完整列表（按需 `fetchLogs`）
- `meta`：appVersion、splash 是否已结束

Web 按快照渲染。按钮可有短暂 loading，最终以快照为准。

### 命令（Web → Kotlin）

- 服务：`startServer` / `stopServer` / `setPort`
- 连接：`saveConnection` / `connect` / `disconnect` / `ping`
- 同步：`startSync` / `cancelSync`
- 移交：`refreshTransfer` / `selectAccount` / `transferAction`（及进度选择若需）
- 日志：`fetchLogs` / `clearLogs`
- 其它：`splashFinished`

每条命令返回 `{ ok, error? }`。副作用通过后续 `uiState` 体现。

### 错误展示

Bridge 返回 `error` 字符串（可选 `code`）。Web 在页内横幅或字段下展示，不改用系统 `AlertDialog` 做业务错误。通知权限请求仍走原生系统 UI。

业务失败文案尽量复用现有日志句（如电脑地址无效、没有连上本机服务），避免同一失败写两套互斥措辞。

## 视觉还原约束

- 只落地方案 A；不做 B/C。
- 基准为稿中单机手机框宽度；缩放铺满实机，保持相对几何。
- 字体使用稿内 woff2 子集/文件，名与字重对齐 Noto Sans SC 等导出项。
- 入库资源为重建后的多页 Web；Page 1 目录仅作设计源，不作为 APK 运行时整页加载。
- 验收：Splash、服务运行态、连接已连接、同步中、移交列表、完整日志、四 Tab 激活态与稿并排或叠图对照。

## 非目标

- Compose 主界面或 Compose+WebView 混排导航壳。
- 修改 Sm/PC 同步与移交协议语义。
- 方案 B/C、老板目录导出、与本 UI 无关的 PC 启动器改动。

## 测试

- **Kotlin**：Bridge 命令单测——端口校验、未启动/未连接拒操作、`uiState` 关键字段。
- **Web**：关键状态 class / 路由的轻量校验（可选）；实机对照清单必做。
- **回归主路径**：启停 + 通知停服、连接保存与探活、数据同步、移交归还/借出与 409 选择、日志清空与 LogBar/Tab 进入完整日志。
- **合并门槛**：至少一台模拟器或真机跑通四 Tab 主路径，并保留关键屏与设计稿对照截图。

## 实现顺序（指导）

1. 宿主 WebView + Bridge 骨架 + `uiState` 快照。
2. 抽 token/字体，重建 Splash 与底栏壳。
3. 服务 Tab 接线。
4. 连接 Tab 接线（配置迁出原移交页顶栏）。
5. 同步 Tab：数据同步 + 账号移交。
6. 日志 Tab / 完整日志 + LogBar。
7. 退役旧 View Activity UI；像素对照与回归。
