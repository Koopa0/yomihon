# kurodo 設計

> 定位一句話：私有知識的本地閱讀與裁決介面，yomihon 與 kura 的共同繼承者。local、單人、永不對外。
> 邊界模型：**大院子＋不可逆圍牆**——功能空間全開，牆只立在不可逆的地方（四道牆見 `CLAUDE.md`）。功能規格與驗收在 `spec.md`；本文件講架構。除了牆與退役閘，其餘一切是「提案」，建造 session 有調整權。

## 1. 系統脈絡

```
                    ┌─ 判官 ── kura（15 rules，服役至判官 parity 閘）
~/obsidian（真相）──┤
   ↑ 唯讀整座 vault  └─ 閱讀器 ─ yomihon（凍結服役至閱讀 parity 閘）
   │
 kurodo ──寫──▶ status 單欄位 ＋ git commit（人類終端）
   │            └─ CLI 工具箱：vault 側 agent 也透過 kurodo 做事（判官 parity 後）
   ├─ hermes：寫入走 worktree / QA-Gate，與 kurodo 無介面
   └─ koopa0.dev：對外執行與發布，兩邊互不侵入
```

kurodo 讀全部、寫一欄、不修檔、不越權。DB 只放派生資料（索引），真相永遠是 vault 檔案 + git。

## 2. 技術棧（每個依賴自證必要）

| 依賴 | 理由 |
|---|---|
| goldmark + frontmatter ext | yomihon 用真課文驗證過的 pipeline 基礎 |
| a-h/templ | server-rendered HTML，既有 muscle memory（yomihon、goilerplate） |
| Tailwind v4 standalone CLI | 無 Node 依賴；typography plugin 做 prose |
| pgx/v5 + sqlc + PostgreSQL | 搜尋索引與結構化過濾；pg_trgm 內建免安裝（搜尋面起用） |
| fsnotify | vault 變更即時重索引（搜尋面） |
| `os/exec` git | 審計要與人手 git 完全同語意；不引 go-git |
| vanilla JS（單檔） | 承 yomihon：原生元素優先（details/dialog），無框架 |

**不引入**：Alpine（yomihon M2 已移除的先例）、HTMX（等 partial-update 需求真的出現再說）、go-git、任何 ORM、任何 JS 框架、向量檢索（D05 三關）。

## 3. 二進位與指令面

單一 binary `kurodo`，`cmd/kurodo` 只做 wiring（go-spec 教義）。這同時是**人的介面（serve）與 agent 的介面（check/exists/coverage）**——obsidian 側的 Claude Code 是 CLI 面的直接消費者，輸出格式必須對齊 kura（JSONL / human / md）：

| 指令 | 作用（規格見 spec.md） |
|---|---|
| `kurodo serve` | 工作台本體，`127.0.0.1:9610`（く9ろ6ど10，只有 port 可設） |
| `kurodo reindex` | 全量重建派生索引（serve 啟動時亦自動） |
| `kurodo check` | kura check 的 Go 重寫，JSONL 黃金比對（yomihon SPEC §13 的 check 計畫作廢，lint 歸此） |
| `kurodo exists` / `coverage` | kura 的 agent 工具箱同步吸收；後續擴展依 vault 側真實需求進院子 |
| `kurodo export` | 吸收 yomihon 的 SSG 匯出模式 |

設定（config struct 照 go-spec config-management）：`KURODO_ROOT`（vault 路徑，預設 `~/obsidian`）、`KURODO_PORT`（預設 9610）、`KURODO_DB`（pg DSN，搜尋面起用）。**沒有 KURODO_ADDR**——loopback 寫死是牆 2 長成程式碼的樣子。

## 4. 套件佈局（提案，package-by-feature）

```
cmd/kurodo/         wiring only：config、deps、routes、graceful shutdown
internal/vault/     fs walk（路徑 NFC 化）、frontmatter 切分與容錯解析、fsnotify watch
internal/schema/    vault-schema.toml 載入（enum、status_group、lifecycle、slug pattern）——唯一讀 toml 者（牆 3）
internal/render/    Obsidian 方言 → HTML：goldmark pipeline（照 yomihon parser.go）＋ yomihon 缺的 ==highlight== 與 ![[embed]]
internal/graph/     wikilink 解析（照 kura graph.rs 語意）、link 索引、診斷
internal/search/    搜尋 feature：query.sql、store.go、handler.go
internal/status/    唯一的寫：狀態機驗證、外科手術式單行改寫、git commit（牆 1）
internal/note/      閱讀 feature：載入＋渲染＋TOC＋診斷面板、課綱樹（Maps 解析）
internal/ui/        templ：layouts / pages / blocks（yomihon 三層慣例）
internal/db/        sqlc 生成，永不手改
```

