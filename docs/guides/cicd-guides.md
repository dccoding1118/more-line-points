# GitHub 配置指南

> 以下為在 GitHub 上啟用 CI/CD 所需的手動配置步驟，請依序完成。

## 1. 設定 GitHub Secrets

前往 **GitHub Repo → Settings → Secrets and variables → Actions → New repository secret**，依序新增：

| 順序 | Secret 名稱                 | 值來源                                                |
| ---- | --------------------------- | ----------------------------------------------------- |
| 1    | `DISCORD_BOT_TOKEN`         | 從本機 `.env` 中的 `DISCORD_BOT_TOKEN` 值複製         |
| 2    | `DISCORD_GUILD_ID`          | 從本機 `.env` 中的 `DISCORD_GUILD_ID` 值複製          |
| 3    | `DISCORD_NOTIFY_CHANNEL_ID` | 從本機 `.env` 中的 `DISCORD_NOTIFY_CHANNEL_ID` 值複製 |
| 4    | `DISCORD_ADMIN_CHANNEL_ID`  | 從本機 `.env` 中的 `DISCORD_ADMIN_CHANNEL_ID` 值複製  |
| 5    | `GMAIL_CREDENTIALS_JSON`    | 將本機 `credentials.json` 檔案**完整內容**貼入        |
| 6    | `GMAIL_TOKEN_JSON`          | 將本機 `token.json` 檔案**完整內容**貼入              |

> **操作提示**：可使用 `cat credentials.json | pbcopy`（macOS）或 `cat credentials.json | xclip`（Linux）快速複製檔案內容。

## 2. 確認 Workflow 權限

前往 **GitHub Repo → Settings → Actions → General → Workflow permissions**：

1. 選擇 **Read and write permissions**（sync.yml 需要 push 權限）
2. 勾選 **Allow GitHub Actions to create and approve pull requests**（可選，目前不需要）
3. 點擊 **Save**

> `GITHUB_TOKEN` 為 GitHub 自動注入，安全且無需額外管理，不必另建 PAT。

## 3. 確認 Branch Protection（選配）

若 `main` 分支已啟用 **Branch Protection Rules**，需確認以下設定：

- **Require status checks to pass before merging**：勾選，並加入 `CI` check（ci.yml 的 job name）
- **Include administrators**：視需求決定是否對管理者也強制執行
- **Allow force pushes**：**不建議**開啟（sync commit-back 使用一般 push）

> 如果尚未設定 Branch Protection，可於 CI 驗證通過後再啟用。

## 4. 驗證 Secrets 設定正確性

在 **GitHub Repo → Settings → Secrets and variables → Actions** 頁面確認 6 個 Secrets 都已新增，Secret 名稱無拼字錯誤。

> Secret 值新增後無法再查看內容，若不確定是否正確可選擇 **Update** 重新貼入。

---

## 5. Git 分支策略

### 5.1 Patch 4 開發流程

```mermaid
%%{init: {'theme':'dark'}}%%
gitGraph
    commit id: "Patch 3 done"
    branch feat/github-cicd
    checkout feat/github-cicd
    commit id: "feat: add composite action setup-go"
    commit id: "feat: add ci.yml workflow"
    commit id: "feat: add sync.yml workflow"
    commit id: "feat: add notify.yml workflow"
    commit id: "docs: update AGENTS.md with CI/CD info"
    checkout main
    merge feat/github-cicd id: "merge PR #N" tag: "patch4"
    commit id: "chore(data): auto-sync" type: HIGHLIGHT
```

### 5.2 分支命名與操作步驟

1. **從 `main` 切出開發分支**：
   ```bash
   git checkout main
   git pull origin main
   git checkout -b feat/github-cicd
   ```

2. **按 TDD 順序提交**（每個 Workflow 一個 commit）：
   ```bash
   # Step 1: Composite Action
   git add .github/actions/
   git commit -m "feat: add composite action setup-go"

   # Step 2: CI Pipeline
   git add .github/workflows/ci.yml
   git commit -m "feat: add ci.yml with lint, test, and build"

   # Step 3: Sync Workflow
   git add .github/workflows/sync.yml
   git commit -m "feat: add sync.yml with cron schedule and db commit-back"

   # Step 4: Notify Workflow
   git add .github/workflows/notify.yml
   git commit -m "feat: add notify.yml with cron schedule and date input"

   # Step 5: Documentation
   git add AGENTS.md docs/
   git commit -m "docs: update AGENTS.md and add design-cicd.md"
   ```

