# yomihon v2（読本）

私有知識的本地閱讀與裁決介面。local-only、單人、永不對外。讀 `~/obsidian` 全部，寫恰好一個欄位（`status`）。本 repo 是 v2；v1（`~/go/src/github.com/koopa0/yomihon`）凍結服役至退役閘，是參考實作不是依賴。

## 四道牆（違反＝停下來找 Koopa，不是繞過去）

1. **寫入面 = frontmatter `status` 單欄位**。轉移必須過 vault-schema.toml 的狀態機（from + owner）驗證；每次轉移一個 git commit。要寫任何其他欄位 → 新決定。
2. **永遠 127.0.0.1**。listener 寫死 loopback，只有 port 可設定。搜尋索引與一切派生資料不離開本機。
3. **schema 理解唯一來源 = `~/obsidian/System/schemas/vault-schema.toml`**。`internal/schema` 是唯一讀它的套件；repo 內不准出現硬編碼的 enum／狀態機第二份。
4. **渲染器永不「修好」筆記**。容錯地讀、呈現診斷（壞 YAML、斷鏈、撞名）；判官是 kura，改檔的是人。

## 事實

- Stack：Go 1.26 / templ / Tailwind v4（standalone CLI，無 Node）/ PostgreSQL + pgx + sqlc / goldmark
- Module：`github.com/koopa0/yomihon`（與 v1 同名；folder 暫名 `yomihon-v2`，見 decisions D01）
- DB 全是派生資料：可隨時 drop 重建，真相永遠是 vault 檔案 + git
- Migrations：pre-release 階段一切 schema 變更直接改 `001_initial`，不開 002+
- 生成碼（`internal/db/`、`*_templ.go`）永不手改

## Go 標準

一切 Go 慣例依 go-spec：`~/go/src/github.com/koopa0/go-spec`（完整規則在其 `AGENTS.md` 與 `.claude/rules/`）。重點：package-by-feature（禁 services/repository/models/handlers 等分層目錄名）、pgx + sqlc（禁 database/sql、ORM）、測試 stdlib + go-cmp（禁 testify、mock 框架）、golangci-lint v2 零容忍（本 repo 的 `.golangci.yml` 即從 go-spec 同步）。

## 必讀（依序）

1. `docs/vault-model.md` — 動 renderer / graph / search 前必修
2. `docs/design.md` — 架構、資料流、里程碑與退役閘
3. `docs/decisions.md` — 決策記錄（為什麼是這樣）

## 參考實作（參考不搬碼；正確性以 fixtures 轉移，見 decisions D04）

| 範圍 | 位置 |
|---|---|
| Obsidian 方言渲染的參考實作 | `~/go/src/github.com/koopa0/yomihon/internal/markdown/parser.go`（v1） |
| wikilink 解析語意的參考 spec | `~/rust/github.com/koopa0/kura/src/graph.rs`、`src/wikilink.rs` |
| 既有 markdown component | `~/koopa0.dev/frontend/src/app/core/services/markdown.service.ts`（注意：它有 untrusted-body 前提，v2 是 trusted corpus，情境不同） |
| templ UI blocks | `~/go/src/github.com/koopa0/goilerplate/blocks/`（只拿 UI 塊；它的 boilerplate 分層結構與 go-spec 教義相反，不拿） |
| 閱讀介面樣式參照 | `~/Downloads/tailwind-plus-syntax/syntax-ts/` |
