# Part 4 細部設計：Patch 3 — 推播通知（Notify）

> **對應開發階段**：Patch 3
> **前置條件**：Patch 2 完成（`activities` 與 `daily_tasks` 有解析並寫入正確資料）。
> **驗收標準**：執行 `scheduler notify` 後，Discord 與 Email 收到按設計格式化正確的訊息，深層連結 (Deep link) 可點擊並開啟 LINE。

---

## 1. 範圍與目標

### 交付物

| 項目                  | 說明                                                                                                                       |
| --------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| 通知格式規範          | 確立 Discord Markdown 解析模式與 Email MIME 格式化之明確排版與降級訊息呈現，針對**每一種任務類型**列出獨立章節與完整範例。 |
| storage 擴充          | 僅擴充 `ActivityStore.GetActivitiesByDate` 單表 CRUD 選取，將「組合邏輯」移交給 Notify 服務層。                            |
| discord 模組（新增）  | 封裝 `bwmarrin/discordgo` 套件，實作推播發送 (`ChannelMessageSend`)。                                                      |
| email 模組（新增）    | 實作 Gmail API 發送 HTML 信件 (OAuth2 授權)。                                                                              |
| notify 服務層（新增） | 核心業務模組（Service Layer），負責跨 Storage 查詢、記憶體組裝、套用 ChannelMapping 降級及推播排版與派發。                 |
| CLI 命令擴充          | 新增 `scheduler notify` 指令，負責參數解析與依賴注入。                                                                     |

### 設計指導原則

- **關注點分離**：`storage` 僅負責對應資料表的直接 CRUD；複雜的資料過濾、業務判斷（如日期邊界、找尋有無對應任務）與格式化組裝，全由 `notify` 服務層負責。
- **行為契約核心**：所有模組的開發與單元測試，皆嚴格遵守文件內明列的正面、反面與邊界行為契約。

---

## 2. 推播訊息格式與降級規範

為使每種任務類型 (`type`) 操作意義明確，推播內容需將 **8 種已知支援的任務類型** 各自分獨立章節呈現。不將它們合併，以確保使用者一眼能區分需要輸入關鍵字的、需收藏店家的、需投票的或是直接點擊的。

支援的任務類型對應章節大綱如下：
1. `keyword`：🔑 關鍵字任務
2. `shop-collect`：🛍️ 收藏指定店家
3. `lucky-draw`：🎁 點我試手氣
4. `poll`：🗳️ 投票任務
5. `app-checkin`：📱 App 簽到任務
6. `passport`：📗 購物護照任務
7. `share`：🔗 分享好友任務
8. `other`：📌 其他任務

> 註：`unknown` 類型代表尚未被解析，不應出現在推播清單內。

### 2.1 Discord Markdown 格式範例

Discord 支援標準 Markdown，包含 `**` (粗體)、`*` (斜體)、隱藏文字以及 `[名稱](網址)` 的超連結格式。每個存在任務的類型皆有獨立區塊；若該類型當日無任務，則該區塊不顯示。

```markdown
**📅 03/05 LINE 任務清單**

**🔑 關鍵字任務**
• [LINE 購物: 輸入 0305關鍵字](https://line.me/R/oaMessage/@lineshopping/?0305%E9%97%9C%E9%8D%B5%E5%AD%97)
• ⚠️ LINE Pay: 輸入 0305PAY (需手動前往頻道)

**🛍️ 收藏指定店家**
• [某某品牌館](https://buy.line.me/u/partner/xxx)

**🎁 點我試手氣**
• [週三輪盤抽獎](https://event.line.me/r/xxx)

**🗳️ 投票任務**
• [年度愛用品投票](https://event.line.me/poll/xxx)

**📱 App 簽到任務**
• [LINE 購物 App 簽到](https://buy.line.me/xxx)

**📗 購物護照任務**
• [開通護照拿點數](https://ec-bot-web.line.me/xxx)

**🔗 分享好友任務**
• [好友分享抽好禮](https://event.line.me/s/xxx)

**📌 其他任務**
• [集點卡活動](https://event.line.me/xxx)
```

### 2.2 Email HTML 格式範例

Email 支援完整的 HTML。我們使用 `<ul>` 和 `<li>` 排版清單，並加上適度樣式。

