# COS 开通操作手册（腾讯云对象存储）

> 目的：在**腾讯云**开通对象存储 COS，创建视频/音频资源的存储桶，并拿到代码所需的
> **SecretId / SecretKey / 桶名 / 地域**。
> 开通后把值写进服务器 `/opt/cw-tc/shared/.env`（本文 §四 表格），即可接通
> 「课程视频/音频」的资源下载（讲义/课件/代码已走文件服务，不依赖 COS）。
>
> ⚠️ 腾讯云控制台界面会改版，以下「入口 / 菜单名」以**操作当天实际界面为准**；
> 但**每一步要拿到什么产物**（SecretId/SecretKey、桶名、地域）是固定不变的，照做即可。

---

## 一、前置确认

1. 你已有腾讯云账号（生产 PostgreSQL / 备案都在腾讯云，主账号或已有子账号均可）。
2. COS 是**按量付费**：存储 + 下行流量计费。视频约 17GB，属小体量，月度成本很低
   （远低于自建对象存储），但**不是免费**——控制台会提示开通计费，点确认即可。
3. 资源访问策略：**私有读写桶 + 预签名 URL**（与课程资源「一律私有读」一致，铁律/数据字典 §1），
   **不要**开「公有读」，否则视频可被无签名直链抓取。

---

## 二、开通 COS 服务并创建存储桶

### 2.1 开通 COS

1. 登录腾讯云控制台 `https://console.cloud.tencent.com/`。
2. 顶部搜索框输入「**对象存储**」，进入「对象存储 COS」。
3. 首次进入会提示「**开通 COS 服务**」——点开通（免费开通，按量付费，无需预充值）。
4. 同意服务协议，完成开通。

### 2.2 创建存储桶

1. 在「存储桶列表」点「**创建存储桶**」。
2. 填写四项：

| 项 | 填什么 | 说明 |
|---|---|---|
| **名称** | `cwtc-content` | 自定义小写+数字，全局唯一；系统会自动追加你的 **APPID**，最终桶名形如 `cwtc-content-1251234567` |
| **所属地域** | 与现有服务器/PostgreSQL**同地域**（如 上海 `ap-shanghai` 或 南京 `ap-nanjing`） | 同地域内网流量更省，下载延迟低；记下地域码（如 `ap-shanghai`） |
| **访问权限** | **私有读写** | 关键：别选「公有读私有写」 |
| **版本控制/其他** | 默认关闭 | 视频资源无需多版本 |

3. 点「确定」创建。

> **产物①：桶名**（完整名 `cwtc-content-<APPID>`）→ `.env` 的 `CWTC_COS_BUCKET`。
> **产物②：地域码**（如 `ap-shanghai`）→ `.env` 的 `CWTC_COS_REGION`。
>
> ⚠️ 记桶名时**连 APPID 一起记**：控制台列表显示的就是完整桶名（`cwtc-content-1251234567`），
> 别只记 `cwtc-content`——API 调用需要带 APPID 的完整名。

---

## 三、获取 API 密钥（SecretId / SecretKey）

### 方式 A（推荐）：子账号 + 最小权限

生产密钥不要用主账号（主账号密钥能删库删桶，泄露后果大）。给 COS 单独开一个子账号：

1. 进「**访问管理 CAM**」`https://console.cloud.tencent.com/cam`。
2. 「**用户 → 用户列表 → 新建用户**」→ 选「**快速创建**」或「自定义创建」。
3. 用户名：`cwtc-cos`；**访问方式勾选「编程访问」**（要拿到 API 密钥）。
4. 创建完成后，在用户列表点该用户 →「**API 密钥**」→「**新建密钥**」。
5. 页面弹出 **SecretId** 与 **SecretKey**（SecretKey 只显示一次，立即复制保存）。
6. 「**关联策略**」给该子账号加一条**只授本桶**的最小权限策略：

   - 「策略 → 新建自定义策略 → 按策略生成器」或直接用 JSON：

   ```json
   {
     "version": "2.0",
     "statement": [{
       "effect": "allow",
       "action": ["cos:PutObject", "cos:GetObject", "cos:HeadObject", "cos:DeleteObject"],
       "resource": ["qcs::cos:<地域>::uid/<APPID>:<桶名>/*"]
     }]
   }
   ```

   - 把 `<地域>`/`<APPID>`/`<桶名>` 换成真实值；最小动作是 `PutObject`(上传)+`GetObject`(下载)+`HeadObject`(查元数据)。