牆如何長成程式碼：`internal/status` 是唯一有寫檔與 git 能力的套件；`internal/schema` 是唯一碰 toml 的套件；`internal/render` 的診斷型別唯讀；listener 寫死 loopback。

## 5. 資料流

**讀**：啟動全量掃描 → `vault` 解析 → 派生索引入 PG → fsnotify 增量重索引。渲染 per-request（419 檔規模下毫秒級；HTML 快取等實測慢了再加——convergent）。

**寫（唯一的一條）**：正式演算法、UI、錯誤語彙、驗收全在 `spec.md` §4。骨架：讀檔取現況 → 狀態機驗證（from＋owner，actor=koopa）→ dirty 檢查 → 外科手術式單行改寫 → 原子寫回 → git commit（author＝Koopa identity，`(via kurodo)`）→ PRG redirect。

## 6. 資料庫（全派生、可隨時 drop 重建）

```sql
-- 概念草圖，實作時進 migrations/001_initial（pre-release 一律只改 001）
note      (rel_path PK, title, note_type, domain, status, slug, topics text[],
           frontmatter jsonb, fm_valid bool, fm_errors jsonb,
           content_hash, mtime, indexed_at)
link      (src_path, target_raw, resolved_path?, kind wikilink|embed|pathref, ambiguous bool)
note_text (rel_path PK, plain_text)   -- GIN (plain_text gin_trgm_ops)
```

沒有 status 歷史表：`git log` 就是歷史（vault-model §3）。

## 7. 搜尋

確定性全文（pg_trgm）＋結構化過濾；規格與驗收見 `spec.md` §3。pg_trgm 對 2 字 CJK 查詢會退化 seq scan——現有規模下毫秒級，可接受；升級 pg_bigm / PGroonga 或語意檢索一律走 kura-field-log 三關（`System/kura-field-log.md:22`）。

## 8. UI（樣式參照 Syntax template，實作全 server-rendered templ）

- **Shell**：sticky header（搜尋面板 ⌘K、theme toggle）＋左側欄＋內容欄＋右欄。參照 `syntax-ts/src/components/{Layout,Navigation,Prose}.tsx` 與 `src/styles/tailwind.css` 的 type scale——只搬視覺語言，不搬 React。視覺身分等 Koopa 帶回 Claude design 的 `@theme` tokens（需求見 `claude-design-brief.md`）後套上。
- **左側欄**：lifecycle 資料夾（vault 順序，頂層 ≤9）＋課綱樹（兩份 study-path 結構不同）＋ Reports 區（daily-briefing HTML 以 sandboxed iframe 呈現）。
- **內容欄**：typography prose；callout 兩桶配色；ruby 原樣通過；mermaid client 渲染；程式碼高亮 server-side（chroma，不引 prism/JS）。
- **右欄**：TOC（CJK-safe slug）＋ frontmatter/status 面板（全部合法轉移，ready 突出）＋診斷欄（只顯示，永不修）。
- **日文課互動**：照 yomihon 已驗證的機制原樣重現——furigana 用 `visibility` 不用 `display`（防 reflow）、TTS 的 `data-tts` 在 build/render 期剝好 `<rt>/<rp>`、slot 吃 `slots/*.yaml` sidecar、concept 抽屜用原生 `<dialog>`。

## 9. 目標與退役閘

目標（終態四點）見 `spec.md` §0。**沒有里程碑柵欄（D15）**——實作順序自由，唯一排序建議（非圍欄）：先打通「讀完→認證」那一鍵（D10，v0 出貨閘），它攻擊的是整個系統當前的真瓶頸——Koopa 的裁決摩擦。

兩個退役是**憑證據的閘，不是日期**（D11）：
- **yomihon 退役閘**＝`spec.md` §1（視覺 parity）＋ §6（五互動＋fixtures＋兩週實用）。達成前 yomihon 凍結服役（tag `v1.0.0`，只修 bug）。
- **kura 退役閘**＝`spec.md` §5（JSONL byte-compat＋snapshots＋掃描邊界＋四管線切換）。達成前 kura 一行不動。

## 10. 未排程（院子開放，牆不擋）

graph view、backlinks 面板、frontmatter query、閱讀進度、匯出 PDF、MCP server（kura README 留下的線頭）……都合法，哪個先長讓使用中的痛與 vault 側 agent 的真實需求決定。
