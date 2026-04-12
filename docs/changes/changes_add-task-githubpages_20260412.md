# 需求變更：以 GitHub Pages 靜態任務網頁取代每日推播通知

## 變更背景

- 原設計以每日 00:02:00 推播關鍵字清單（Email / Discord）為主要通知方式。
- 實際使用痛點：任務數量多，推播內容為一堆連結清單，用戶點擊某連結跳去 LINE 操作後，
回來無法辨識哪些已完成、哪些尚未點擊，體驗不佳。

---

## 新需求：GitHub Pages 靜態任務網頁

### 核心設計原則

- HTML 殼（`index.html`）永久長駐，只部署一次，後續不再重新 build
- 每日任務資料獨立存為 `tasks.json`，每次 sync 完成後立即更新
- 點擊狀態記錄於瀏覽器 `localStorage`，不需後端
- 通知訊息只需傳送一個網頁 URL，不再傳送任務連結清單

---

## 架構說明

### 靜態檔案結構（GitHub Pages `gh-pages/` 目錄）

```
gh-pages/
  index.html     # 任務網頁殼，只部署一次
  tasks.json     # 當日任務資料，每次 sync 後更新
```

### 資料流程

```
[Sync 排程觸發（00:00:05）]
        ↓
  抓取 API + 解析詳細頁 HTML
        ↓
- **tasks.json 產生**: 實作 `internal/taskpage` 模組，於 sync 完成後將 DB 內當日有效任務序列化並處理分類：
  - 產生 `data/tasks.json`。
  - GitHub Actions 中設定 `git add` 自動偵測 DB 與此檔案更新。
  - `git commit & push`: `data/line_tasks.db`、`data/tasks.json`。
        ↓
  GitHub CDN 更新（約 10~30 秒）
        ↓
  立即觸發 notify Workflow，發送通知（內含網頁 URL）
```

### tasks.json 資料結構

```json
{
  "date": "2026-03-13",
  "generated_at": "2026-03-13T00:00:45+08:00",
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
      "page_url": "https://giftshop.landpress.line.me/checkin_260313-260318/?utm_source=OA&utm_medium=giftshop_collectcard&utm_campaign=traffic_20260313_R2_0000_mass_mk_event_C98_P06_non_checkin_202603"
    },
    {
      "id": "ggzIaIheuWHU0L9fD",
      "title": "好友分享抽好禮",
      "channel_name": "",
      "channel_id": "",
      "type": "share",
      "day": 4,
      "keyword": "",
      "deep_link": "",
      "page_url": "https://event.line.me/..."
    }
  ]
}
```

- date: 當日日期，格式為 YYYY-MM-DD
- generated_at: 產生 tasks.json 的時間，格式為 ISO 8601
- tasks: 任務清單
  - id: 任務 ID，對應 db 的 `activities.id`
  - title: 任務標題，對應 db 的 `activities.title`
  - channel_name: 頻道名稱，對應 db 的 `activities.channel_name`
  - channel_id: 頻道 ID，對應 db 的 `activities.channel_id`
  - type: 任務類型，對應 db 的 `activities.type`
  - day: 任務上架天數，產生 tasks.json 時以 `今日日期 - activities.valid_from` 計算。`0` = 當天上架的新任務（前端顯示 "NEW"），`1` 以上 = 已上架多天的任務（前端顯示 "DAY{day+1}"）
  - keyword: 關鍵字，對應 db 的 `daily_tasks.keyword`，若該 id 有對應的 daily_tasks 則有值，否則為空字串
  - deep_link: deep link，對應 db 的 `daily_tasks.url`，若該 id 有對應的 daily_tasks 則有值，否則為空字串
  - page_url: 頁面 URL，對應 db 的 `activities.page_url`

### index.html 前端行為

```
頁面載入
  → fetch `https://raw.githubusercontent.com/{owner}/{repo}/main/data/tasks.json?t={timestamp}`
  → 與 localStorage 比對當日點擊記錄
  → 渲染任務清單，已點擊項目顯示打勾樣式

點擊任務連結
  → 寫入 localStorage:
      key:   "clicked_{date}"
      value: ["ygzIQIheuHU0L9fC", "nizsaIhg5WHA0L9fE", ...]
  → 該列視覺立即更新為已完成樣式
  → 開啟 "deep_link"（關鍵字、shop_collect 任務）
      或開啟 "page_url"（分享 / 其他任務）

跨日清除
  → 頁面載入時比對 localStorage 內的 date 與今日日期
  → 若不同，清除舊記錄，重新開始
```

### 任務清單呈現格式

```
┌──────────────────────────────────────────┐
│  📅 今日任務  04/12  ·  已完成 3 / 7     │
├──────────────────────────────────────────┤
│  🔑 關鍵字任務                           │
│                                          │
│  ✅ NEW  LINE 購物    SHOP0412   [已完成]     │
│  ☐  DAY2 LINE Pay     PAY0412   [前往 →] │
│  ☐  DAY4 LINE TODAY   TODAY04   [前往 →] │
│                                          │
│  🔗 分享任務                             │
│                                          │
│  ✅ NEW  好友分享抽好禮           [已完成]    │
│  ☐  DAY5 LINE TODAY 分享活動     [前往 →]    │
└──────────────────────────────────────────┘
```

- 顯示今日任務日期為今日日期，例如 04/12
- 顯示已完成任務數量與總任務數量，例如 已完成 3 / 7
- 任務清單依照原先 notify 的任務分類去分類呈現，並依照原先排序邏輯排序顯示
- 顯示任務開始天數：`day` 為 0 時顯示 "NEW"，`day` 為 1 以上時顯示 "DAY{day+1}"（如 day=1 顯示 "DAY2"）；後接任務頻道名稱、任務標題、任務關鍵字（若有）
- 有 deep_link 的任務顯示 "前往 →"，點擊後開啟 deep_link
- 沒有 deep_link 的任務顯示 "前往 →"，點擊後開啟 page_url

---

## 通知訊息變更

### 原設計

```
每日 00:02:00 Cloud Scheduler 觸發 notify workflow，推播包含所有任務連結的完整清單訊息
```

### 新設計

```
sync 完成後立即觸發 notify workflow，依照配置檔的通知通道發送通知：

📅 [日期] 任務清單已更新，共 N 項
👉 https://{username}.github.io/{repo-name}/
```

推播時機由「固定 00:02:00」改為「每次 sync 完成且 tasks.json 有實際異動時」發送。
若 sync 後內容與上次相同（hash 未變），則不重複推播。

---

## GitHub Actions 新增 Workflow

### deploy.yml（新增）

```
觸發條件：當 gh-pages/index.html 有異動或 repository_dispatch 觸發
執行內容：
  1. checkout repo
  2. Upload artifacts，上傳 "gh-pages/" 目錄與其下檔案
  3. 部署 github pages
```

---

## 部署速度說明

| 更新項目                 | 部署方式           | 生效時間                     |
| ------------------------ | ------------------ | ---------------------------- |
| `tasks.json`（每日資料） | 直接 push 到 repo  | 約 10~30 秒（透過 CDN 生效） |
| `index.html`（網頁殼）   | GitHub Pages build | 約 1.5~4 分鐘                |

`tasks.json` 為純靜態檔案且留在 repo 中不隨 Pages 部署，由前端直接向 `raw.githubusercontent.com` 取得，
因此 sync 完成後推播通知的當下即可被順利讀取，前端自帶 `?t={timestamp}` 繞過 CDN 快取。