### 方式 B（省事）：主账号密钥

若只想尽快跑通、后续再收紧，可临时用主账号密钥：

1. 进「访问管理 CAM」→「**API 密钥管理**」`https://console.cloud.tencent.com/cam/capi`。
2. 「新建密钥」→ 得到 SecretId / SecretKey（SecretKey 只显示一次）。

> ⚠️ **主账号密钥权限极大**，只用于临时联调，正式上线务必切到方式 A 的子账号最小权限。
>
> **产物③：SecretId / SecretKey** → `.env` 的 `CWTC_COS_SECRET_ID` / `CWTC_COS_SECRET_KEY`。

---

## 四、下号后回填清单（写服务器 `/opt/cw-tc/shared/.env`）

| 变量 | 取值来源 | 示例 |
|---|---|---|
| `CWTC_COS_SECRET_ID` | CAM 的 SecretId | `AKIDxxxxxxxxxxxxxxxx` |
| `CWTC_COS_SECRET_KEY` | CAM 的 SecretKey | `xxxxxxxxxxxxxxxxxxxx` |
| `CWTC_COS_BUCKET` | 完整桶名（带 APPID） | `cwtc-content-1251234567` |
| `CWTC_COS_REGION` | 地域码 | `ap-shanghai` |
| `CWTC_COS_BASE_URL` | 可选，自定义 CDN/域名；缺省用 `https://<桶名>.cos.<地域>.myqcloud.com` | `https://cdn.cw-tc.ccit-agent.cn` |

> 回填后 `sudo systemctl restart cw-tc-server`，我再接「视频/音频上传到 COS + 预签名下载」代码并实测。

---

## 五、验证方法（确认桶已可用）

开通后可先在控制台验证桶权限（不用等我写代码）：

1. **网页上传**：进存储桶 → 「上传文件」传一个测试文件，确认能传、能预览（私有桶预览即走签名）。
2. **公网地址探测**：浏览器直接访问
   `https://<桶名>.cos.<地域>.myqcloud.com/<测试文件名>`
   应返回 **AccessDenied**（XML 报错）——这证明是私有读写、没公网直链，正确。
3. （可选）用 `coscmd` 命令行上传 17GB 视频（比控制台网页传快），需先
   `pip install coscmd && coscmd config -a <SecretId> -s <SecretKey> -b <桶名> -r <地域>`。

---

## 六、关键注意事项

1. **访问权限务必「私有读写」**：课程资源是私有的（数据字典「一律私有读」），公开读会泄露付费内容。
2. **地域与服务器一致**：同地域走内网、延迟低且内网流量常免流量费；地域不同会收跨地域流量费。
3. **密钥只写服务器 `/opt/cw-tc/shared/.env`**（`chmod 600`），**绝不放仓库**（同支付密钥铁律）。
4. **子账号最小权限**：正式上线用方式 A，只授 `PutObject/GetObject/HeadObject/DeleteObject` 于本桶。
5. **成本口径**：17GB 视频存储 + 学员下载流量按量计费；早期学员少，成本很低，但上线前心里有数。
6. **对象键 = storage_key**：上传时保持目录结构（如 `AI化教学创新03-CF/01_你好世界/video/讲解.mp4`），
   与数据库 `asset.storage_key` 一致，下载才拼得上。

---

**（COS 开通操作手册 v1.0 · 由 CCIT-CO 维护 · 开通后按 §四 写服务器 `.env` 并重启服务）**
