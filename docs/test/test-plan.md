# 整合測試與端到端測試計畫

> **適用範圍**：Patch 0 ~ Patch 5 整合測試 + 端到端測試
> **核心工具**：`testscript`（聲明式 `.txtar` 格式）
> **原則**：每個案例獨立測試資料、自設自拆、冪等性保證

---

## 1. 工具選型

### 1.1 testscript（整合測試與 E2E 主力工具）

使用 Go 的 [`testscript`](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript) 套件，以 `.txtar` 聲明式檔案描述測試案例。

**優勢**：
- **聲明式語法**：測試意圖清晰可讀，不是「用 Go 寫測試」，而是「用腳本描述場景」
- **自帶隔離**：每個 `.txtar` 在獨立暫存目錄執行，天然冪等
- **內建清理**：暫存目錄自動刪除，不殘留測試資料
- **可自訂指令**：註冊 `db-query`、`db-count` 等 helper，聲明式驗證 DB 狀態

**適用場景**：
- CLI 指令測試（`scheduler sync`、`scheduler notify`）
- 跨模組整合測試（config → storage → apiclient → syncer）
- 端到端流程（Sync → Notify 全鏈路）

### 1.2 hurl 適用性評估

| 面向       | 分析                                                                      |
| ---------- | ------------------------------------------------------------------------- |
| 本專案特性 | 本專案是 **CLI 工具**（消費外部 API），不對外暴露 HTTP 端點               |
| hurl 定位  | 適合測試 **HTTP Server API**（聲明式 request → assert response）          |
| 結論       | **目前不適用**。若未來新增 REST API 端點（如 Web Dashboard），再引入 hurl |

### 1.3 工具總覽

| 測試層級 | 工具         | 格式                       |
| -------- | ------------ | -------------------------- |
| 單元測試 | Go `testing` | `_test.go`（Table-Driven） |
| 整合測試 | `testscript` | `.txtar`（聲明式）         |
| E2E 測試 | `testscript` | `.txtar`（聲明式）         |

---

## 2. 測試基礎設施

### 2.1 testscript 入口

```go
// internal/integration_test.go
//go:build integration

package integration_test

import (
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"scheduler": schedulerMain, // Wire cmd/scheduler/main.go
	}))
}

func TestIntegration(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:                 "tests/integration/patch0", // include patch 0 ~ 5
		RequireExplicitExec: true,
		Setup:               setupMockServers, // Start mock HTTP servers
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"db-count":  cmdDBCount,  // db-count <table> <expected_count>
			"db-query":  cmdDBQuery,  // db-query <sql> → stdout
			"db-insert": cmdDBInsert, // db-insert <fixture_name>
		},
	})
}
```

### 2.2 自訂 testscript 指令

| 指令           | 語法                                                  | 用途                                    |
| -------------- | ----------------------------------------------------- | --------------------------------------- |
| `db-count`     | `db-count activities 5`                               | 斷言表的筆數                            |
| `db-query`     | `db-query "SELECT type FROM activities WHERE id='A'"` | 查詢並輸出至 stdout，搭配 `stdout` 斷言 |
| `db-insert`    | `db-insert fixture/seed_3activities.sql`              | 執行 SQL 腳本寫入測試資料               |
| `db-reset`     | `db-reset`                                            | 清空所有表（保留 schema）               |
| `mock-api`     | `mock-api page1.json page2.json`                      | 設定 mock LINE API 回傳的分頁回應       |
| `mock-discord` | `mock-discord`                                        | 驗證 Discord mock 收到的訊息            |
| `mock-email`   | `mock-email`                                          | 驗證 Email mock 收到的訊息              |

### 2.3 Mock Server 架構

```
testscript Setup()
  ├── Mock LINE API Server  → $MOCK_LINE_API (env var)
  ├── Mock Discord Server   → $MOCK_DISCORD_API (env var)
  └── Mock SMTP Server      → $MOCK_SMTP_ADDR (env var)
```

每個 `.txtar` 透過 `mock-api` 指令設定回應，mock server 根據設定回傳對應 JSON。

### 2.4 冪等性與隔離機制

```
每個 .txtar 案例：
  1. testscript 建立獨立 $WORK 暫存目錄
  2. 透過 $ROOT_DIR 環境變數參照 tests/fixture/ 下的共用測資
  3. 測試使用 $WORK/data/test.db 作為 SQLite 路徑
  4. 執行測試步驟
  5. testscript 自動刪除 $WORK

  → 天然隔離、天然冪等、無殘留、測資集中管理
```

