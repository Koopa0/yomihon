# kurodo 功能規格（最終形）

> **沒有里程碑柵欄**（decisions D15）：這份文件定義「做完長什麼樣」，不定義「先做哪個」。每個功能面各有規格與驗收標準，實作順序由使用中的痛決定。唯一排序建議（非圍欄）：先打通「讀完→認證」那一鍵（D10）。
> 四道牆（`CLAUDE.md`）凌駕本文件一切內容。狀態：**已認證（2026-07-02，含審查修訂：§0.1 依賴邊界、§4 fail-closed 與審計邊界、D16=(a)）**。

## 0. 目標

讓 Koopa 在一個地方讀完整座 vault，並在讀完的地方完成裁決；同時接下 kura 的判官與 agent 工具箱職責，成為 vault 生態的人類終端＋CLI 介面。

**終態的樣子（系統成功的定義）**：
1. Koopa 每天在 kurodo 裡讀書——yomihon 憑退役閘退役。
2. draft 佇列在讀完當下被 flip——裁決摩擦消失。
3. kura 的四條管線跑在 `kurodo check` 上——kura 憑退役閘退役。
4. vault 側 agent 把 kurodo 當工具箱（`check` / `exists` / `coverage` 及後續擴展）。

### 0.1 依賴邊界（全域不變量）

- **檔案是真相，DB 只加速。閱讀面不得依賴 DB**：PostgreSQL 不在場 → 讀照常，只有 ⌘K 明示降級（§3）。「每天讀書」這個習慣不得繼承任何 daemon 依賴——否則工具吸收法則會殺掉它。
- **`check` / `exists` / `coverage` 是無狀態檔案掃描，不碰 DB**：四條管線與 vault 側 agent 消費的必須是零依賴 binary（kura 的部署形狀）。判官面若要求 PG 在場＝部署性倒退＝退役閘變相加嚴，禁止。
- **schema 唯一來源＝`~/obsidian/System/schemas/vault-schema.toml`（牆 3），且容錯方向不對稱**：讀面對 schema 問題 fail-open（照渲染＋診斷——lint 少報無害）；**寫入面 fail-closed**（schema 讀不到或壞掉 → 不顯示任何轉移鍵、POST 一律拒絕——讀壞了少報，寫壞了毀檔）。

---

## 1. 閱讀面

**規格**：
- 全 vault 渲染：方言完整表＝`vault-model.md` 第一層（wikilink 四態、embed、callout 兩桶＋中文標題、`==highlight==`、tasks、raw HTML ruby 原樣通過、mermaid；含 yomihon 缺的 highlight 與 embed）。
- 容錯鐵則：壞 YAML／未知 callout／斷鏈 → 照渲染＋診斷，永不 crash、永不修（牆 4）。
- TOC：CJK-safe slug（照 yomihon `Slug()` 語意：`\p{L}\p{N}` 保留、撞名 `-2` 後綴、fallback `section`）。
- 診斷欄：壞 YAML、unresolved／ambiguous link、corrections ledger——只顯示。
- Reports：`System/reports/` 含 daily-briefing HTML 以 sandboxed iframe 原樣呈現。sandbox 政策：**不帶 `allow-same-origin`**——即使帶 scripts 也落在 opaque origin，與同源防護形成雙層防禦。
- `.base`／`.canvas` 連回 Obsidian 開啟；D2 不渲染（koopa0.dev 已決策）。
- 程式碼高亮 server-side（chroma）。JS 原則：**零框架、零外部 JS 依賴；手寫 vanilla 允許**（yomihon 的五互動即 ~207 行手寫 JS 的先例）。

**Body 首 H1 政策**：頁標題來自 frontmatter `title`；body 開頭的 H1 移除不重複顯示（照 yomihon 已驗證行為）。