```html
<h2>📅 03/05 LINE 任務清單</h2>

<h3>🔑 關鍵字任務</h3>
<ul>
  <li><a href="https://line.me/R/oaMessage/@lineshopping/?0305%E9%97%9C...%">LINE 購物: 輸入 0305關鍵字</a></li>
  <li>⚠️ LINE Pay: 輸入 0305PAY (需手動前往頻道)</li>
</ul>

<h3>🛍️ 收藏指定店家</h3>
<ul>
  <li><a href="https://buy.line.me/u/partner/xxx">某某品牌館</a></li>
</ul>

<h3>🎁 點我試手氣</h3>
<ul>
  <li><a href="https://event.line.me/r/xxx">週三輪盤抽獎</a></li>
</ul>

<h3>🗳️ 投票任務</h3>
<ul>
  <li><a href="https://event.line.me/poll/xxx">年度愛用品投票</a></li>
</ul>

<h3>📱 App 簽到任務</h3>
<ul>
  <li><a href="https://buy.line.me/xxx">LINE 購物 App 簽到</a></li>
</ul>

<h3>📗 購物護照任務</h3>
<ul>
  <li><a href="https://ec-bot-web.line.me/xxx">開通護照拿點數</a></li>
</ul>

<h3>🔗 分享好友任務</h3>
<ul>
  <li><a href="https://event.line.me/s/xxx">好友分享抽好禮</a></li>
</ul>

<h3>📌 其他任務</h3>
<ul>
  <li><a href="https://event.line.me/xxx">集點卡活動</a></li>
</ul>
```

### 2.3 ChannelMapping 降級策略

當 `notify` 服務層在組裝 Deep Link (`https://line.me/R/oaMessage/{channel_id}/?{keyword}`) 時（僅針對 `keyword` 等需依賴 channel 名稱轉換為頻道 ID 的類型），需呼叫 `config.ChannelMapping.LookupChannelID(channelName)`。若找不到：
- **`on_missing: "warn"`（預設）**：記錄 warning log，推播內容顯示為純文字（無 `<a href>`），文字前綴加上 `⚠️`，並提示 `(需手動前往頻道)`。
- **`on_missing: "skip"`**：記錄 warning log，此筆活動在該日推播清單中直接剔除（不呈現）。
- **`on_missing: "error"`**：記錄 error log，中斷整個 Notify 流程，不發送任何通知。

---

## 3. 模組分解與細部設計

### 3.1 storage 擴充

**職責**：僅對 `activities` 表進行單表資料條件檢索，不包含與 `daily_tasks` 的 JOIN 操作。

#### 介面設計
```go
// ActivityStore 擴充
type ActivityStore interface {
    // ...既有方法...
    // GetActivitiesByDate 取得指定日期內所有 is_active=1 且滿足有效日期的活動
    GetActivitiesByDate(ctx context.Context, targetDate time.Time) ([]model.Activity, error)
}
```

#### 行為契約 (`GetActivitiesByDate`)

| #   | 行為分類 | 觸發條件                                                    | 預期結果                                         |
| --- | -------- | ----------------------------------------------------------- | ------------------------------------------------ |
| S1  | 正面     | 活動 `is_active=1` 且 `valid_from <= target <= valid_until` | 成功回傳該活動物件，陣列包含此項目。             |
| S2  | 反面     | 活動已過期 (`valid_until < target`)                         | 陣列中不包含此活動。                             |
| S3  | 反面     | 活動尚未開始 (`valid_from > target`)                        | 陣列中不包含此活動。                             |
| S4  | 反面     | 活動被標記停用 (`is_active=0`)                              | 無論日期為何，陣列中均不包含此活動。             |
| S5  | 邊界     | 資料庫查無任何符合條件的活動                                | 回傳空陣列 `[]model.Activity{}` 及 `nil` error。 |

> 註：`DailyTaskStore.GetDailyTasksByDate(ctx, targetDate)` 在 Patch 2 已存在，可直接沿用以撈取目標日所有任務。

---

### 3.2 discord 模組

**職責**：封裝 `bwmarrin/discordgo`，負責對 Discord 發送推播訊息。

#### 介面設計
```go
package discord

type Sender interface {
    SendMessage(ctx context.Context, text string) error
}

// 內部會初始化 discordgo.Session
func NewSender(botToken, notifyChannelID string) (Sender, error)
```

#### 行為契約 (`SendMessage`)

| #   | 行為分類 | 觸發條件                            | 預期結果                                            |
| --- | -------- | ----------------------------------- | --------------------------------------------------- |
| T1  | 正面     | 網路正常，`ChannelMessageSend` 成功 | 成功不報錯 (回傳 `nil`)。                           |
| T2  | 反面     | API 呼叫失敗 (如權限不足或限流)     | 回傳 error 並包含 discordgo 的錯誤訊息。            |
| T3  | 反面     | 連線逾時或網路錯誤                  | 透過 context 取消或 timeout，回傳對應的底層 error。 |
| T4  | 邊界     | 傳入的 `text` 為空字串              | (可選) 提早防禦回傳錯誤，不去呼叫 API。             |

---

### 3.3 email 模組

**職責**：單純的發送代理，負責對 Gmail API 送出 MIME 格式信件。