---

## 3. 測試資料目錄 (Fixture)

為避免 `.txtar` 檔案過於臃腫，所有共用的設定檔、API 回應與 SQL 腳本皆存放於 `tests/fixture/`。

### 3.1 目錄結構

```
tests/fixture/
├── config/              # 共用設定檔範本
│   ├── config.yaml
│   ├── channel_mapping.yaml
│   └── parse_rules.yaml
├── api/                 # Mock API 回應 JSON (.json)
│   ├── api_AB.json
│   ├── api_5activities_2pages_p1.json
│   └── ...
├── html/                # Mock HTML 詳細頁 (.html)
│   ├── detail.html
│   ├── detail_share.html
│   └── ...
└── db/                  # 共用 Fixture SQL (.sql)
    ├── seed_A.sql
    └── seed_AB.sql
```

### 3.2 關鍵環境變數

| 變數名           | 說明                 | 範例用途                                                                  |
| ---------------- | -------------------- | ------------------------------------------------------------------------- |
| `$ROOT_DIR`      | 專案根目錄絕對路徑   | `exec scheduler sync --config $ROOT_DIR/tests/fixture/config/config.yaml` |
| `$MOCK_LINE_API` | Mock Server 動態 URL | `api: base_url: $MOCK_LINE_API` (於 config.yaml 中自動注入)               |

### 3.3 設定檔範本 ($ROOT_DIR/tests/fixture/config/config.yaml)

```yaml
database:
  path: data/test.db
channel_mapping:
  path: $ROOT_DIR/tests/fixture/config/channel_mapping.yaml
parser:
  rules_path: $ROOT_DIR/tests/fixture/config/parse_rules.yaml
api:
  base_url: $MOCK_LINE_API
  region: tw
```

### 3.4 測試活動物件定義

| 物件 ID | title          | channelName | type    | valid_until | channel_id    |
| ------- | -------------- | ----------- | ------- | ----------- | ------------- |
| `ACT-A` | LINE 購物集點  | LINE 購物   | keyword | 2026-03-31  | @lineshopping |
| `ACT-B` | LINE Pay 回饋  | LINE Pay    | keyword | 2026-03-31  | @linepay      |
| `ACT-C` | 好友分享抽獎   | LINE        | share   | 2026-03-31  | —             |
| `ACT-D` | 集點卡任務     | LINE Points | other   | 2026-03-31  | —             |
| `ACT-E` | 新活動（未知） | LINE TODAY  | unknown | 2026-04-15  | @linetoday    |
| `ACT-X` | 過期活動       | LINE        | keyword | 2026-03-01  | —             |

### 3.5 測試 DailyTask 物件定義

| activity_id | use_date   | keyword  | url                                               |
| ----------- | ---------- | -------- | ------------------------------------------------- |
| ACT-A       | 2026-03-04 | SHOP0304 | `https://line.me/R/oaMessage/@shopping/?SHOP0304` |
| ACT-A       | 2026-03-05 | SHOP0305 | `https://line.me/R/oaMessage/@shopping/?SHOP0305` |
| ACT-B       | 2026-03-04 | PAY0304  | `https://line.me/R/oaMessage/@linepay/?PAY0304`   |
| ACT-B       | 2026-03-05 | PAY0305  | `https://line.me/R/oaMessage/@linepay/?PAY0305`   |

---

## 4. 測試案例

### 4.1 testscript 範例格式

以 IT-1-01 為例，展示使用 Fixture 的 `.txtar` 結構：

```txtar
# IT-1-01: First Sync - Empty DB
# Scenario: First sync against empty database, API returns 5 activities across 2 pages.

# Setup mock API with 2-page response
mock-api $ROOT_DIR/tests/fixture/api/api_5activities_2pages_p1.json $ROOT_DIR/tests/fixture/api/api_5activities_2pages_p2.json

# Execute sync
exec scheduler sync --config $ROOT_DIR/tests/fixture/config/config.yaml

# Verify stderr log (log.Println writes to stderr by default)
stderr 'Changes detected and updated'

# Verify DB state
db-count activities 5
db-count sync_state 6
db-query 'SELECT COUNT(*) FROM activities WHERE type = "unknown"'
stdout '^4$'
```

