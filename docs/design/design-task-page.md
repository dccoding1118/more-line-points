# 第 8 部分：GitHub Pages 靜態任務網頁 (設計文件)

## 1. 簡介與目標

本文件定義以 GitHub Pages 靜態網頁與 JSON 資料檔案，取代原有的 Discord/Email 長篇通知清單。
主要設計原則為：
1. **極速部署**：`tasks.json` 直接 push 到 Repo，由 `raw.githubusercontent.com` 讀取，跨越 GitHub Pages build 延遲。
2. **純靜態架構**：沒有背景伺服器，由純 HTML/JS 從本機讀取 JSON 與透過 localStorage 管理使用狀態。
3. **無縫接軌**：繼承原 `notify` 模組的 8 大過濾與排序規則，但透過現代 Web UI 加強使用者體驗。

---

## 2. 交付物

| 項目                         | 說明                                                                   |
| ---------------------------- | ---------------------------------------------------------------------- |
| `gh-pages/index.html` (前端) | 任務列表介面，實作 JSON 取得與使用者點擊狀態的 localStorage 控制。     |
| `internal/taskpage/` (後端)  | Go 服務層，負責從 SQLite 撈取當日任務資料並產出 `tasks.json`。         |
| `deploy.yml` (CI/CD)         | 僅於 `index.html` 異動時將此網頁部屬至 GitHub Pages 的工作流程。       |
| `tasks.json`                 | 每日任務的資料載體，由後端服務每次 sync 後更新並 commit-back 至 repo。 |

---

## 3. 架構說明

### 3.1 靜態檔案結構與佈署
- **`index.html`**：存放於 `gh-pages/index.html`。
- **`tasks.json`**：存放於 `data/tasks.json`。

**讀取機制**：
因為 Repo 屬 Public，前端 `index.html` 將透過 Fetch API 從:
`https://raw.githubusercontent.com/{owner}/{repo}/main/data/tasks.json?t={timestamp}`
取得資料，此方法完全繞過 GitHub Actions 的部屬延遲（1~3分鐘），僅由 `sync` 完成自動 push 後約 10~30 秒即自動隨 CDN 覆蓋更新。

### 3.2 流程
1. `00:00:05` Cloud Scheduler 發動 `sync` 工作流程。
2. Go 應用程式抓取 API、更新資料庫。
3. **[新] 呼叫 `internal/taskpage` 產出當日 `tasks.json`**。
4. 將更新的 `db` 與 `tasks.json` 一起提交 (git commit & push)。
5. `sync.yml` 利用 `workflow_call` 同步發動 `notify`。
6. `notify` 發送只包含總任務數量與該 `index.html` Web URL 給 Discord/Email 使用者。
7. 使用者由手機或電腦點閱網頁查看與執行點擊任務。

---

## 4. `internal/taskpage` 模組設計 (Go 端)

**職責**：將資料庫存放的當日 `Activity` 及 `DailyTask` 組合，並轉匯出為符合前端需求的 JSON。

### 4.1 介面設計

```go
package taskpage

type JSONExporter struct {
    activityStore storage.ActivityStore
    taskStore     storage.DailyTaskStore
}

func NewJSONExporter(as storage.ActivityStore, ts storage.DailyTaskStore) *JSONExporter

// Export 將 targetDate 當日的任務打包輸出至指定路徑的 JSON 檔案
func (e *JSONExporter) Export(ctx context.Context, targetDate time.Time, outputPath string) error
```

### 4.2 資料處理邏輯

1. 呼叫 `activityStore.GetActivitiesByDate` 與 `taskStore.GetDailyTasksByDate` 取回 `targetDate` 當日的任務清單。
2. 針對每一筆 Activity 計算上架天數 (Day)：
   - `day = targetDate 以天為單位與 activities.valid_from 之間的差值`。
     - Day `0`: 當日上架 (NEW)
     - Day `>= 1`: (DAY2, DAY3 ...)
   - 注意：若 `valid_from` 大於 `targetDate` 則過濾不輸出 (尚未開始)。若活動處於 `is_active=0` (已標記停用) 則同樣跳過。
3. 將此等資料 mapping 成 json struct，包含必要欄位 (`id`, `title`, `channel_name`, `channel_id`, `type`, `day`, `keyword`, `deep_link`, `page_url`)。
4. 時間與日期：頂層需記錄 `date` ("YYYY-MM-DD") 與 `generated_at` (RFC3339)。
5. 將結構 encode 後以覆寫模式寫入 `outputPath`。