**驗收**：
- dialect conformance 測試全過（結構斷言模式承 yomihon `testdata/lesson.md`）。fixture 至少覆蓋：ruby/`<br>` 原樣通過；callout 全型別＋`[!x]-`/`[!x]+` 摺疊＋未知型別降級 blockquote；wikilink 四態＋alias 顯示＋ambiguous 標記；`![[embed]]`；`==highlight==`；task list；GFM 表格（含跳脫 `\|`）；fence 內方言不處理＋一次性警告；壞 YAML 單診斷不 cascade；body 首 H1 移除；CJK slug 與 TOC anchor 對齊（含撞名 `-2` 後綴）。
- 真實 vault 全部 `.md` 可開：0 個 500、0 個空白頁（fault-tolerance 的機械定義）。
- 日文課文（L01–L20＋P 系列）與 yomihon 視覺 parity：`m1-review/` 14 張截圖為基準。

## 2. 導航面

**規格**：側欄＝lifecycle 資料夾（vault 順序，頂層 ≤9）＋課綱樹＋ Reports 區。首頁沿用 `Vault-Index.md` 的入口分區（細節等 obsidian CC 回覆，見 `obsidian-cc-questions.md` §2）。

**課綱樹解析規格**（機械可判）：
- 課綱的判定靠 frontmatter `type: study-path`（toml enum），**不硬編碼檔名**。
- Go 課綱形態：H2＝部、H3＝模組，皆為 pipe 格式 `slug | English | 中文`（切三段、trim）；清單項＝課。
- 大家的日本語形態：只有「課程序列」該個 H2 之下是導航樹（H3＝學習階段）；其餘 H2（每日 loop、學習階段、缺口）不是導航，不解析。
- 課的連結＝清單項中**第一個 wikilink**，以 graph 語意解析（vault-model 第一層）；unresolved／ambiguous 的課**照列＋診斷樣式**，不掉項（讀面 fail-open）。
- 課節點帶 status badge。

**驗收**：任一 vault 檔案 ≤3 次點擊可達；課綱頁順序＝檔內列序；兩份真實課綱各自渲染成樹且課數與檔內清單項數一致；故意寫一個斷鏈課 → 照列＋標記。

## 3. 搜尋面

**規格**：
- 確定性全文：pg_trgm substring/phrase ＋ 結構化過濾；⌘K 面板。
- **Query 語意**（確定性、可重現）：空白分詞；裸詞＝case-insensitive、NFC 正規化後的 substring 匹配（與 wikilink 同一套 normalize）；多詞＝AND。過濾詞六個固定 key：`type:` `status:` `domain:` `topic:` `folder:`（rel_path 前綴）`slug:`，值為字面等值不驗 enum。無魔法語法，就這些。
- **結果與排序**（確定性）：title 命中排 body 命中之前，同組內 rel_path 字典序；每筆＝路徑＋title＋status badge＋首個命中的上下文 snippet。
- 索引全派生（`note` / `link` / `note_text`，見 `design.md` §6）：drop 後 `kurodo reindex` 完整重建；fsnotify 增量更新。darwin/kqueue 會漏事件——加週期性 mtime 對帳（分鐘級全量掃描）自癒。
- PG 不可用：搜尋明示「索引離線」，其餘功能照常（§0.1 不變量）。
- 語意／向量：未排程，升級走三關（D05）。

**驗收**：2 字 CJK 查詢正確返回（允許 seq scan）；抽查與 `rg` 結果一致；drop DB → reindex → 結果不變；vault 檔案變更後 ≤2s 反映（fsnotify 路徑），漏事件由對帳掃描保證最終一致；停掉 PG → 讀照常、搜尋明示降級。

## 4. 裁決面（唯一的寫）

**前提（牆 3）**：狀態機唯一來源＝`internal/schema` 於 runtime 從 vault-schema.toml 載入，repo 內零硬編碼。schema 載入失敗 → 寫入面 **fail-closed**：不顯示任何轉移鍵、`POST /status` 一律拒絕；閱讀面照常（§0.1 的不對稱容錯）。

