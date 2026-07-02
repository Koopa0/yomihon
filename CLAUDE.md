# kurodo（蔵人）

這份檔案存在，是因為 LLM 在這個 repo 會犯的錯是**可預測的**——不是隨機的錯，是同樣的錯，一犯再犯。以下不是建議，是規則。通用工程紀律（read-before-write、最小 diff、先測後修、一次改一件事）由 go-spec 的 rules 與本 repo 的 hooks 機械執行，完整版在 `.claude/rules/`；這裡只寫這個 repo 特有的。

## 定位（30 秒版）

私有知識的本地閱讀與裁決介面，yomihon 與 kura 的共同繼承者。local-only、單人、永不對外。讀 `~/obsidian` 全部，寫恰好一個欄位（`status`）。yomihon（`~/go/src/github.com/koopa0/yomihon`）與 kura（`~/rust/github.com/koopa0/kura`）凍結服役至各自退役閘——它們是參考實作，不是依賴。

## 四道牆（違反＝停下來找 Koopa，不是繞過去）

1. **寫入面 = frontmatter `status` 單欄位**。轉移必須過 vault-schema.toml 的狀態機（from + owner）驗證；每次轉移一個 git commit（author 用 Koopa 本人的 git identity）。要寫任何其他欄位 → 新決定。
2. **永遠 127.0.0.1**。listener 寫死 loopback，只有 port 可設定。搜尋索引與一切派生資料不離開本機。
3. **schema 理解唯一來源 = `~/obsidian/System/schemas/vault-schema.toml`**。`internal/schema` 是唯一讀它的套件；repo 內不准出現硬編碼的 enum／狀態機第二份。
4. **渲染器永不「修好」筆記**。容錯地讀、呈現診斷（壞 YAML、斷鏈、撞名）；判官只報告，改檔的是人。

## 這個 repo 的可預測錯誤

1. **沒讀 `docs/vault-model.md` 就動 renderer／graph／search。** 你會用泛用 Obsidian 知識寫出錯的 wikilink 解析——這個 vault 的方言有 spec（kura 的 `graph.rs`：NFC、title 永不作 key、撞名不猜）。先讀那份文件。
2. **把 enum 或狀態機抄進程式碼。** 當你想寫 `if status == "ready"` 之類的清單時，答案永遠在 `internal/schema`，來源永遠是 toml（牆 3）。
3. **順手「修好」一筆壞 frontmatter。** 診斷型別是唯讀的。kurodo 報告、kura 判決、人改檔（牆 4）。
4. **從 yomihon 搬程式碼。** 正確性以 fixtures 轉移，實作全新寫（decisions D04）。可以搬的是測試斷言與截圖基準，不是 parser。
5. **重新序列化 YAML。** status 寫入是外科手術：frontmatter 區塊內恰好換一行，其餘 byte-identical。任何 yaml marshal 往返都會毀掉檔案格式。
6. **發明第二種正確。** JSONL 欄位、fingerprint（FNV-1a＋`0x1f`）、排序、exit code、掃描邊界，全以 kura 為 byte-compat 目標（`docs/spec.md` §5）。「改進」它＝破壞四條管線。
7. **加依賴或框架。** Alpine 已在 yomihon M2 被移除過一次；htmx 等真的出現 partial-update 需求；向量搜尋走 kura-field-log 三關（D05）。`docs/design.md` §2 的「不引入」清單是院子裡的法律。
8. **自己立里程碑柵欄。** 沒有 M1／M2（D15）。規格與驗收在 `docs/spec.md`，實作順序由使用中的痛決定。

## 事實

- Stack：Go 1.26 / templ / Tailwind v4（standalone CLI，無 Node）/ PostgreSQL + pgx + sqlc / goldmark
- Module：`github.com/koopa0/kurodo`；binary `kurodo`；serve 預設 `127.0.0.1:9610`
- DB 全是派生資料：可隨時 drop 重建，真相永遠是 vault 檔案 + git
- Migrations：pre-release 階段一切 schema 變更直接改 `001_initial`，不開 002+
- 生成碼（`internal/db/`、`*_templ.go`）永不手改

## Go 標準

一切 Go 慣例依 go-spec：`~/go/src/github.com/koopa0/go-spec`。重點：package-by-feature（禁 services/repository/models/handlers 等分層目錄名）、pgx + sqlc（禁 database/sql、ORM）、測試 stdlib + go-cmp（禁 testify、mock 框架）、golangci-lint v2 零容忍。

## 必讀（依序）

1. `docs/vault-model.md` — 動 renderer / graph / search 前必修
2. `docs/spec.md` — 目標、最終功能規格、驗收標準
3. `docs/design.md` — 架構、資料流、退役閘
4. `docs/decisions.md` — 決策記錄（為什麼是這樣）

## 參考實作（參考不搬碼；正確性以 fixtures 轉移，見 D04）

| 範圍 | 位置 |
|---|---|
| Obsidian 方言渲染的參考實作 | `~/go/src/github.com/koopa0/yomihon/internal/markdown/parser.go` |
| wikilink 解析語意的參考 spec | `~/rust/github.com/koopa0/kura/src/graph.rs`、`src/wikilink.rs` |
| 既有 markdown component | `~/koopa0.dev/frontend/src/app/core/services/markdown.service.ts`（untrusted-body 前提，情境不同） |
| templ UI blocks | `~/go/src/github.com/koopa0/goilerplate/blocks/`（只拿 UI 塊，不拿它的分層結構） |
| 閱讀介面樣式參照 | `~/Downloads/tailwind-plus-syntax/syntax-ts/` |

## Harness（2026-07-02 從 go-spec bootstrap 同步）

規則在 `.claude/rules/`（path-scoped）；決策樹在 `.claude/QUICKSTART.md`；hooks 已註冊於 `.claude/settings.json`（分層目錄封鎖、生成碼封鎖、自動格式化、commit message 驗證等）。驗證閘：`make verify`（fmt→vet→lint→test→build）與 `make verify-spec`（harness 自檢）。

## Available Agents

| Agent | Purpose |
|---|---|
| `comprehend` | 動工前理解 codebase |
| `planner` | 實作前設計計畫 |
| `scaffold` | 建 feature package 骨架 |
| `go-reviewer` | Go code review |
| `review-code` | 深度偏執 review |
| `db-reviewer` | SQL / schema review |
| `test-writer` | 測試生成 |
| `build-resolver` | build / lint 錯誤修復 |

## Available Skills

36 個 skills 同步自 go-spec（清單見 `.claude/skills/`）：pgx / sqlc / postgres / http-server / testing / debug / lifecycle / verify / checkpoint 等。已剔除不適用者（genkit、nats、auth、docker、otel、ristretto、api-design）。