### 4.3 JSON 結構定義
```json
{
  "date": "2026-04-12",
  "generated_at": "2026-04-12T00:00:45+08:00",
  "tasks": [
    {
      "id": "ygzIQIheuHU0L9fC",
      "title": "收集幸福禮物",
      "channel_name": "LINE 禮物",
      "channel_id": "@linegiftshoptw",
      "type": "keyword",
      "day": 0,
      "keyword": "0313幸福禮物",
      "deep_link": "https://line.me/R/oaMessage/@linegiftshoptw/?0313幸福禮物",
      "page_url": "https://..."
    }
  ]
}
```

---

## 5. `gh-pages/index.html` 前端設計

本介面為一個不具任何依賴框架 (No React/Vue)、極輕量的原生 HTML/JS 應用，並完整具備 RWD 設計。

### 5.1 8 大分類與排序規則 (繼承原 notify)

從 JSON `tasks` 取出後，前端需依照下列順序重組畫面區塊，若該分類下的項目為空則不顯示該分類標題：

1. **🔑 關鍵字任務** (`type == "keyword"`)
2. **🛍️ 收藏指定店家** (`type == "shop-collect"`)
3. **🎁 點我試手氣** (`type == "lucky-draw"`)
4. **🗳️ 投票任務** (`type == "poll"`)
5. **📱 App 簽到任務** (`type == "app-checkin"`)
6. **📗 購物護照任務** (`type == "passport"`)
7. **🔗 分享好友任務** (`type == "share"`)
8. **📌 其他任務** (`type == "other"`)

*(註：若 `type == "unknown"` 或超出上述定義，則一律略過不顯示。)*

### 5.2 欄位處理規則
- 若具有 `day: 0` 標籤，顯示紅/綠底色的 `[NEW]` badge。
- 若 `day > 0`，則顯示 `[DAY{day+1}]` badge。
- 連結觸發選擇 (`[前往 →]`)：
   - 優先判定是否有 `deep_link`，若有則直接開啟 (如 keyword 系列)；若無再開啟 `page_url`。

### 5.3 點擊追蹤 (localStorage) 邏輯

1. **取得紀錄**
   讀取 `localStorage.getItem("clicked_{date}")` (如 `clicked_2026-04-12`) 取得該日的已點擊 ID Array。若為空，則建立空 Array (`[]`)。
2. **跨日清理防護 (Garbage Collection)**
   - 剛載入頁面時遍歷 localStorage，檢查開頭是否為 `clicked_` 而其 date 卻不等於 JSON root 上的 `date`，是的話主動清除它清除佔用量。
3. **觸發變色**
   - 渲染各清單項目迴圈時，比對 `id` 是否在剛才取得的點擊陣列中，是的話加上 `class="done"`：
     - 打勾符號 (顯示 ✅)
     - 灰階、半透明 (Opacity 降到 50%)
     - 取代 `[前往 →]` 為 `[已完成]`
4. **發生點擊**
   - 點擊按鈕後將 ID 加入陣列並回寫 `localStorage.setItem("clicked_{date}", JSON.stringify(arr))`。
   - 透過 DOM 即時掛上 `done` class 切換視圖，同時透過 `window.open` 開啟目標 URL。

### 5.4 UI 版面構成
一個簡潔卡片的頂部顯示今日總結 (如：`📅 今日任務 04/12 · 已完成 3 / 7`)，其下方就是 8 個分類大標題下的清單：

```text
┌──────────────────────────────────────────┐
│  📅 今日任務  04/12  ·  已完成 3 / 7         │
├──────────────────────────────────────────┤
│  🔑 關鍵字任務                               │
│                                          │
│  ✅ NEW  LINE 購物    SHOP0412   [已完成]    │
│  ☐  DAY2 LINE Pay     PAY0412   [前往 →]   │
│  ☐  DAY4 LINE TODAY   TODAY04   [前往 →]   │
│                                          │
│  🔗 分享任務                                 │
│                                          │
│  ✅ NEW  好友分享抽好禮            [已完成]   │
│  ☐  DAY5 LINE TODAY 分享活動      [前往 →]   │
└──────────────────────────────────────────┘
```

---

## 6. GitHub Actions 影響 (Deploy)

### 6.1 `deploy.yml` 部署流程
- 僅綁定 `gh-pages/index.html` 發生異動或是手動 `workflow_dispatch` 時觸發。
- 利用 `actions/upload-pages-artifact@v3` 將 `gh-pages/` 抓取封裝，再由 `actions/deploy-pages@v4` 送交 GitHub Pages 管線。
- 注意：此排程與每日 Sync 動作毫無耦合，確保 Sync 流的極致穩定與效率。
