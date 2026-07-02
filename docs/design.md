# kurodo 設計

> 定位一句話：私有知識的本地閱讀與裁決介面，yomihon 與 kura 的共同繼承者。local、單人、永不對外。
> 邊界模型：**大院子＋不可逆圍牆**——功能空間全開，牆只立在不可逆的地方（四道牆見 `CLAUDE.md`）。本文件其餘一切是「提案」，建造 session 有調整權；牆不是。

## 1. 系統脈絡

```
                    ┌─ 判官 ── kura（15 rules，服役至 M4 parity）
~/obsidian（真相）──┤
   ↑ 唯讀整座 vault  └─ 閱讀器 ─ yomihon（凍結服役至 M3 parity）
   │
 kurodo ──寫──▶ status 單欄位 ＋ git commit（人類終端）
   │            └─ CLI 工具箱：vault 側 agent 也透過 kurodo 做事（M4 起）
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
| pgx/v5 + sqlc + PostgreSQL | 搜尋索引與結構化過濾；pg_trgm 內建免安裝 |
| fsnotify（M2） | vault 變更即時重索引 |
| `os/exec` git | 審計要與人手 git 完全同語意；不引 go-git |
| vanilla JS（單檔） | 承 yomihon：原生元素優先（details/dialog），無框架 |

**不引入**：Alpine（yomihon M2 已移除的先例）、HTMX（等 partial-update 需求真的出現再說）、go-git、任何 ORM、任何 JS 框架、向量檢索（見 §7）。

## 3. 二進位與指令面

單一 binary `kurodo`，`cmd/kurodo` 只做 wiring（go-spec 教義）。這同時是**人的介面（serve）與 agent 的介面（check/exists/coverage）**——obsidian 側的 Claude Code 是 CLI 面的直接消費者，輸出格式必須對齊 kura（JSONL / human / md）：

| 指令 | 里程碑 | 作用 |
|---|---|---|
| `kurodo serve` | M1 | 工作台本體，`127.0.0.1:9610`（く9ろ6ど10，只有 port 可設） |
| `kurodo reindex` | M2 | 全量重建派生索引（serve 啟動時亦自動） |
| `kurodo check` | M4 | kura check 的 Go 重寫，JSONL 黃金比對（yomihon SPEC §13 的 check 計畫作廢，lint 歸此） |
| `kurodo exists` / `coverage` | M4 | kura 的 agent 工具箱同步吸收（dedup oracle、MOC 覆蓋）；後續擴展（backlinks、frontmatter query…）依 vault 側真實需求進院子 |
| `kurodo export` | M5 | 吸收 yomihon 的 SSG 匯出模式 |

設定（config struct 照 go-spec config-management）：`KURODO_ROOT`（vault 路徑，預設 `~/obsidian`）、`KURODO_PORT`（預設 9610）、`KURODO_DB`（pg DSN，M2 起）。**沒有 KURODO_ADDR**——loopback 寫死是牆 2 長成程式碼的樣子。

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

**讀**：啟動全量掃描 → `vault` 解析 → 派生索引入 PG →（M2）fsnotify 增量重索引。渲染 per-request（419 檔規模下毫秒級；HTML 快取等實測慢了再加——convergent）。

**寫（唯一的一條）**：
```
UI 轉移鍵 → 讀檔取現況 status
  → schema.lifecycle 驗證 from→to ＋ owner（kura 做不到的 from→to 驗證，kurodo 天然做得到）
  → 外科手術式改寫：只改 frontmatter 區塊內 `status:` 那一行，其餘位元組不動
    （絕不重新序列化整份 YAML——那會毀掉格式與註解）
  → content_hash 比對防競態（讀寫之間檔案變了 → 中止並回報）
  → git add <file> && git commit -m "status(<path>): draft → ready (via kurodo)"
    （author＝Koopa 本人的 git identity——審計記「是 Koopa 按的」，2026-07-02 裁定）
  → 該檔增量重索引
