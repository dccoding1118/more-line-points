# STATUS — more-line-points

> 工作階段交接檔。專案理解（核心/流程/結構/索引/雷）看 AGENTS.md；需求看 docs/PRD；設計看 docs/design。
> 這裡只放「沒寫進 docs/碼會遺失」的兩件事，每回合就地增刪。commit 進本 repo。最後更新：2026-07-20

## 1. 未歸檔結論

- Branch protection／PR review 缺失：經評估非立即風險（repo 只有 owner 一名 collaborator，push 權限本就綁 collaborator 名單，與是否設定 branch protection 無關）；長期仍建議補上（禁止 force-push、要求 PR review），作為帳號/PAT 外洩或自身誤操作時的第二道防線。暫緩處理，無對應 docs 可歸檔，故留此追蹤。

## 2. 未完成任務

- [ ] 開啟 GitHub repo 的 secret scanning + push protection（Settings → Code security and analysis）。現況：兩者皆為 disabled；repo 為 public 且 Actions secrets 內有 GMAIL_CREDENTIALS_JSON／GMAIL_TOKEN_JSON／DISCORD_BOT_TOKEN 等真實憑證，優先度最高、成本最低。
- [ ] 修正 `gh-pages/index.html` 的 XSS 隱患：`render()` 內用 `itemEl.innerHTML` 直接插入 `task.title`／`descText`（含 `channel_name`、`keyword`）等未跳脫欄位，資料來源（`data/tasks.json`，爬 LINE 官方活動頁而來）一旦被污染即可執行任意 JS。改用 `textContent` 賦值或對插入字串做 HTML escape。
- [ ] 開啟 GitHub repo 的 Dependabot / vulnerability alerts（Settings → Code security and analysis），目前為 disabled，go.sum 相依套件的已知 CVE 不會被通知。
- [ ] 將 repo 層級 Actions 預設 workflow permissions（Settings → Actions → General）由 `write` 改為 `read`。現況各 workflow 檔已各自明確宣告 `permissions:` block（多數 read-only 或最小權限）蓋過此預設值，故非緊急，屬保險措施，避免未來新增 workflow 忘記宣告時繼承過寬權限。
