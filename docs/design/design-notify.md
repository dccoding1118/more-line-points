# Part 4 細部設計：Patch 3 — 推播通知（Notify）

> **對應開發階段**：Patch 3
> **前置條件**：Patch 2 完成（`activities` 與 `daily_tasks` 有解析並寫入正確資料）。
> **驗收標準**：執行 `scheduler notify` 後，Discord 與 Email 收到按設計格式化正確的訊息，深層連結 (Deep link) 可點擊並開啟 LINE。

---

## 1. 範圍與目標

### 交付物

| 項目                  | 說明                                                                                                       |
| --------------------- | ---------------------------------------------------------------------------------------------------------- |
| 通知格式規範          | 確立 Discord Markdown 解析模式與 Email HTML 格式化之明確排版。現已簡化為僅發送「本週總數」與「網頁連結」。 |
| storage 擴充          | (已轉移至 TaskPage 依賴，`notify` 模組僅單純查詢筆數)。                                                    |
| discord 模組（新增）  | 封裝 `bwmarrin/discordgo` 套件，實作推播發送 (`ChannelMessageSend`)。                                      |
| email 模組（新增）    | 實作 Gmail API 發送 HTML 信件 (OAuth2 授權)。                                                              |
| notify 服務層（新增） | 核心業務模組（Service Layer），負責查詢當日共計幾項任務，並將推播網址派發與 Discord 與 Email。             |
| CLI 命令擴充          | 新增 `scheduler notify` 指令，負責參數解析與依賴注入。                                                     |

### 設計指導原則

- **極簡化通知**：推播的責任已從顯示一切詳細內容，轉為提醒使用者前往 `GitHub Pages` 首頁執行，大幅降低洗版問題。
- **關注點分離**：排版與過濾已轉往靜態首頁，`notify` 完全不須經手字串處理與 `Deep Link` 生成。

---

## 2. 推播訊息格式

為避免群組或信箱被洗版，影響重要訊息傳遞，推播內容僅包含**活動日期**、**任務總篇數**，以及前往**任務首頁的 URL**。
這可以讓使用者自由選擇時間開啟首頁來完成。

### 2.1 Discord Markdown 格式範例

```markdown
📅 **04/12 LINE 任務清單已更新**
共有 7 項任務等待完成！

👉 [點我前往任務首頁](https://dccoding1118.github.io/more-line-points/)
```

### 2.2 Email HTML 格式範例

```html
<h2>📅 04/12 LINE 任務清單已更新</h2>
<p>共有 7 項任務等待完成！</p>
<br/>
<p>👉 <a href="https://dccoding1118.github.io/more-line-points/">點我前往任務首頁</a></p>
```

---

## 3. 模組分解與細部設計

### 3.1 storage 擴充

*(因架構變更，`activityStore` 等所有邏輯判斷與組合已移交給 `TaskPage` 模組，`notify` 模組目前僅需使用既有之 `DailyTaskStore` 查詢筆數即可，本節廢除。)*

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

**職責**：統計當日的任務長度，並夾帶 config 中的 `github_pages_url` 派發給多個發送源。

#### 介面設計
```go
package notify

type Notifier struct {
    taskStore     storage.DailyTaskStore
    dcSender      discord.Sender       // 允許為 nil (設定檔未啟用時)
    emailSender   email.Sender         // 允許為 nil (設定檔未啟用時)
    pagesURL      string               // 取自 config.TaskPage.GithubPagesURL
}

func NewNotifier(ts storage.DailyTaskStore, dc discord.Sender, em email.Sender, pagesURL string) *Notifier

// Run 執行完整推播資料流
func (n *Notifier) Run(ctx context.Context, targetDate time.Time) error
```

#### 資料流程 (Data Flow) 詳解

當呼叫 `Notifier.Run(ctx, targetDate)` 時，依以下步驟執行：

1. **獲取任務計數**：
   - 呼叫 `taskStore.GetDailyTasksByDate(ctx, targetDate)` 取得該當日的所有任務 `[]model.DailyTask`。
