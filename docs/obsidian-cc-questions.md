# 給 obsidian Claude Code 的四件事

> 用途：Koopa 帶去 vault repo 的 session 貼給 obsidian CC。回覆請落檔（vault 側自定落點），Koopa 會帶回來。
>
> 請求方脈絡（兩行）：蔵人 kurodo（`~/go/src/github.com/koopa0/kurodo`）是 vault 的本地閱讀＋裁決介面，也是 yomihon 與 kura 的繼承者——kura 退役閘（M4）達成前 kura 一行不動。kurodo 的 schema 理解唯一來源是 `System/schemas/vault-schema.toml`，不硬編碼。

## 1. CLI 需求清單（對 M4 介面設計最有價值）

kurodo M4 會吸收 kura 的 `check` / `exists` / `coverage`，JSONL 契約、`--format`、exit code 與掃描邊界維持 byte-compatible。想知道 vault 側的**真實用法**：

- 你（與 hermes 管線之外的手動場景）實際用哪些指令與 flags？頻率？（`check --deny`、`exists` 當 dedup oracle、`coverage`、`--format md`、`--all`…）
- 有沒有「每次都要繞路」的缺口？候選：`backlinks <note>`（反向連結／blast radius）、orphans 查詢、frontmatter query（如「列出所有 status=draft 的 lesson」）、其他。
- 只收集需求排優先序，不是承諾清單；擴展走 kurodo 的院子排程。

## 2. Vault-Index 的入口語意

kurodo 的首頁（M2）想直接沿用 `System/Vault-Index.md` 的分區當資訊架構（四張看板、缺口帳、domain MOC 入口），而不是發明自己的首頁。請指出：

- 哪些分區是**穩定入口**（值得做成 kurodo 首頁區塊）、哪些是暫時性的？
- `Views/` 五個 `.base` 檔的角色定位——kurodo v0 對 Bases 只做「連回 Obsidian 開啟」，這樣分工對嗎？

## 3. diary type ＋ 隱私出站線（vault 側 pending，請起草）

kurodo 是 local-only（loopback 寫死），會索引**整座 vault**——所以「哪些內容永不出站、永不進 agent context」需要在 hermes 的雲端 lane 開動之前有專文。請 vault 側起草：

- `type: diary` 的 schema 決定：落點 folder、status 組、是否進 `[scan]` 範圍。
- 隱私線專文（建議落 `System/agent-guides/`）：哪些 type/folder 允許送雲端腦、哪些永不；kura/kurodo 是否要一條機械可判的規則。

## 4. slug 一頁化（方向已定，請落文）

Koopa 已點頭的四句收斂：**只有 lessons 需要 slug；格式＝namespace 前綴＋編號（`jp-minna-lNN` / `jp-kana-pNN`，Go 課用 plain slug）；一經定稿永不改；其他筆記一律不需要 slug。**

請落成一頁文件，並確認 `vault-schema.toml` 是否需要同步（目前 toml 只有 `slug_pattern`，namespace 前綴慣例還在文件層）。