#### 介面設計
```go
package email

type Sender interface {
    SendHTML(ctx context.Context, subject, htmlBody string) error
}

func NewSender(credentialsPath, tokenPath, senderMail string, recipients []string) Sender
```

#### 行為契約 (`SendHTML`)

| #   | 行為分類 | 觸發條件                            | 預期結果                                                                   |
| --- | -------- | ----------------------------------- | -------------------------------------------------------------------------- |
| E1  | 正面     | 登入與傳輸正常                      | 組裝合法 MIME 特徵 (`Content-Type: text/html`) 且順利呼叫 Gmail API 發送。 |
| E2  | 反面     | 登入認證失敗 (如 Token 失效/未授權) | 觸發 OAuth 授權流程或回傳 Auth error。                                     |
| E3  | 邊界     | Recipients (收件人陣列) 為空        | 提早防禦回傳 error，不發送。                                               |
| E4  | 邊界     | Subject 包含非 ASCII 字元           | 必須正確使用 `mime.QEncoding` 編碼為 `=?utf-8?q?...?=`，避免亂碼。         |

---

### 3.4 notify 服務層（核心）

**職責**：控制資料獲取、邏輯過濾、依照 `ChannelMapping` 解析字串、產生平台對應的 HTML 報告，並派發給多個發送源。

#### 介面設計
```go
package notify

type Notifier struct {
    activityStore storage.ActivityStore
    taskStore     storage.DailyTaskStore
    dcSender      discord.Sender       // 允許為 nil (設定檔未啟用時)
    emailSender   email.Sender         // 允許為 nil (設定檔未啟用時)
    mapping       *config.ChannelMapping
}

func NewNotifier(as storage.ActivityStore, ts storage.DailyTaskStore, dc discord.Sender, em email.Sender, cm *config.ChannelMapping) *Notifier

// Run 執行完整推播資料流
func (n *Notifier) Run(ctx context.Context, targetDate time.Time) error
```

#### 資料流程 (Data Flow) 詳解

當呼叫 `Notifier.Run(ctx, targetDate)` 時，依以下步驟執行：

1. **獲取有效活動**：
   - 呼叫 `activityStore.GetActivitiesByDate(ctx, targetDate)` 取得 `[]model.Activity`。
2. **獲取目標日任務**：
   - 呼叫 `taskStore.GetDailyTasksByDate(ctx, targetDate)` 取得 `[]model.DailyTask`，並轉為以 `activity_id` 為鍵的 `map[string]model.DailyTask`。
3. **過濾與歸類 (業務邏輯)**：
   - 建立資料結構存放所有 8 大獨立分類 (`keyword`, `share`, `passport`, `shop-collect`, `lucky-draw`, `app-checkin`, `poll`, `other`)。
   - 遍歷每一筆 `Activity`：
     - 若 `type == "keyword"`：確保 task map 中有任務，無則忽略；有則歸入「關鍵字任務」。
     - 若 `type == "shop-collect"`：確保 task map 中有任務，無則忽略；有則歸入「收藏指定店家」。
     - 若 `type` 屬於其他單次行動 (例如 `share`, `passport` 等)：直接取 `activity.ActionURL`，歸入各自對應分類。
     - 若 `type == "unknown"` 則略過。
4. **準備呈現項目與降級處理**：
   - 對於 `keyword` 等類別中需要組裝 LINE Deep Link 的項目，呼叫 `mapping.LookupChannelID(channelName)`。
   - 因應 LINE deep link 的要求，keyword 中的空格 (` `) 必須編碼為 `%20` 而非 `+`，需要使用 `strings.ReplaceAll(url.QueryEscape(), "+", "%20")` 進行轉換。
   - 處理降級 (`error` -> 中斷流程回傳 error；`skip` -> 剔除；`warn` -> 標記為純文字模式並加 `⚠️`)。
5. **格式化報告 (Formatter)**：
   - 若過濾後全部分類皆空，準備「當日無任務」訊息。
   - 否則，分別呼叫內部的 DiscordFormatter 產出 `discordMsg` (節 2.1 格式) 與 EmailFormatter 產出 `emailHTML` (節 2.2 格式)。
6. **派發推播**：
   - 若 `dcSender != nil`，呼叫 `dcSender.SendMessage(ctx, discordMsg)`。
   - 若 `emailSender != nil`，呼叫 `emailSender.SendHTML(ctx, subject, emailHTML)`。
   - 若兩者其一出錯，記錄 error log，不阻斷另一端的派發。

#### 行為契約 (`Run`)