### 4.2 testscript 範例格式 (Patch 2 包含動態修改 Mock)

當需要動態注入常數（如 `$MOCK_LINE_API`）至 Mock JSON 時，需先將 Fixture `cp` 進沙盒環境：

```txtar
# IT-2-01: Parse Keyword Activity

# 1. 將 Fixture 複製到沙盒以利 exec sed 修改
cp $ROOT_DIR/tests/fixture/api/api_list.json api_list.json

# 2. 注入 Mock Server URL 至 JSON
exec sed -i s|MOCK_API_URL|$MOCK_LINE_API|g api_list.json

# 3. 設定 Mock 順序
mock-api api_list.json $ROOT_DIR/tests/fixture/html/detail.html

# 4. 執行與斷言
exec scheduler sync --config $ROOT_DIR/tests/fixture/config/config.yaml
db-query 'SELECT type FROM activities WHERE id = "ACT-KW"'
stdout '^keyword$'
```


---

### Patch 0 — 專案骨架

| #       | 案例                   | 類型   | 測試資料                   | 關鍵斷言                                 |
| ------- | ---------------------- | ------ | -------------------------- | ---------------------------------------- |
| IT-0-01 | Config → DB 完整初始化 | ✅ 正面 | 合法 config.yaml + mapping | `db-count activities 0`；三表存在        |
| IT-0-02 | Channel Mapping 查詢   | ✅ 正面 | 含 3 筆 mapping            | stdout 檢查查詢結果                      |
| IT-0-03 | DB 重複初始化冪等      | ✅ 正面 | 同路徑執行兩次             | 第二次無 error，資料保留                 |
| IT-0-04 | Config 不存在          | ❌ 反面 | 路徑指向不存在檔案         | stderr 含 `failed to read`               |
| IT-0-05 | Config 格式錯誤        | ❌ 反面 | 非法 YAML                  | stderr 含 `failed to parse`              |
| IT-0-06 | DB 路徑不可寫          | ❌ 反面 | 路徑 `/root/no_perm.db`    | `! exec scheduler sync`，stderr 有 error |
| IT-0-07 | 環境變數未設定         | ❌ 反面 | `${UNSET_VAR}` 未設        | 對應欄位為空                             |

---

### Patch 1 — 清單 Sync (L1+L2)

| #       | 案例                | 類型 | API Mock               | DB Fixture                | 關鍵斷言                            |
| ------- | ------------------- | ---- | ---------------------- | ------------------------- | ----------------------------------- |
| IT-1-01 | 首次 Sync（空 DB）  | ✅    | 5 筆 / 2 頁            | `seed_empty`              | `db-count activities 5`；NewCount=5 |
| IT-1-02 | 二次 Sync（無變化） | ✅    | 相同 5 筆              | IT-1-01 後                | stdout `unchanged`；DB 無變動       |
| IT-1-03 | 新增活動            | ✅    | A/B/C（+C）            | `seed_3activities`(A/B)   | `db-count activities 3`；NewCount=1 |
| IT-1-04 | 活動消失            | ✅    | A/B（-C）              | `seed_3activities`(A/B/C) | C 的 `is_active=0`                  |
| IT-1-05 | 資料變更 (L2)       | ✅    | B title 變             | `seed_3activities`        | B title 已更新                      |
| IT-1-06 | 過期清除            | ✅    | A/B                    | `seed_expired`(含 X)      | X 被刪除                            |
| IT-1-07 | Channel ID 對應成功 | ✅    | channelName 有 mapping | `seed_empty`              | `channel_id` 正確                   |
| IT-1-08 | Channel ID 無對應   | ✅    | channelName 無 mapping | `seed_empty`              | `channel_id` 空；stderr 有 warning  |
| IT-1-09 | 多頁分頁遍歷        | ✅    | 25 筆 / 3 頁           | `seed_empty`              | `db-count activities 25`            |
| IT-1-10 | 三次 Sync 冪等性    | ✅    | 相同資料 ×3            | `seed_empty`              | 三次執行結果一致                    |
| IT-1-11 | API HTTP 500        | ❌    | 回傳 500               | `seed_empty`              | `! exec`；DB 無變動                 |
| IT-1-12 | API 回傳無效 JSON   | ❌    | `{broken`              | `seed_empty`              | `! exec`；DB 無變動                 |
| IT-1-13 | API 連線逾時        | ❌    | 延遲回應               | `seed_empty`              | `! exec`；stderr 含 timeout         |
| IT-1-14 | API 分頁中斷        | ❌    | 第 2 頁 500            | `seed_empty`              | `! exec`；不寫入部分資料            |
| IT-1-15 | Context 取消        | ❌    | 正常回應               | `seed_empty`              | `! exec`；stderr 含 canceled        |
| IT-1-16 | API 回傳空清單      | ❌    | 0 筆                   | `seed_3activities`        | 所有活動 MarkInactive               |

