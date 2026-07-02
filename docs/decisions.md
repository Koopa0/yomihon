# 決策記錄

> 每條記「決定＋為什麼」。推翻任何一條都可以，但要跟 Koopa 開新決定，不是在 code 裡繞過。

## D01 命名：蔵人 kurodo

命名走了三步：文机 fuzukue（作廢）→ 沿用 yomihon（folder 暫名 yomihon-v2）→ **定名 kurodo**（2026-07-02，Koopa 裁定）。最後一步的觸發：Koopa 點出 obsidian 側的 agent 也要把這工具當 CLI 用（kura 的 check/exists/coverage 及後續擴展）——本專案是 **yomihon＋kura 兩個工具的共同繼承者**，「読本」只命名了讀那一半。

蔵人（くろうど）：蔵人所是天皇的秘書機構——管文書庫、為主上讀文件、傳達裁決、把關出入。裁決權在主上、執行在蔵人，正是「ready 只有 Koopa 能按」的形式；字面「蔵之人」直接續 kura（蔵）系譜。binary `kurodo`、module `github.com/koopa0/kurodo`。yomihon 保留其名凍結服役至退役閘——無 module 撞名、無 v1/v2 消歧。

## D02 四道牆（見 CLAUDE.md，此處記理由）

危險的軸從來不是「讀多廣」，是「寫多深」和「對外多開」。所以牆立在：寫入面（status 單欄位＋狀態機＋git commit）、網路面（loopback 寫死）、契約面（toml 單源）、誠實面（渲染永不修檔）。牆內功能空間全開。

## D03 範圍：全 vault ＋搜尋，不設功能圍欄

原案的窄 v0（只渲染 Writing/Concepts、不做搜尋）被 Koopa 否決，理由成立：唯讀渲染指向十個資料夾和指向兩個是同一條 pipeline 換輸入清單；搜尋對 400+ 且持續成長的 corpus 是真需求。防第二系統效應的手段改為 D02 的牆＋M1 的縱切出貨閘（「讀完一篇、認證一課」），不是功能清單圍欄。

## D04 不 import yomihon 套件；正確性以 fixtures 轉移

Koopa 的架構決定：實作全新寫，用既有 component 當參考。真正要防的不是重寫，是**同語意兩套實作的無聲漂移**——所以遷移介質從 code 換成測試：yomihon 的 `testdata/lesson.md` 斷言模式、m1-review 截圖、kura 的 conformance snapshots 搬過來當驗收規格。正確性定義不准重新發明。參考實作三件套與各自角色見 CLAUDE.md 表。

## D05 搜尋：確定性優先，升級走三關

v0 = pg_trgm 全文＋結構化過濾。pg_bigm / PGroonga / 語意向量的升級條件一律走 kura-field-log 的三關（真實案例＋反覆＋確定性可判，`System/kura-field-log.md:22`）。2026-06-30 Koopa 已解除硬禁令——門是開的，但要證據。

## D06 PG 只放派生資料

真相＝vault 檔案＋git。DB 可隨時 drop 重建（`kurodo reindex`）。沒有 status 歷史表——git log 就是歷史，別雙重記帳。Migrations 依 Koopa 慣例：pre-release 只改 `001_initial`，不開 002+。DB 用 koopa0.dev 同一 local PG instance 開新 database，M2 才引入。

## D07 status 寫入＝外科手術式單行改寫＋shell git＋Koopa 的 identity

只改 frontmatter 內 `status:` 一行，絕不重新序列化 YAML（會毀格式與註解）。git 走 `os/exec` 不走 go-git：審計層要與人手 git 完全同語意，且依賴自證必要。寫前 content_hash 防競態。**commit author 用 Koopa 本人的 git identity**（vault 的 git config），message 註明 `(via kurodo)`——審計語意「是 Koopa 按的」（2026-07-02 Koopa 裁定）。

## D08 狀態機執行是 kurodo 的貢獻，不是重複 kura

vault-schema.toml 已含 `[[lifecycle]]`（from＋owner）與 slug pattern——「契約先行」的前置工作比預想小。toml 自承 file-scan 驗不了 from→to（看不到前態）；kurodo 是互動式寫入者，讀得到現態，天然補上這塊。**契約仍缺的**：renderability 需求不在 toml——v0 不需要（牆 4 本來就要求容錯渲染）；若日後出現可判定的 renderability 契約，加進 toml，不寫進 code。

## D09 薄 harness

一頁 CLAUDE.md＋pointer 回 go-spec；AGENTS.md 只是指針；不 vendor `.claude/` 全套、不建 `.codex` 鏡像（kura 的鏡像帶著壞字串是前車之鑑）。`.golangci.yml`、`.lsp.json`、`sqlc.yaml` 從 go-spec 同步（只改 module path）。goilerplate 只當 UI blocks 來源——它的 boilerplate 是 service/repository 分層，與 go-spec 教義相反，結構不拿。

## D10 v0 出貨標準

「Koopa 在 kurodo 裡讀完一篇長文並就地認證一課」。那一鍵攻擊的是整個系統當前的真瓶頸（裁決摩擦）。其他功能的生長順序由使用中的痛決定，不預先排程。

## D11 退役是憑證據的閘，不是日期

yomihon：五互動＋fixtures＋截圖驗收＋兩週真實學習 → 退役，SSG 併入 `kurodo export`。kura：JSONL 逐位元組黃金比對＋snapshots＋掃描邊界複製＋四管線切換 → 退役。達成前：yomihon 凍結服役（只修 bug，已 tag `v1.0.0`）、kura 一行不動地當閘。yomihon SPEC §13 的 `yomihon check` 計畫作廢，lint 職責歸 `kurodo check`。

## D12 設定面極小

`KURODO_ROOT`（vault 路徑，預設 `~/obsidian`）/ `KURODO_PORT`（預設 9610，く9ろ6ど10 的諧音，無深意）/ `KURODO_DB`（M2 起）。沒有 bind address 設定——那是牆 2。

## D13 UI 寫入面開放全部合法轉移

Koopa 裁定（2026-07-02）：凡 toml lifecycle 允許的 from→to 都能在 UI 按（同一條寫入路徑、同樣驗證＋commit），`ready` 鍵視覺上突出。理由：裁決摩擦全面下降，而寫入面風險不隨轉移種類增加——都是 status 單欄位（牆 1 不變）。

## D14 kurodo 同時是 agent 的 CLI

obsidian 側的 Claude Code（與 hermes 管線）是 `check` / `exists` / `coverage` 的直接消費者——輸出格式（JSONL 契約、`--format`、exit code）因此是**對外介面**，不是內部細節，對齊 kura 是硬需求（M4 閘）。後續擴展（backlinks、frontmatter query、MCP server…）依 vault 側真實用法提案，走院子排程。