| #   | 行為分類 | 觸發條件                                             | 預期結果                                                              |
| --- | -------- | ---------------------------------------------------- | --------------------------------------------------------------------- |
| N1  | 正面     | 撈取到混雜不同 `type`，且任務對應完整。              | 成功針對不同平台呼叫 format，並正確調度 Sender 發送而後回傳 `nil`。   |
| N2  | 反面     | `keyword` 或 `shop-collect` 活動缺少當日 `DailyTask` | 該活動**不包含**於最終推播清單內。                                    |
| N3  | 邊界     | 儲存層回報 0 筆活動，或過濾後皆無任務                | 發送「當日無需執行任務」的空報表給啟用的 Sender。                     |
| N4  | 正面反面 | `LookupChannelID` 失敗且 `OnMissing="skip"`          | 組裝清單時直接從中捨棄該活動。                                        |
| N5  | 正面反面 | `LookupChannelID` 失敗且 `OnMissing="warn"`          | 此條目以純文字且加上 ⚠️ 前綴與提示呈現於訊息中。                       |
| N6  | 反面     | `LookupChannelID` 失敗且 `OnMissing="error"`         | `Run()` 直接回傳 error，並確保完全不觸發任何 Sender。                 |
| N7  | 邊界     | `dcSender` 或 `emailSender` 配置為 `nil`             | 只觸發非 `nil` 的 Sender，不發生 Panic；兩者皆 `nil` 則直接回傳成功。 |

---

### 3.5 cmd/scheduler/cli/notify

**職責**：接入子指令、解析參數、載入 Config、實例化各依賴元件並啟動 `notifier.Run()`。

#### 介面與流程
```go
var notifyCmd = &cobra.Command{
    Use:   "notify",
    Short: "Push daily tasks to Discord and/or Email",
    RunE: func(cmd *cobra.Command, args []string) error {
        // 1. 解析 `--date` (預設為 time.Now().AddDate(0, 0, 1))
        // 2. config.Load(...) & config.LoadChannelMapping(...)
        // 3. sqlite.NewStore(...)
        // 4. if cfg.Discord.Enabled -> dc, err := discord.NewSender(...)
        // 5. if cfg.Email.Enabled -> email.NewSender(...)
        // 6. n := notify.NewNotifier(store, store, dc, em, mapping)
        // 7. return n.Run(ctx, targetDate)
    },
}
```

---

## 4. TDD 開發順序

| 步驟 | 模組             | 🔴 RED (測項對應上述行為)   | 🟢 GREEN (實作目標)                                                             | 🔵 REFACTOR                          |
| ---- | ---------------- | -------------------------- | ------------------------------------------------------------------------------ | ----------------------------------- |
| 1    | `storage`        | §3.1 (S1~S5)               | 實作 `GetActivitiesByDate`。                                                   | 抽離 SQL Const。                    |
| 2    | `discord, email` | §3.2 (T1~T4), §3.3 (E1~E4) | 實作單純的 API 發送，Mock HTTP/Gmail 行為，不涉及任何商業邏輯。                | 建立共通 HTTP Client / Retry 設定。 |
| 3    | `notify`         | §3.4 (N1~N7)               | 實作核心過濾歸類與雙格式 Formatter。撰寫 Table-Driven Tests 確保邏輯判定無誤。 | 分解 Formatter 與 Classifier 職責。 |
| 4    | `cmd`            | Flag `--date` 解析驗證     | 實作依賴注入建立實體與觸發邏輯。                                               | 把依賴注入抽成共用 Constructor。    |

> 預估新增單元測試 **20 個** (storage:5, discord:4, email:4, notify:7)，完全符合 Patch 規模要求 (≤50 個測項)。

---

## 5. 驗收標準

| 項目               | 驗收操作與方式                                            | 通過條件                                                                              |
| ------------------ | --------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| 單元測試           | `mise run test`                                           | 新增之測試全部通過，模組隔離明確，整體專案覆蓋率 ≥ **90%**                            |
| Lint               | `mise run lint`                                           | golangci-lint 穩定零 warning 通過。                                                   |
| 介面契約           | 審查程式碼呼叫關係                                        | 商業資料過濾、Mapping 與組裝操作全部收斂於 `notify` package 內。                      |
| CLI 本機驅動驗證   | `scheduler notify --date 2026-03-05` (換成 DB 有資料日期) | 資料庫完備且各參數正確下，無報錯成功完成。(含警告測試情境)                            |
| 真機驗證 (Discord) | 執行 notify 命令後，登入 Discord 觀測。                   | 收到 Markdown 格式正確字串，版面清晰可見，並區分 8 大任務區塊，deep link 可正常觸發。 |
| 真機驗證 (Email)   | 執行 notify 命令後，登入指定的 Email 帳號觀測。           | 收到完整 HTML 呈現的列表信件 (包含 8 大類 `<ul>` 清單)，主旨與分類清楚，URL 可點擊。  |