---

### Patch 2 — 詳細頁解析 (L3)

| #       | 案例                    | 類型 | Detail HTML Mock  | DB Fixture    | 關鍵斷言                                    |
| ------- | ----------------------- | ---- | ----------------- | ------------- | ------------------------------------------- |
| IT-2-01 | 新活動判定 keyword      | ✅    | 含關鍵字區塊 HTML | type=unknown  | type→`keyword`；daily_tasks 有資料          |
| IT-2-02 | 新活動判定 share        | ✅    | 分享型 HTML       | type=unknown  | type→`share`；action_url 有值               |
| IT-2-03 | 新活動判定 passport     | ✅    | 護照型 HTML       | type=unknown  | type→`passport`；action_url 有值            |
| IT-2-04 | 新活動判定 shop-collect | ✅    | 收藏店家 HTML     | type=unknown  | type→`shop-collect`；daily_tasks 有資料     |
| IT-2-05 | 新活動判定 lucky-draw   | ✅    | 抽獎型 HTML       | type=unknown  | type→`lucky-draw`；action_url 有值          |
| IT-2-06 | 新活動判定 app-checkin  | ✅    | 簽到型 HTML       | type=unknown  | type→`app-checkin`；action_url 有值         |
| IT-2-07 | poll 前篩命中           | ✅    | —                 | clickURL=poll | type→`poll`；action_url=clickURL；不抓 HTML |
| IT-2-08 | 新活動判定 other        | ✅    | 未知型 HTML       | type=unknown  | type→`other`；無 action_url/tasks           |
| IT-2-09 | L3 Hash 無變化          | ✅    | 相同 HTML         | type=keyword  | daily_tasks 不重寫，時間更新                |
| IT-2-10 | L3 Hash 有變化          | ✅    | 任務清單變更      | type=keyword  | 舊 tasks 刪除，新 tasks 寫入                |
| IT-2-11 | 詳細頁 HTTP 404         | ❌    | 回傳 404          | type=unknown  | type 保留 `unknown`                         |
| IT-2-12 | HTML 解析格式無法辨識   | ❌    | 空 HTML           | type=unknown  | type→`other`                                |
| IT-2-13 | 部分日期解析失敗        | ❌    | 格式錯亂          | type=unknown  | type→`keyword`；僅寫入成功解析的 task       |

---

### Patch 3 — 推播通知

| #       | 案例                         | 類型 | DB Fixture                | 關鍵斷言                                              |
| ------- | ---------------------------- | ---- | ------------------------- | ----------------------------------------------------- |
| IT-3-01 | keyword 推播（含 deep link） | ✅    | `seed_tomorrow_tasks`     | `mock-discord` 含 `https://line.me/R/oaMessage/`      |
| IT-3-02 | channel_id 未對應 (warn)     | ✅    | `seed_no_channel_mapping` | 含 `⚠️` 且無 URL 連結, `on_missing=warn`               |
| IT-3-03 | channel_id 未對應 (skip)     | ✅    | `seed_no_channel_mapping` | 不含該筆推播活動, `on_missing=skip`                   |
| IT-3-04 | channel_id 未對應 (error)    | ❌    | `seed_no_channel_mapping` | `! exec`；`on_missing=error`                          |
| IT-3-05 | 8 大任務類型混合推播         | ✅    | `seed_all_8_types`        | 包含 8 個分類大標（🔑, 🛍️, 🎁, 🗳️, 📱, 🛂, 🔗, 📌）           |
| IT-3-06 | 無明日任務                   | ✅    | 空 DB                     | 提示「當日無需執行任務」或空報表                      |
| IT-3-07 | 僅 Discord                   | ✅    | `seed_tomorrow_tasks`     | Discord ✓、Email ✗                                    |
| IT-3-08 | 僅 Email                     | ✅    | `seed_tomorrow_tasks`     | Discord ✗、Email ✓                                    |
| IT-3-09 | 雙管道同時                   | ✅    | `seed_tomorrow_tasks`     | 兩者各收到一則，且 Email 包含 `<ul>` 清單             |
| IT-3-10 | deep link 格式正確           | ✅    | ACT-A + KW SHOP0304       | `https://line.me/R/oaMessage/@lineshopping/?SHOP0304` |
| IT-3-11 | Discord API 失敗             | ❌    | `seed_tomorrow_tasks`     | Discord error；Email 正常                             |
| IT-3-12 | SMTP 連線失敗                | ❌    | `seed_tomorrow_tasks`     | Email error；Discord 正常                             |
| IT-3-13 | 雙管道皆失敗                 | ❌    | `seed_tomorrow_tasks`     | 回傳合併 error                                        |