```

UI 開放**全部合法轉移**（凡 lifecycle 允許的 from→to 都能按，同一條路徑同樣驗證），`ready` 鍵視覺上突出（2026-07-02 裁定）。

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

- **v0 範圍＝確定性全文**：pg_trgm substring/phrase ＋ 結構化過濾（type / domain / status / topics / 資料夾）。
- 已知限制：pg_trgm 對 2 字 CJK 查詢會退化 seq scan——419 檔規模下毫秒級，可接受。升級 pg_bigm / PGroonga 的條件：真的痛了，且過 kura-field-log 三關。
- **語意／向量搜尋：未排程**。升級規則同一條（`System/kura-field-log.md:22`）：真實案例＋反覆＋確定性可判，三關過了才提案。

## 8. UI（樣式參照 Syntax template，實作全 server-rendered templ）

- **Shell**：sticky header（搜尋面板 ⌘K、theme toggle）＋左側欄＋內容欄＋右欄。參照 `syntax-ts/src/components/{Layout,Navigation,Prose}.tsx` 與 `src/styles/tailwind.css` 的 type scale——只搬視覺語言，不搬 React。視覺身分等 Koopa 帶回 Claude design 的 `@theme` tokens 後套上。
- **左側欄**：lifecycle 資料夾（vault 順序，頂層 ≤9）＋課綱樹（解析 Maps 兩份 study-path，注意兩者結構不同）＋ Reports 區（`System/reports/`，daily-briefing HTML 以 sandboxed iframe 呈現）。
- **內容欄**：typography prose；callout 兩桶配色；ruby 原樣通過；mermaid client 渲染；程式碼高亮 server-side（goldmark-highlighting/chroma，M2 候選——不引 prism/JS）。
- **右欄**：TOC（CJK-safe slug，照 yomihon `Slug()`）＋ frontmatter/status 面板（全部合法轉移，ready 突出）＋診斷欄（壞 YAML、unresolved/ambiguous links、corrections ledger——**只顯示，永不修**）。
- **日文課互動（M3）**：照 yomihon 已驗證的機制原樣重現——furigana 用 `visibility` 不用 `display`（防 reflow）、TTS 的 `data-tts` 在 build/render 期剝好 `<rt>/<rp>`、slot 吃 `slots/*.yaml` sidecar、concept 抽屜用原生 `<dialog>`。

## 9. 里程碑與退役閘

**M0 骨架**：repo → walking skeleton：serve 渲染一篇真實筆記 e2e、schema loader 讀 toml。

**M1 縱切＝v0 出貨閘**：在 kurodo 裡讀完 `Writing/` 一篇長文，就地按 ready——狀態機驗證＋單行改寫＋git commit 全通。出貨標準是這一個動作，不是功能清單。

**M2 廣度**：全 vault 導航＋搜尋＋報告＋課綱樹＋診斷欄＋fsnotify。

**M3 日文 parity → yomihon 退役閘**（達成前 yomihon 凍結服役）：
1. 五種互動獨立重現（機制照 §8）；
2. fixtures 轉移：yomihon `internal/markdown/testdata/lesson.md` 的結構斷言模式＋真檔 L01 測試＋`m1-review/` 14 張截圖作視覺驗收基準；
3. yomihon `slots/L01–L20.yaml` 由 kurodo 直接消費；
4. Koopa 實際用 kurodo 學習兩週。
達成後 yomihon 退役，SSG 模式併入 `kurodo export`（M5）。

**M4 判官 parity → kura 退役閘**（達成前 kura 一行不動）：
1. `kurodo check` 對真實 vault 的 JSONL 與 `kura check` 逐位元組一致（排序 path→line→rule_id；FNV-1a fingerprint 含 `0x1f` 分隔必須重現）；
2. kura conformance snapshots byte-exact（`conformance__jsonl_output.snap`、`conformance__coverage_report.snap`）；schema.* 類依 spec §8 粒度走 (path, rule-class, field/value) 集合等價；
3. 掃描邊界複製 kura（System/Diagrams/Views 預設排除、`--all` 語意、exit code 0/1/2、`--deny` 語意、`--format json|human|md`）；
4. 四條管線切換：CI pre-merge、hermes cron ×2、health-check；obsidian 側 agent 的 CLI 用法同步切換。

## 10. 未排程（院子開放，牆不擋）

graph view、backlinks 面板、frontmatter query、閱讀進度、匯出 PDF、MCP server（kura README 留下的線頭）……都合法，哪個先長讓使用中的痛與 vault 側 agent 的真實需求決定。唯一的排序建議（非圍欄）：先打通 M1 那一鍵，它攻擊的是整個系統當前的真瓶頸——Koopa 的裁決摩擦。
