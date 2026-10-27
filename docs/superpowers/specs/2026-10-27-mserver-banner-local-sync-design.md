# mserver：小图本地同步 + 大文件仅代理

## 目标

手机上四类资源路径今天多走 `resource-upstream` 整段代理。改为按体量分流：

- **大文件**（patch / cpk）：手机不提供本地下载与缓存；仅在 PC 资源上游可用时代理。
- **小文件**（当期 home / gacha banner）：在移交页手动同步到手机后本地直出；未同步或缺文件时用 APK 内嵌默认 PNG。

游戏客户端仍只改服务器地址；不改通用 degrader 语义。

## 决定

采用方案 2：PC 提供受账号同步口令保护的 banner 清单/内容接口；移交页按钮拉取并落盘；运行时 home/gacha 本地优先 + 占位图；patch/cpk 保持代理-only。

不采用：手机按固定路径盲拉（易漏当期 key）；zip 整包（对几张 PNG 过重）。

## 范围

| 类别 | 路径 | 行为 |
|------|------|------|
| 大文件 | `GET/HEAD /local/resources/patch/{path...}` | 仅 `resource-upstream` 代理；无上游 → degrader |
| 大文件 | `GET/HEAD /local/resources/cpk/{path...}` | 同上 |
| 小文件 | `GET/HEAD /local/home/{path...}` | 本地同步文件 → 直出；否则 APK 默认 PNG；**不再**走 upstream |
| 小文件 | `GET/HEAD /local/gacha/{path...}` | 同上 |

同步覆盖与 PC 现有路由一致的当期图：`HomeBanner`、`HomeEventBanner`、默认/五星 gacha、`GachaBanners` 各 key（路径形如 `/local/home/banner.png`、`/local/gacha/{key}.png`）。

不做：CPK/patch 落手机；自动/定时同步；`/local/gacha-covers` 及其它路径；改 httpapi 业务体；改 degrader 文案。

## 架构

```
移交页「同步图片」
    → 本机 POST/GET /local/launcher/banners（与 borrow 同：回环 + 转发电脑地址/口令）
    → PC GET /local/sync/banners + X-Account-Sync-Token
    → 写入 filesDir/banners/…（path 与 URL 对应）

游戏 GET /local/home|gacha/…
    → 有同步文件：ServeFile
    → 无：assets 默认 PNG

游戏 GET /local/resources/patch|cpk/…
    → resource-upstream 代理，否则 degrader
```

占位图：从现有 server 资源中选一张常用 banner，打包进 APK assets，固定复用。

## 同步协议

- 方法/路径：`GET /local/sync/banners`
- 鉴权：`X-Account-Sync-Token`（与借号/账号列表相同；token 未配置则接口不可用）
- 响应 JSON：

```json
{
  "files": [
    { "path": "/local/home/banner.png", "sha256": "…", "bytes": "<base64>" }
  ],
  "missing": 0
}
```

- `files`：能读到的当期 home/gacha 图；单张缺失则跳过该项并增加 `missing`，其余照常返回。
- 手机校验：`path` 必须前缀为 `/local/home/` 或 `/local/gacha/`，禁止 `..`；内容须为可用 PNG（至少魔数检查）；先写临时文件再 rename。
- UI：按钮忙碌不可重入；成功「已同步 N 张」；部分成功带缺张数；失败保留已有本地图并显示原因。

清单来源：与 kairi-server / `ResourceFileHandler` 已挂的 banner 配置同一套，避免手机维护第二份路径表。

## 运行时读写

- 同步根目录：应用 `filesDir` 下固定子目录（如 `banners/`），URL path 映射为相对文件路径。
- `GET`/`HEAD`：本地命中则带合适 `Content-Type: image/png` 与长度；未命中返回内嵌默认图（仍 200），避免空 404/降级踢登录。
- home/gacha **从** `resource-upstream` 代理挂载中移除，只走本地文件或占位图。
- catalog / `cpk_file.csv` / `cpk_patch.txt` 仍按现逻辑读手机 `resources/`，本设计不改。

## 错误处理

| 情况 | 行为 |
|------|------|
| 口令错误 / sync 未开 | 非 2xx，按钮失败，不改本地图 |
| PC 不可达 | 网络错误文案，不清空已同步图 |
| 单张 PC 缺文件 | 计入 `missing`，其余写入 |
| 非法 path / 非 PNG | 跳过不写盘 |
| patch/cpk 无上游或上游失败 | degrader（现语义） |

## 测试

1. PC：`/local/sync/banners` 鉴权；清单含配置中的 home/gacha 路径。
2. 手机：有本地文件直出；无文件出占位；home/gacha 不依赖 upstream。
3. patch/cpk：有 upstream 代理；无 upstream 降级；同步不产生 patch/cpk 文件。
4. 路径穿越被拒绝。
5. 移交页按钮忙碌与成功/失败/部分成功文案。

## 实现落点（指导）

- PC：accountsync 或 cnbootstrap 旁挂 `/local/sync/banners`，复用已解析的 banner 路径。
- 手机 Go：`mserver/internal/server` 拆分 downloads——patch/cpk 代理；home/gacha 本地+占位。
- Android：`TransferActivity` 按钮；经 `/local/launcher/banners` 回环转发后写盘；APK assets 默认 PNG。