---

### Patch 4 — GitHub Actions 自動化

> GitHub Actions 無法在 testscript 中模擬，以手動 workflow_dispatch 驗收。

| #       | 案例                | 類型 | 驗收方法                             |
| ------- | ------------------- | ---- | ------------------------------------ |
| IT-4-01 | sync.yml 手動觸發   | ✅    | Action 成功；DB 變更 → 自動 commit   |
| IT-4-02 | notify.yml 手動觸發 | ✅    | Discord + Email 收到推播             |
| IT-4-03 | Secrets 缺失        | ❌    | Action 失敗，log 含 config error     |
| IT-4-04 | commit-back 無衝突  | ✅    | push 成功，`data/line_tasks.db` 更新 |

---

### Patch 5 — Bot 指令介面

| #       | 案例                    | 類型 | 輸入指令         | DB Fixture     | 關鍵斷言             |
| ------- | ----------------------- | ---- | ---------------- | -------------- | -------------------- |
| IT-5-01 | `/status`               | ✅    | `/status`        | 有 sync_state  | 含 Sync 時間、活動數 |
| IT-5-02 | `/list`                 | ✅    | `/list`          | 3 筆活動       | 列出 3 筆            |
| IT-5-03 | `/keywords`（預設明日） | ✅    | `/keywords`      | 明日有 2 組    | 列出 2 組            |
| IT-5-04 | `/keywords 0305`        | ✅    | `/keywords 0305` | 0305 有資料    | 列出該日             |
| IT-5-05 | `/sync`                 | ✅    | `/sync`          | API mock ×5    | 回覆 Sync 結果       |
| IT-5-06 | `/notify`               | ✅    | `/notify`        | 明日有任務     | 推播 + 回覆確認      |
| IT-5-07 | 未知指令                | ✅    | `/unknown`       | —              | 回覆說明列表         |
| IT-5-08 | `/list` 無資料          | ❌    | `/list`          | 空 DB          | 「無有效活動」       |
| IT-5-09 | `/keywords` 無資料      | ❌    | `/keywords`      | 明日無 keyword | 「無關鍵字任務」     |
| IT-5-10 | `/keywords abc`         | ❌    | `/keywords abc`  | —              | 日期格式提示         |
| IT-5-11 | `/sync` API 失敗        | ❌    | `/sync`          | API mock 故障  | 回覆失敗原因         |
| IT-5-12 | `/status` 從未 Sync     | ❌    | `/status`        | sync_state 空  | 「尚未執行過同步」   |

---

## 5. 端到端測試（E2E）

> 使用 testscript 的 `.txtar` 格式，串聯多個指令模擬完整使用場景。
> 使用 **真實 SQLite 檔案**（非 `:memory:`），驗證持久化行為。

### E2E-01：完整首日循環

```txtar
# E2E-01: Full first-day cycle (sync → verify → notify → verify → re-sync)

# Setup
mock-api api_20activities_mixed.json

# Step 1: First sync
exec scheduler sync --config config.yaml
stdout 'New: 20'

# Step 2: Verify DB
db-count activities 20
db-query "SELECT COUNT(*) FROM activities WHERE type = 'keyword'"
stdout '^8$'
db-query "SELECT COUNT(*) FROM keywords"
! stdout '^0$'

# Step 3: Notify
exec scheduler notify --config config.yaml

# Step 4: Verify notifications sent
mock-discord contains '📅'
mock-discord contains '🔑 關鍵字任務'
mock-email contains '📅'

# Step 5: Re-sync (idempotent)
exec scheduler sync --config config.yaml
stdout 'unchanged'

-- config.yaml --
...（完整 config 含 mock server URLs）

-- channel_mapping.yaml --
mappings:
  "LINE 購物": "@lineshopping"
  "LINE Pay": "@linepay"
on_missing: warn
```

