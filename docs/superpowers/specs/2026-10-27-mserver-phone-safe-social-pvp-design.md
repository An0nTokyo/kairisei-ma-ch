# mserver：社交 / PVP 降级替代（phone-safe stub）

## 目标

手机端未挂路由会走 degrader（`res_code=-1`，「该功能需要连接电脑上的服务器」），客户端常因此回到登录页。本设计用 **路由层 phone-safe stub / 包装** 替换下列 9 个 POST 的降级，使其尽量不踢登录；**不修改**现有 `httpapi` 原实现。

## 决定

采用方案 2：在 `NewSoloPlay` 增加 `registerPhoneSafeRoutes`，把 9 条登记进 play 白名单；catch-all 命中业务房而非 degrader。

不采用：

- 挂真社交写库路由再吞副作用（易误写、边界糊）
- 改通用 degrader 语义（影响所有未实现路径）

## 范围

仅这 9 个 POST：

| 分组 | 路径 |
|------|------|
| 空列表 | `FollowoFollowShow`、`FollowFollowerShow`、`FriendSearch` |
| 成功空操作 | `FollowAdd`、`FollowUnFollow`、`PvpEnd` |
| 尽量真实 | `UserProfileShow`、`PvpShow`、`PvpStart2` |

不做：真人好友持久化；改 degrader；静态页（公告/日程表等）；扩大其它未实现协议。

约束沿用既有 Sm 设计：不建号；非持有者拒绝；游戏客户端只改服务器地址。

## 路由架构（落点）

```
ServeMux 显式 Handler（登录 / catalog / proxy / launcher…）
        │ 未命中
        ▼
"/" catch-all → solo.Dispatch(play)
        ├ path ∈ 白名单 → OpenPlay chi
        │     ├ registerSoloRoutes
        │     ├ registerMultiRoomRoutes
        │     ├ registerDefenseRoutes（有 defense 时真 PvpStart2）
        │     └ registerPhoneSafeRoutes  ← 本设计
        └ 否则 → degrade.Handler（-1）
```

- **显式 Handler**：接线（不同路径不同实现），不依赖 play。
- **play 白名单**：门禁；chi Handler：业务。
- phone-safe 挂在业务房并进入白名单；原 `httpapi` 函数体不改。

## 分流与回包契约

统一：成功路径禁止 `res_code=-1`；session/holder 中间件与现 play 一致；字段非法仍按现有坏请求处理，不伪装成功。

### 空列表 stub

- `FollowoFollowShow`：仅 session → `follow_max` 固定 `80`（与高等级跟随时上限同量级，stub 不读账号成长表）+ `friends: []`
- `FollowFollowerShow`：session + `page` → `friends: []`，`follower_num: 0`，`page_max: 1`
- `FriendSearch`：session + `inviteid` → `user: []`（不查库）

CN 外壳字段与现有 `friend_adapters` 成功形对齐。

### 成功空操作 stub

- `FollowAdd`：校验 `userids`（可丢弃 0）→ `request_userids: []`，`is_friend_full: 0`；**不写关注**
- `FollowUnFollow`：校验 `userid` → `{}`；**不写库**
- `PvpEnd`：校验 `btluid` / `is_win` / `is_retire` / `input_cmd` 形状 → 结算壳用**当前未改动**的积分与挑战次数（before=after）；**不**调用 `FinishPVP`、不记 receipt、不改存档

### 尽量真实（包装，可转发 business）

- `UserProfileShow`：转发现有 business；自己走本机资料。他人查无 → 最小占位 `profile`（避免 400/-1 踢链路），不实现真人社交图
- `PvpShow`：转发现有业务；若因缺 pvp master 等失败 → 安全壳（`result=0`，积分/挑战用账号已有或 0，`pvp_reset` 可用 0）
- `PvpStart2`：**始终由 phone-safe 挂一条包装路由**（从 `registerDefenseRoutes` 移除对该 path 的重复注册，避免 chi 冲突）。有 defense 且有可用对手 → 包装内转发现有开战逻辑；无 defense / 无对手 → `res_code=-1200` + 可读文案（沿用「暂无可用…」类）；**禁止** degrader

## 错误处理

| 情况 | 行为 |
|------|------|
| session / 非持有者 | 现有中间件 |
| 字段非法 | 4xx / 现适配坏请求 |
| stub 成功 | `res_code=0` 等价成功，无写库 |
| PvpStart2 不可开 | `-1200`，非 `-1` |
| Profile/PvpShow 转发失败 | 占位成功或非 `-1` 软失败 |
| 其它未挂路径 | 仍 degrader |

## 测试

1. 九条均在白名单，响应不含 degrader 文案。  
2. 三列表 stub：空数组 + 必填外壳。  
3. FollowAdd / FollowUnFollow / PvpEnd：成功且 revision / 好友关系 / 积分不变。  
4. 无 defense 时 PvpStart2 → `-1200`。  
5. 有 defense 时真开打路径回归。  
6. 未列入的 POST 仍 degrader。

## 实现位置（指导）

- 新增 phone-safe 注册与 stub/包装：优先 `server/internal/cnbootstrap`（与现有 CN 适配同层），由 `NewSoloPlay` 调用。  
- 不改 `httpapi` 内 follow/pvp 函数体。  
- `mserver` 侧仅随 OpenPlay 白名单自动带上新 path；无需为这九条改 ServeMux 显式表。

## 非目标回顾

通用 degrader 语义、静态 `/disabled/web*`、image CDN、真人组队好友图、Admin —— 均不在本次。