**審計邊界（明文）**：`author=Koopa` 的審計聲明以**本機信任邊界**為限——同帳號下的本機程序（瀏覽器、curl、agent）在密碼學上不可區分，kurodo 不以 token 過度工程。CrossOriginProtection 擋的是瀏覽器跨站 form POST，不是本機程序。治理補強：vault 側 agent-guides 立硬規則「agent 永不呼叫 kurodo 寫入端點」（已列入 `obsidian-cc-questions.md` §5）。

**已裁決**：flip **不動 `updated` 欄位**（D16=(a)）——`updated` 的語意是內容鮮度，認證不修訂理解。

**規格——寫入路徑正式演算法**：

```
POST /status (path, from, to)
 1. 同源檢查（Go 1.26 http.CrossOriginProtection——任意網站可對 127.0.0.1 發
    form POST 且不觸發 CORS preflight，此防護是牆 2 的必要深化）
 2. 讀檔 → 切 frontmatter → 現況 status = from_actual
    · from_actual ≠ form.from → 409「頁面已過期」，不寫
 3. schema.Transition(type, from_actual, to, "koopa") → 失敗 422 附原因，不寫
 4. pre-flight：vault 是 git repo 且該檔 clean
    · `git status --porcelain -- <file>` 非空 → 中止「此檔有未 commit 變更，
      flip 會污染審計」（寧可多按一次，不污染 git 歷史）
 5. 外科手術式改寫：只在 frontmatter 區塊內、行首匹配 `status:` 行
    · 恰好一行 → 整行替換為 `status: <to>`；零或多行 → 中止（schema 違規
      交給 kura／人——kurodo 不修檔）
    · 該行以外 byte-identical；絕不重新序列化 YAML
 6. 原子寫回（temp + rename，保留權限）；寫前 stat 比對步驟 2 的 mtime，
    變動即中止
 7. git add <file> && git commit -m "status(<relpath>): <from> → <to> (via kurodo)"
    · vault root 執行；不設 author ＝ vault git config（＝Koopa identity，D07）
    · commit 失敗 → 明確顯示「檔案已改、commit 失敗」＋原文；不自動回滾
 8. 302 → 閱讀頁（PRG）
```

**規格——UI**：status 面板列出**當前合法**轉移（`schema` 以 actor=`koopa` 計算；只顯示合法鍵，不顯示 disabled）；全部合法轉移開放、`ready` 唯一 primary 樣式（D13）；每顆鍵一個 form，無 JS；無 frontmatter（drills）→「無 frontmatter（合法）」無鍵；壞 YAML → 只顯診斷無鍵（讀不可靠就不寫）；schema 載入失敗 → 顯示「契約不可用」診斷、無鍵（fail-closed）。

**已知取捨**：dirty-file 中止會擋住「讀到一半在 Obsidian 修個 typo → flip」的真實流程。v0 先出貨，「先 commit 手改再 flip」的兩段式按 D15 由痛決定。

**規格——錯誤語彙**：

| 情境 | HTTP | 呈現 |
|---|---|---|
| form.from ≠ 現況 | 409 | 頁面已過期，重新載入再按 |
| 非法轉移／owner 禁止 | 422 | schema 拒絕原因原文 |
| 檔案 dirty | 409 | 有未 commit 變更，flip 會污染審計 |
| status 行零或多行 | 422 | schema 違規，交給 kura／人工 |
| mtime 變動 | 409 | 檔案在讀寫之間被修改 |
| git commit 失敗 | 500 | 檔案已改＋git 原文＋手動補救指令 |

**驗收（自動化）**：
1. 手術精準性：含引號值、註解、尾隨空白、`based_on: "[[...]]"` 的 frontmatter，改寫後除 status 行外 **byte-identical**（golden 比對）。
2. 狀態機表驅動：合法／非法 from→to／owner 全覆蓋。
3. dirty file → 中止且未寫。
4. 過期 form → 409 且未寫。
5. 壞 YAML → 無鍵；直接 POST 被拒。
6. 真 git 驗證（temp git repo）：commit 存在、message 格式正確、author 取自 repo git config、diff 恰好一行。
7. 跨源 POST（`Sec-Fetch-Site: cross-site`）被拒。

