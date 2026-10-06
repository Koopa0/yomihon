# 做一門課

[English](authoring.md) | 繁體中文

準備文字編輯器與 [yomihon 執行檔](https://github.com/koopa0/yomihon/releases)。任何 Markdown 資料夾都能閱讀與搜尋；要排成課程，還需要契約，宣告哪些筆記是課文、哪些是學習路徑。

## 取得入門契約

開啟你安裝的執行檔所屬版本的發佈頁，下載 **Source code** 壓縮檔。取出 `examples/vault/System/schemas/vault-schema.toml`，複製到筆記資料夾裡的 `System/schemas/vault-schema.toml`。不必安裝 Go，也不必 clone Git repo。例如 [v0.2.0 的入門契約](https://raw.githubusercontent.com/koopa0/yomihon/v0.2.0/examples/vault/System/schemas/vault-schema.toml) 適用於 v0.2.0；若你的版本不同，請改用該版本的 tag。從原始碼編譯時，使用[同一份 checkout 的入門契約](../examples/vault/System/schemas/vault-schema.toml)。

入門契約是讓你調整的起點。請逐一核對筆記類型、欄位、生命週期轉換、掃描目錄與隱私目錄是否符合你的筆記。尤其 `privacy.never_egress_dirs` 會讓那些目錄不出現在指令輸出裡，但不會把它們從本機閱讀介面隱藏。yomihon 只讀取契約，不寫入它。

下面的小範例先保留入門契約原樣。在筆記資料夾建立 `Concepts/`、`Inbox/`、`Lessons/`、`Maps/` 與 `Notes/`，對應入門契約的掃描目錄。第一、第二與第四個目錄可以先留空，再把下面四個檔案存入 `Notes` 與 `Lessons`。範例沿用英文檔名，wikilink 也寫這些檔名；frontmatter 的 title 不能代替檔名。

## 排好課程

### `Notes/First course.md`

```markdown
---
title: First course
type: study-path
status: ready
lang: en
---

## Main line {sequence=primary}
- [[Start]]
    - More detail {sequence=local}
        - [[Extra]]
- [[Continue]]

## Reference {sequence=none}
- [[Start]] — revisit when needed
```

最短的主線只有三行：一個以 `{sequence=primary}` 結尾的標題，加上兩行課文。選讀支線要在 `Start` **底下**多開一行；`{sequence=local}` 標在這行，不能直接標在 `Start` 上。

| 標記 | 讀者會看到什麼 |
| --- | --- |
| `primary` | 按寫下的順序走主線，這些課文算入課程總數。 |
| `local` | 選讀支線，有自己的數量與前後順序，不接入主線。 |
| `none` | 查閱資料，不算課程數量，也不參與前後課順序。 |

標記放在 H2–H6 標題的末尾，或開啟子清單的那行末尾。每條分支都要宣告自己的角色。課文那行以一個 wikilink 起頭，後面可以加說明。

## 寫課文

### `Lessons/Start.md`

```markdown
---
title: Start
type: lesson
status: ready
domain: yomihon
slug: first-course-start
level: fundamental
lang: en
---

Read this first. What do you already know about the subject?
```

### `Lessons/Continue.md`

```markdown
---
title: Continue
type: lesson
status: ready
domain: yomihon
slug: first-course-continue
level: fundamental
lang: en
---

Try one small example using what you read in Start.
```

### `Lessons/Extra.md`

```markdown
---
title: Extra
type: lesson
status: ready
domain: yomihon
slug: first-course-extra
level: fundamental
lang: en
---

An optional explanation for readers who want more detail.
```

這些值由入門契約宣告。換成自己的課程時，請選你調整後的契約允許的值。每課的 slug 要獨一且保持穩定，`lang` 則寫課文的語言。

## 閱讀與檢查

執行 `yomihon ~/notes`，把路徑換成你的資料夾。開啟 <http://127.0.0.1:9610/paths>，再點 **First course**。主線有兩課，另有一課選讀；Start 的下一課是 Continue，Extra 留在支線裡。

執行 `yomihon check --root ~/notes`，檢查失效連結、frontmatter 問題與未宣告的課程分支。檢查不改檔案，請在編輯器裡修正。資料夾沒有契約時，這個指令會以 exit 2 拒絕；`yomihon ~/notes` 仍可閱讀，<http://127.0.0.1:9610/health> 仍列出連結問題。

更多寫法可參考[學習路徑語法參考](../skills/yomihon/references/study-paths.md)。
