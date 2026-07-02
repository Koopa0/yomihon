# Vault Model — 建造者的 Obsidian 理解指南

> 動 renderer / graph / search 前的必修課。這份文件描述的是 `~/obsidian` 這個**特定 vault** 的方言、語意與權威結構——不是泛用 Obsidian 教學。
> 文中「v1」＝舊 yomihon（`~/go/src/github.com/koopa0/yomihon`，參考實作）；「v2」＝本專案。
>
> 事實錨定日期：2026-07-02，全數經實檔查證。vault 是活的：規模數字會變，但契約層（第二、三層）只會透過 vault-schema.toml 演化。

---

## 第一層：Obsidian 方言（任何 md parser 都不會自動給你的東西）

### Wikilink（最重要的一節）

語法四態：`[[名稱]]`、`[[名稱|顯示文字]]`、`[[名稱#標題]]`、`[[名稱^區塊]]`。

解析語意的參考 spec 是 kura 的 `src/graph.rs` + `src/wikilink.rs`，v2 必須逐條複製：

- **Normalize**：`trim → Unicode NFC → lowercase`。NFC 是 CJK 必須（macOS 檔名走 NFD，kura 在 walk 時也把路徑 NFC 化，見 `vault.rs`）。
- **Key 集合**：每則筆記以四種形式為 key——檔名 stem、含 `.md` 全名、vault 相對路徑 stem、完整相對路徑——**加上 frontmatter `aliases`**。
- **`title` 永不作 key**。這是 kura killer rule `link.title_not_alias` 的存在理由：用 title 寫的連結 Obsidian 會靜默斷掉。
- **剝離順序**：先 `|`（顯示文字）、再 `#`（標題）、再 `^`（區塊）。表格內的跳脫 `\|` 要處理（pre-pipe 段尾的 `\` 剝掉）。`[[#標題]]` 剝完為空 → 同檔跳轉，直接略過。
- **Anchor 永不驗證**：`[[X#h]]` 只要 X 存在就算 resolved，不查 heading／block 是否存在。
- **撞名 → Ambiguous，不猜**：同一 normalized 名稱對到多個路徑時全列出（排序後），永不選一個。不同資料夾的同名檔即撞名。
- **非 md 資源**（canvas / pdf / 圖片）以「含副檔名」的檔名與路徑為 key——Obsidian 連結非筆記檔需要副檔名。
- **掃描邊界**：code span / fenced block 與 `%%…%%` Obsidian 註解內的 `[[...]]` 不算連結；wikilink 不跨行。

### Embed：`![[...]]`

轉錄嵌入，圖片／筆記／PDF 都可能出現。注意：v1 的 parser **沒有處理** embed（前導 `!` 會殘留為字面文字）——v2 要補上，這是參考實作已知的空隙之一。

### Callout：`> [!type]`

v1 有一套用真課文磨出來的完整轉換，照抄其語意：

- 摺疊記號：`[!type]-` → 預設**收合**的 `<details>`；`[!type]+` → 展開的 `<details>`；無後綴 → 靜態底色 alert。
- 型別對照（兩桶配色：`note`＝sky、`warning`＝amber）＋預設中文標題：

| callout 型別 | 桶 | 預設標題 |
|---|---|---|
| info / note / tip / hint / abstract / summary / todo | note | 提示 |
| question / help / faq | note | 問題 |
| example / quote / cite | note | 範例 |
| warning / caution / attention | warning | 注意 |
| danger / error / bug / fail / failure / missing | warning | 警告 |

- **未知型別 → 降級為普通 `<blockquote>` ＋ log warning，永不 crash**（容錯鐵則）。
- 中譯／解答摺疊用原生 `<details>`：零 JS、離線安全、可及性好。

### Highlight：`==文字==`

v1 **沒有處理**——v2 要處理（渲染整座 vault 時會遇到）。

### Tasks：`- [ ]` / `- [x]`

GFM task list，goldmark `extension.GFM` 涵蓋。

### YAML frontmatter

結構層而非裝飾（見第二層）。容錯要求：壞 YAML 只發**一則**診斷（對齊 kura 的 `schema.frontmatter`，不 cascade），不能讓一篇筆記毀掉整次渲染。先切出 frontmatter 再跑任何 body 前處理（v1 的 `splitFrontmatter` 教訓：否則 `based_on: [[...]]` 這種值會被 wikilink pass 弄壞）。

### Body 內 raw HTML

日文課文整段手寫 `<ruby>…<rt>…</rt></ruby>` 與 `<br>`，**必須原樣通過、不消毒**（goldmark `WithUnsafe`）。這成立的前提是 trusted corpus ＋ local-only——也就是牆 2 存在的理由之一。不設自動硬換行，讓顯式 `<br>` 生效。

實例：`Writing/lessons/japanese/L20 普通形と常体.md:141`（片假名也上 ruby）；助詞標實際讀音——`<ruby>は<rt>わ</rt></ruby>`、`<ruby>を<rt>お</rt></ruby>`。

### Code fence 安全

行式前處理（callout / wikilink / 表格類 pass）不懂 fence。政策承 v1：偵測 fence 內出現 `[[...]]`、`> [!…]`、pipe-table 時發一次 build warning，而不是靜默弄壞輸出。

### 非 markdown 檔

| 種類 | 實況 | 處理 |
|---|---|---|
| `.base`（Obsidian Bases） | `Views/` 有 5 個（knowledge-overview、needs-attention、pipeline、share-rewrite、日本語課程） | v0 連回 Obsidian 開啟，**不要實作查詢引擎** |
| `.canvas` | 恰好 1 個：`Diagrams/canvas/DDIA-Ch1-Overview.canvas`（JSON） | 同上，連回或縮圖 |
| `.d2` | `Diagrams/d2/` 2 個 | **不渲染**——koopa0.dev 已決策 7.8MB WASM 不值得，別推翻 |
| mermaid code fence | Writing/ 有使用 | 渲染（client-side，koopa0.dev 已對齊過） |
| `System/reports/daily-briefing/*.html` | hermes 每日簡報 | 原樣 serve（trusted corpus） |

---

## 第二層：這個 vault 的語意模型（比方言更重要）

### 規模（2026-07-02 快照）

全 vault 419 個 `.md`：Writing 163、Concepts 120、Sources 88、System 36、Maps 11、Synthesis 1、Inbox 0。搜尋與索引的設計以「低千位、持續被 agent 灌大」為前提。

### 資料夾＝生命週期，不是分類

主流：`Inbox → Sources → Concepts → Maps / Synthesis → Writing`；`System/`＝治理層、`Views/`＝儀表板、`Diagrams/`＝圖。硬規則（Vault-Architecture.md）：**頂層 ≤ 9 個資料夾、domain 只一層深、無子資料夾**——導航靠 MOC 與 `topics`，不靠樹。v2 的側欄應該反映這個模型，而不是發明自己的樹。

### 四條鐵則（Note-Schema.md）

1. Properties 表達結構化資料（type / status / domain / topics / provenance）。
2. Tags 只做跨域檢索，永不編碼 type / status / domain。
3. 資料夾表達 artifact lifecycle，不表達完整主題分類。
4. `type`＝artifact 種類、`status`＝生命週期階段——同一事實永不雙重編碼。

### Frontmatter 資料模型

- 必填（知識筆記）：`title, aliases, type, domain, topics, status, created, updated`；Inbox 免 `domain`；`system / guide / template / research-brief` 四型免 domain。
- `type` 共 20 值、`domain` 共 13 值（golang、rust、japanese、meta …）——**enum 一律以 toml 為準，不要抄進程式碼**（牆 3）。
- Source 類另有 provenance 五欄（`source_kind / source_provider / source_work / source_locator` ＋ `based_on`）；concept 必須有 `based_on` 或 `source_locator` 其一（kura `schema.provenance`）。
- Lesson 專屬欄位：`slug, title_en, version_sensitive, objectives_count, assessment_item_count, corrections, evolution_predecessor, evolution_successors`。
- `domain` 值必須等於所在資料夾名（kura `schema.domain_folder`）。
- schema 是**封閉的**：`fields.known` 之外的 key 即錯誤。

### status 是分組狀態機（不是一條扁平 enum）

toml 的 `[fields.status_group]` 把 type 映到三組：

- **note 組**：captured → cleaned（sources）；seedling → growing → evergreen（concepts）；draft → ready（writing）
- **lesson 組**：imported → draft → ready
- **system 組**：active / archived
- `archived` 可從任意狀態進入（`from = ["*"]`）。

toml 的 `[[lifecycle]]` 表對每個 status 宣告 `from`（合法前態）與 `owner`——**`ready` 的 owner 是 `koopa`，任何 agent 寫 ready 即違規**。v2 是單人 local app、操作者即 Koopa，所以 UI 上的 ready 鍵合法；但非法 from→to 轉移要擋。

關鍵事實：toml 註解自承 file-scan 工具（kura）看不到「前一個狀態」，所以只驗 enum + owner，from→to 執行被 deferred。**v2 是互動式寫入者，寫入前讀得到現況，天然能執行完整 from→to + owner 驗證**——這是 v2 對契約的第一個實質貢獻，不是重複 kura 的工作。

### slug

只有 lesson 需要。pattern `^[a-z0-9]+(-[a-z0-9]+)*$`（toml 內建）；namespace 前綴：日文 `jp-minna-lNN` / `jp-kana-pNN`，Go 課用 plain slug。**slug 一經定稿不改；檔名可變**。

### 課綱（study-path）是機器可解析的

- `Maps/Go 課綱.md`：H2＝部、H3＝模組，皆為 pipe 格式 `slug | English | 中文`；清單項＝課（wikilink）；列序＝順序。
- `Maps/大家的日本語 初級I 學習路徑.md`：**結構略不同**——「課程序列」H2 之下 H3＝學習階段（解碼期、動詞入門…），清單項＝課。「清單項＝課、列序＝順序」兩者一致，但別假設兩份課綱同構。
- 渲染課綱頁時，這個結構就是導航樹。

### 日文教材的特殊事實

- 兩個系列：`L01`–`L20`（文法主課）＋ `P01`–`P07`（假名前置課），皆 `type: lesson, domain: japanese`。
- **drills（`Writing/lessons/japanese/drills/`，8 檔）無 frontmatter 是故意且合法的**（toml `no_frontmatter_is_legal`；kura 與 Bases 都排除它們）——當附件級內容處理，別要求 schema。
- 分工線：vault 管**理解**（P 系列、文法課），Kotonoha app 管**反射**（假名/漢字 drill）——drill 型互動永不長進 v2。
- 正字法規則（Japanese-Companion-Guide.md）：furigana 可 fade、**片假名永遠上 ruby 且永不 fade**、助詞標實際讀音、嚴格 level-gating。

### golang 課文的特殊事實

改寫過的課帶 **corrections ledger**：frontmatter `corrections:` 列表，每項 `{claim / fix / source}`（例：`Writing/lessons/golang/Garbage Collection Guide.md`）。渲染時值得呈現——它是「這份教材修過什麼」的審計面。

---

## 第三層：權威與治理（yomihon v2 在生態裡的位置）

### vault-schema.toml 是機器真相源

`System/schemas/vault-schema.toml`（schema_version 1）自我宣告為 SoT，消費者：kura（schema.* 規則）、`gen_fileclasses.py`、Note-Schema.md（人類教義，「改 enum 先改 toml」）、以及 yomihon v2。v2 只讀它，永不硬編碼（牆 3）。

### kura 是 corpus 判官（15 條規則）

7 條 link/graph（`link.title_not_alias`、`link.broken`、`link.broken.path`、`collision.alias`、`provenance.unresolved`、`map.disk_mismatch`、`map.disk_unlisted`）＋ 8 條 schema.*（enum / required / unknown_key / slug / domain_folder / legacy_tag / provenance / frontmatter，全為 error 級）。gate 語意：`--deny error`；info 永不 gate。

v2 看到壞東西：**照渲染＋標記診斷，不修、不擋、不越權**（牆 4）。

### 管線（v2 的 `check` 未來要接的四條）

- pre-merge / CI：`kura check --deny error`
- hermes cron ×2：`cron-vault-wrapper.sh`、`cron-translator-wrapper.sh`（絕對路徑 `~/.cargo/bin/kura`）
- `/health-check` slash command

JSONL 契約（M4 黃金比對目標，欄位形狀）：`rule_id, severity, path, line?, field?, message, evidence, suggested_action, source_rule, target?, resolved_to?, collision_members?, fingerprint`。排序 `path → line → rule_id`；fingerprint＝FNV-1a over（rule_id, path, target），各段後接 `0x1f` 分隔位元組，16 位小寫 hex；exit code 0 / 1 / 2。byte-exact 目標：`kura/tests/snapshots/conformance__jsonl_output.snap` 與 `conformance__coverage_report.snap`。

### 掃描邊界 ≠ 渲染邊界

kura 預設**不掃** `System/`、`Diagrams/`、`Views/`（`--all` 才掃）。v2 的渲染面比 kura 掃描面大（報告、簡報都要讀）；但 M4 做 `yomihon check` 對齊時，必須複製 kura 的掃描邊界，否則 JSONL 比不齊。

### git 是審計層

vault 是 git repo。v2 每次 status 轉移一個 commit（牆 1）——這讓一切可逆，也是院子敢開大的前提。**不要另建 status 歷史表：`git log` 就是歷史。**

### 寫入管線全景

hermes 走 worktree branch → QA-Gate 三層（kura → Codex → Claude）→ 只有 Claude merge → **只有 Koopa 按 ready**。v2 的 status-flip 是這條鏈的人類終端介面，不是旁路。

### 隱私線（現況：尚無專文）

vault 目前**沒有** privacy / outbound 政策文件，也還沒有 diary type——這兩項是 vault 側的待辦（在 hermes Kimi lane 開動前要定）。對 v2 的含義：索引裡遲早會有永不出站的內容，牆 2 因此是**前置條件**而非附加功能。

---

## 第四層：建造者的閱讀順序（真檔案，非轉述）

1. **資料模型**：`System/schemas/vault-schema.toml` → `System/schemas/Note-Schema.md` → `System/schemas/Vault-Architecture.md`
2. **系統哲學與判官規格**：`System/Vault-Index.md` → `System/Koopa-Knowledge-Compiler.md` → `System/vault-guard-spec.md`（注意：spec 檔名仍是舊名，工具已改名 kura）＋ `System/kura-field-log.md`
3. **人、分工、閘**：`System/agent-guides/about-koopa.md` → `collaboration-charter.md` → `QA-Gate.md` → `Japanese-Companion-Guide.md`
4. **參考實作三件套**：`~/go/src/github.com/koopa0/yomihon/internal/markdown/parser.go`（v1，方言處理）＋ `kura/src/graph.rs`、`src/wikilink.rs`（連結解析 spec）＋ `~/koopa0.dev/frontend/src/app/core/services/markdown.service.ts`（既有 component；untrusted 前提，情境不同）
5. **真內容抽樣（先讀真檔再寫第一行渲染碼）**：
   - `Writing/lessons/japanese/L20 普通形と常体.md`（HTML-ruby 課文）
   - `Writing/lessons/golang/Garbage Collection Guide.md`（corrections ledger）
   - `Maps/Go 課綱.md` ＋ `Maps/大家的日本語 初級I 學習路徑.md`（兩種課綱結構）
   - `System/reports/kura-vault-check.md` ＋ `System/reports/daily-briefing/latest.html`（報告面）