3. **Push 並開 PR**：
   ```bash
   git push -u origin feat/github-cicd
   ```
   - 於 GitHub 開啟 PR：`feat/github-cicd` → `main`
   - PR Title：`feat: GitHub Actions CI/CD 與排程自動化`
   - 此時 `ci.yml` 會自動觸發 CI 檢查（第一次驗證）

4. **CI 通過後合併**：
   - 確認 CI 全綠（lint + unit test + integration test + build）
   - 使用 **Squash and merge** 或 **Create a merge commit**（依團隊慣例）
   - 合併後 `sync.yml` 與 `notify.yml` 的 Cron 排程開始生效

5. **合併後驗證**：
   - 手動觸發 `sync.yml`（§8 驗證步驟 2）
   - 手動觸發 `notify.yml`（§8 驗證步驟 3）
   - 確認排程 Cron 正常執行（可於隔日檢查 Actions 歷史）

---

## 6. Gmail OAuth Production Mode 設定

Gmail API 的 OAuth consent screen 必須切換至 **Production Mode**，否則 refresh token 每 7 天過期、導致 CI 中 token 失效需重新授權。

### 6.1 切換步驟

1. 前往 [Google Cloud Console](https://console.cloud.google.com/) → **APIs & Services** → **OAuth consent screen**
2. 點擊 **PUBLISH APP**，將狀態從 "Testing" 改為 "In Production"
3. 本專案僅使用 `gmail.send` scope（非敏感 scope），Google **不需要額外審查**，可直接 publish

### 6.2 重新產生 Token

切換後需重新授權以取得不過期的 refresh token：

```bash
# 刪除舊 token
rm token.json

# 重新執行 notify 觸發授權流程
./bin/scheduler notify --config config/config.yaml
# 瀏覽器開啟授權頁面 → 授權 → 輸入驗證碼 → 產生新 token.json
```

### 6.3 更新 GitHub Secret

將新的 `token.json` 內容更新至 GitHub Secret：

1. `cat token.json` 複製完整內容
2. 前往 **GitHub Repo → Settings → Secrets → Actions**
3. 更新 `GMAIL_TOKEN_JSON` 的值

---

## 7. GCP Cloud Scheduler 設定

GitHub Actions 的 Cron 排程不保證準時（高峰時段延遲可達 30–60 分鐘），使用 **GCP Cloud Scheduler** 外部觸發 `workflow_dispatch` 以確保 Notify 準時推播。

### 7.1 建立 Fine-grained PAT

1. 前往 [GitHub Settings → Developer Settings → Fine-grained tokens](https://github.com/settings/personal-access-tokens)
2. 點擊 **Generate new token**，設定以下參數：

| 設定項                | 值                                     |
| --------------------- | -------------------------------------- |
| **Token name**        | `gcp-scheduler-notify`                 |
| **Expiration**        | 90 天                                  |
| **Repository access** | `Only select repositories` → 僅本 repo |
| **Permissions**       | `Actions: Read and write`              |

3. 點擊 **Generate token** 並複製 token 值（僅顯示一次）

> ⚠️ **定期輪替**：PAT 到期前需重新建立並更新 GCP Cloud Scheduler 中的 Header 值。建議設定日曆提醒。

### 7.2 建立 Cloud Scheduler Job

1. 前往 [GCP Console → Cloud Scheduler](https://console.cloud.google.com/cloudscheduler)
2. 點擊 **CREATE JOB**，設定以下參數：

| 設定項          | 值                                                                                    |
| --------------- | ------------------------------------------------------------------------------------- |
| **Name**        | `trigger-notify-workflow`                                                             |
| **Region**      | `asia-east1`（或就近區域）                                                            |
| **Frequency**   | `50 23 * * *`（台灣時間 23:50，GCP 支援時區設定）                                     |
| **Timezone**    | `Asia/Taipei`                                                                         |
| **Target type** | `HTTP`                                                                                |
| **URL**         | `https://api.github.com/repos/{OWNER}/{REPO}/actions/workflows/notify.yml/dispatches` |
| **Method**      | `POST`                                                                                |
| **Body**        | `{"ref":"main"}`                                                                      |

3. 設定 HTTP Headers：

```
Authorization: Bearer {YOUR_FINE_GRAINED_PAT}
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
Content-Type: application/json
```

4. 點擊 **CREATE** 完成建立

### 7.3 驗證

1. 在 Cloud Scheduler 列表頁面，點擊 **Force Run** 手動測試
2. 檢查 GitHub Actions 頁面，確認 `notify.yml` 被 `workflow_dispatch` 事件觸發
3. 確認 Notify 執行成功（Discord 與 Email 收到推播）