### E2E-02：活動生命週期

```
1. Sync-1：API 回傳 A/B/C → 全部寫入
2. Sync-2：API 回傳 A/B/C（B title 變更）→ B 更新
3. Sync-3：API 回傳 A/B（C 消失）→ C MarkInactive
4. Sync-4：C valid_until 設為過去 → C 被刪除
驗證：每步 db-count + db-query 斷言
```

### E2E-03：推播管道容錯

```
1. Discord ✓ / Email ✗ → 執行 notify → Discord 收到，回傳 partial error
2. Discord ✗ / Email ✓ → 執行 notify → Email 收到，回傳 partial error
3. 兩次收到的訊息內容邏輯一致
```

### E2E-04：Bot → Sync → Notify 全鏈路

```
模擬 Discord WebSocket Gateway 收到互動事件 /sync → /list → /keywords → /notify
每步驗證 Bot 回覆內容正確
```

### E2E-05：Channel Mapping 降級

```
mapping 僅 2 組，sync 3 筆 keyword 活動
notify 輸出：2 筆 deep link + 1 筆 ⚠️ 純文字
```

### E2E-06：大量資料壓力

```
100 筆活動 / 10 頁，50 筆 keyword 各含 7 天排程
完整 Sync + Notify 在 30 秒內完成
db-count activities 100；db-count keywords 350
```

---

## 6. 檔案結構

```
tests/
├── integration/           # 整合測試 .txtar 檔案
│   ├── patch0/
│   │   ├── it_0_01_config_db_init.txtar
│   │   ├── it_0_04_config_not_found.txtar
│   │   └── ...
│   ├── patch1/
│   │   ├── it_1_01_first_sync.txtar
│   │   ├── it_1_02_second_sync_unchanged.txtar
│   │   └── ...
│   ├── patch2/
│   ├── patch3/
│   └── patch5/
├── e2e/                   # E2E 測試 .txtar 檔案
│   ├── e2e_01_first_day_cycle.txtar
│   ├── e2e_02_activity_lifecycle.txtar
│   └── ...
├── fixture/               # 共用測試資料
│   ├── db/
│   │   ├── seed_empty.sql
│   │   ├── seed_3activities.sql
│   │   ├── seed_with_keywords.sql
│   │   ├── seed_expired.sql
│   │   ├── seed_tomorrow_tasks.sql
│   │   └── seed_no_channel_mapping.sql
│   ├── api/               # Mock API 回應 JSON
│   │   ├── api_5activities_1page.json
│   │   ├── api_5activities_2pages_p1.json
│   │   ├── api_5activities_2pages_p2.json
│   │   └── ...
│   └── html/              # Mock 活動詳細頁 HTML
│       ├── detail_keyword.html
│       ├── detail_share.html
│       └── detail_other.html
└── helpers/
    └── testmain_test.go   # testscript TestMain + 自訂指令
```

---

## 7. 執行指令

```bash
# 所有整合測試
go test -v -tags integration -timeout 120s ./tests/...

# 特定 Patch
go test -v -tags integration -run "IT_1_" ./tests/...

# E2E 測試
go test -v -tags e2e -timeout 300s ./tests/...

# 單一案例
go test -v -tags integration -run "it_1_01" ./tests/...
```

---

## 8. 案例統計

| 階段     | 正面   | 反面   | 格式     | 小計   |
| -------- | ------ | ------ | -------- | ------ |
| Patch 0  | 3      | 4      | `.txtar` | **7**  |
| Patch 1  | 10     | 6      | `.txtar` | **16** |
| Patch 2  | 10     | 3      | `.txtar` | **13** |
| Patch 3  | 9      | 4      | `.txtar` | **13** |
| Patch 4  | 2      | 2      | 手動驗收 | **4**  |
| Patch 5  | 7      | 5      | `.txtar` | **12** |
| E2E      | 6      | —      | `.txtar` | **6**  |
| **合計** | **47** | **24** |          | **71** |