2. **格式化報告 (Formatter)**：
   - 若任務數量為 0，組裝「當日無任務」訊息。
   - 否則，將數量與 `n.pagesURL` 組合成 DiscordMsg (節 2.1) 與 EmailHTML (節 2.2)。
3. **派發推播**：
   - 若 `dcSender != nil`，呼叫 `dcSender.SendMessage(ctx, discordMsg)`。
   - 若 `emailSender != nil`，呼叫 `emailSender.SendHTML(ctx, subject, emailHTML)`。
   - 若兩者其一出錯，記錄 error log，不阻斷另一端的派發。

#### 行為契約 (`Run`)

| #   | 行為分類 | 觸發條件                                 | 預期結果                                                              |
| --- | -------- | ---------------------------------------- | --------------------------------------------------------------------- |
| N1  | 正面     | 當日有多項任務。                         | 成功發送總任務數與 URL 並回傳 `nil`。                                 |
| N2  | 邊界     | 儲存層回報 0 筆任務                      | 發送「當日無任務」的訊息給啟用的 Sender。                             |
| N3  | 邊界     | `dcSender` 或 `emailSender` 配置為 `nil` | 只觸發非 `nil` 的 Sender，不發生 Panic；兩者皆 `nil` 則直接回傳成功。 |

---

### 3.5 cmd/scheduler/cli/notify

**職責**：接入子指令、解析參數、載入 Config、實例化各依賴元件並啟動 `notifier.Run()`。

#### 介面與流程
```go
var notifyCmd = &cobra.Command{
    Use:   "notify",
    Short: "Push daily tasks to Discord and/or Email",
    RunE: func(cmd *cobra.Command, args []string) error {
        // 1. config.Load(...)
        // 2. sqlite.NewStore(...)
        // 3. if cfg.Discord.Enabled -> dc, err := discord.NewSender(...)
        // 4. if cfg.Email.Enabled -> email.NewSender(...)
        // 5. n := notify.NewNotifier(store, dc, em, cfg.TaskPage.GithubPagesURL)
        // 6. return n.Run(ctx, time.Now())
    },
}
```

---

## 4. TDD 開發順序

| 步驟 | 模組             | 🔴 RED (測項對應上述行為)   | 🟢 GREEN (實作目標)                                               | 🔵 REFACTOR                          |
| ---- | ---------------- | -------------------------- | ---------------------------------------------------------------- | ----------------------------------- |
| 1    | `discord, email` | §3.2 (T1~T4), §3.3 (E1~E4) | 實作單純的 API 發送，Mock HTTP/Gmail 行為，不涉及任何商業邏輯。  | 建立共通 HTTP Client / Retry 設定。 |
| 2    | `notify`         | §3.4 (N1~N3)               | 實作計數與字串 Formatter。撰寫 Table-Driven Tests 確保發送正確。 | 分離 Sender Error Logging 邏輯。    |
| 3    | `cmd`            | Flag `--config` 解析驗證   | 實作依賴注入建立實體與觸發邏輯。                                 | 把依賴注入抽成共用 Constructor。    |

---

## 5. 驗收標準

| 項目               | 驗收操作與方式                                  | 通過條件                                 |
| ------------------ | ----------------------------------------------- | ---------------------------------------- |
| 單元測試           | `mise run test`                                 | 新增之測試全部通過，整體覆蓋率維持。     |
| Lint               | `mise run lint`                                 | golangci-lint 穩定零 warning 通過。      |
| CLI 本機驅動驗證   | `scheduler notify`                              | 無報錯成功完成。(含警告測試情境)         |
| 真機驗證 (Discord) | 執行 notify 命令後，登入 Discord 觀測。         | 收到含任務數量總數、前往連結的簡短通知。 |
| 真機驗證 (Email)   | 執行 notify 命令後，登入指定的 Email 帳號觀測。 | 收到包含前往連結 URL 的 HTML 信件。      |