**驗收（人工，＝v0 出貨閘 D10）**：
8. Koopa 讀完 `Writing/` 一篇真實長文按下合法轉移；`git -C ~/obsidian log -1 --stat` 顯示該 commit（author＝Koopa、一檔一行）；Obsidian 確認只有 status 變了。
9. `ready` 檔案面板無鍵（無合法轉移呈現正確）；drills 顯示「無 frontmatter（合法）」。

## 5. 判官與 agent 工具箱（kura 繼承面）

**規格**：`kurodo check`（15 規則）／`exists`（dedup oracle）／`coverage`（MOC 覆蓋）。**部署形狀＝kura：單一 binary、無狀態檔案掃描、不碰 DB、無 daemon 依賴（§0.1）**。

**指令語意照 kura，不重新發明**：`check [PATHS...]` 的位置參數只過濾輸出、graph 永遠整樹建；`--root` 預設 cwd；`exists` 的匹配面刻意比 resolver 寬（resolver keys ＋ `title`／`title_en`），exit 0/1；`coverage` 輸出是單一 pretty JSON object（非 JSONL），shape 以 `conformance__coverage_report.snap` 為準。對外介面 byte-compatible：JSONL 欄位形狀、排序 `path→line→rule_id`、fingerprint（FNV-1a，`0x1f` 分隔，16 位小寫 hex）、exit code 0/1/2、`--deny <severity|rule>`、`--format json|human|md`、掃描邊界（System/Diagrams/Views 預設排除、`--all`）。後續擴展（backlinks、frontmatter query、MCP server）依 vault 側真實需求進院子（D14；需求清單見 `obsidian-cc-questions.md` §1）。

**驗收（＝kura 退役閘）**：
1. kura conformance snapshots byte-exact（`conformance__jsonl_output.snap`、`conformance__coverage_report.snap`）。
2. 對真實 vault：`kurodo check` 與 `kura check` JSONL 逐位元組一致。
3. schema.* 類依 vault-guard-spec §8 粒度：(path, rule-class, field/value) 集合等價。
4. 四條管線切換（CI pre-merge、hermes cron ×2、health-check）＋ obsidian CC 用法切換。達成前 kura 一行不動。
5. 判官三指令在無 PG 的環境可跑（CI 環境即為證）。

## 6. 匯出面（yomihon 繼承面）

**規格**：`kurodo export` ＝ SSG 靜態輸出（`dist/`），涵蓋日文課文＋課綱 index＋五互動（furigana visibility 切換、原生 details 摺疊、TTS `data-tts` build 期剝 `<rt>/<rp>`、slot sidecar、concept `<dialog>`）。PWA／Service Worker：**裁掉，不繼承**——yomihon 的 SW 因 HTTP-only 從未真正註冊，是已驗證的死重。export 輸出＝純靜態檔案。

**驗收（＝yomihon 退役閘）**：
1. 五種互動獨立重現，fixtures 全過（yomihon testdata 斷言模式＋`slots/L01–L20.yaml` 直接消費）。
2. `m1-review/` 截圖視覺 parity。
3. Koopa 實際用 kurodo 學習兩週。達成前 yomihon 凍結服役（tag `v1.0.0`）。

## 7. 全域品質閘

- `make verify`（fmt→vet→lint→test→build）全過；lint 0 issues；`go test -race -shuffle` 全綠；`make verify-spec` 全綠。
- 四道牆有測試鎖：loopback-only、path escape 拒絕、寫入面僅 status 行、渲染永不修檔（診斷型別唯讀）。

## 8. 已裁決

**D16＝(a)：flip 不動 `updated`**（Koopa 裁定，2026-07-02，否決了原推薦的 (b)）。理由：在 provenance 紀律的 vault 裡，`updated` 意謂「這篇筆記的理解上次被修訂於何時」——認證恰恰沒有修訂理解；(b) 會把「內容鮮度」這個真訊號污染成「任何觸碰」，而 stale/superseded 類視圖靠的正是鮮度。flip 的可見性本來就有家：git log，以及按 status 分組的 pipeline.base。牆 1 維持零註解。
