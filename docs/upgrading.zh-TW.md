# 升級說明：把書庫從舊格式遷移過來

[English](upgrading.md) | 繁體中文

`System/schemas/vault-schema.toml` 的第一行是 `schema_version = "1"`，或 yomihon 說「這份書庫規則使用舊格式」時，請看這一頁。遷移分兩部分：先改規則檔，再改筆記。

yomihon 只讀取你的書庫。下面每一項修改都由你自己動手，在編輯器裡改，或用你自己的腳本改。請在 Git repo 裡或在複本上進行，這樣可以先看過改了什麼，再決定要不要留下。

## 遷移之前

閱讀與搜尋照常運作，和沒有規則檔的資料夾一樣。`yomihon check`、`coverage`、`exists` 與 `marks` 會以結束碼 2 拒絕這個書庫，訊息是「privacy authority unavailable」。針對舊格式建置的 yomihon 同樣讀不了遷移後的規則檔，所以請在安裝新執行檔的同一天完成遷移。做完之後重新啟動 yomihon：它只在啟動時讀一次規則檔。

## 1. 規則檔

先把第一行改成 `schema_version = "2"`，再依下表搬動每個鍵。

| 舊格式 | 現在 |
| --- | --- |
| `[scan] knowledge_dirs` | `[folders] checked` |
| `[scan] skip_basenames` | `[folders] skipped_files` |
| `[scan] exclude_dirs` | `[folders] ignored` |
| `[scan] no_frontmatter_is_legal` | `[folders] frontmatter_optional` |
| `[rules] domain_equals_folder_under` | `[folders] domain_from` |
| `[artifacts] non_instance_dirs` | `[folders] templates` |
| `[navigation] journal_dir` | `[folders] journal` |
| `[navigation] path_types` | 清單裡每個類型各有一個 `[types.<name>]` 區段，寫 `role = "course"` |
| `[navigation] map_types` | 清單裡每個類型各有一個 `[types.<name>]` 區段，寫 `role = "map"`；空清單直接刪掉 |
| `[navigation] answer_type` | 那個類型的區段寫 `role = "thought"`，並在它的 `fields` 裡列 `based_on` |
| 名為 `lesson` 的類型 | 一個 `[types.lesson]` 區段，寫 `role = "lesson"`，並在它的 `fields` 裡列 `slug` |
| `[rules] slug_pattern` | 移到 `lesson` 類型區段的 `slug_pattern` |
| `[fields] known` | `[fields] common`，只留下下面「2. 筆記」列出的欄位 |
| `[fields] lesson_only` | `lesson` 類型區段的 `fields`，同樣只留下那份清單裡的欄位 |
| `[fields] required` | 保留，但只能寫每篇筆記都能帶的欄位（見下面「2. 筆記」）。把 `title`、`status` 與 `domain` 拿掉 |
| `[enums] type`、`domain`、`level` | 不變。領域現在拿筆記所在的資料夾名稱來比對 |
| `[rules] concept_requires_provenance`、`planned_gap_marks`、`planned_inline_marks` | 不變 |
| `[privacy] never_egress_dirs` | 不變 |

類型只有在要給它角色、它自己的欄位、必填欄位或 slug 格式時，才需要 `[types.<name>]` 區段。寫在 `[enums] type` 裡的名稱本來就是一個類型。

沒有替代的要刪掉：`[enums.status]`、每一個 `[[lifecycle]]`、`[supersession]`、鍵都搬完之後的 `[navigation]`、`[fields] required_inbox`、`domain_exempt_types` 與 `status_group`、`[rules] forbid_tag_with_slash`、列舉 `source_kind`、`source_provider` 與 `map_kind`，以及最上層的 `aligned_with` 與 `generated_at_must_match`。規則檔只要還帶著其中任何一個，這一版 yomihon 就拒絕讀取。

一份小檔案，改前與改後：

```toml
schema_version = "1"

[enums]
type = ["lesson", "study-path"]
domain = ["go", "sql"]

[enums.status]
note = ["draft", "ready"]

[fields]
required = ["title", "type", "status", "domain"]
known = ["title", "aliases", "type", "status", "domain", "tags", "lang"]
lesson_only = ["slug", "level", "sources", "go_version"]

[rules]
slug_pattern = '^[a-z0-9-]+$'
domain_equals_folder_under = ["courses"]

[scan]
knowledge_dirs = ["courses"]
no_frontmatter_is_legal = true

[navigation]
path_types = ["study-path"]
map_types = []

[artifacts]
non_instance_dirs = ["templates"]

[privacy]
never_egress_dirs = []

[[lifecycle]]
status = "draft"
applies_to = ["*"]
initial = true
from = ["ready"]
owner = ["author"]
```

```toml
schema_version = "2"

[folders]
checked = ["courses"]
templates = ["templates"]
domain_from = ["courses"]
frontmatter_optional = true

[enums]
type = ["lesson", "study-path"]
domain = ["go", "sql"]

[fields]
common = ["type", "aliases", "draft", "lang"]
required = ["type"]

[types.lesson]
role = "lesson"
fields = ["slug", "level", "verified_with"]
slug_pattern = '^[a-z0-9-]+$'

[types.study-path]
role = "course"

[privacy]
never_egress_dirs = []
```

`sources` 與 `go_version` 不在 `lesson` 的欄位清單裡了，因為筆記不再寫它們；內容要搬到哪裡，見「2. 筆記」。`verified_with` 取代了 `go_version`。

## 2. 筆記

筆記只能帶這些鍵：`type`、`aliases`、`draft`、`based_on`、`lang`、`created`、`slug`、`level`、`prerequisites`、`verified_on` 與 `verified_with`，另外還有 Obsidian 自己的 `tags` 與 `cssclasses`。`type`、`draft`、`aliases`、`tags` 與 `cssclasses` 不必列出；其他的鍵都要先列在 `[fields] common`，或列在所屬類型的 `fields`，筆記才能寫。清單以外的鍵，`yomihon check` 一律以 `schema.unknown_key` 回報。

對 `folders.checked` 所列資料夾裡的每一篇筆記：

| 筆記帶著的鍵 | 怎麼處理 |
| --- | --- |
| `title` | 標題就是第一個 `#` 標題。確認筆記有一個寫著這些字的 `#` 標題，沒有就補上，再刪掉這個鍵。沒有 `#` 標題的筆記，標題會用檔名 |
| `domain` | 刪掉。領域是 `[folders] domain_from` 所列根目錄底下的第一層資料夾。舊值和資料夾不一致時，以資料夾為準：把筆記搬走，或改資料夾的名稱 |
| `status: draft`，或任何表示筆記還沒寫完的狀態 | 改寫成 `draft: true` |
| `status: ready`，或其他任何 `status` | 刪掉那一行。沒有 `draft` 就是完成 |
| `status: archived` | 已經沒有封存這個狀態。如果是另一篇筆記取代了它，就刪掉這篇，並把它的檔名加進取代者的 `aliases`；什麼時候這樣做才安全，見[取代一篇筆記](authoring.zh-TW.md#取代一篇筆記)。不是這種情況，就只刪那一行 |
| `sources` | 還不在內文裡的，搬進內文，例如文末加一節「## 來源」，再刪掉這個鍵。指向書庫裡另一篇筆記的參照，寫進 `based_on: ["[[Note]]"]`，要加引號 |
| 筆記查證所用工具的版本，例如 `go_version` | 寫進 `verified_with`，例如 `verified_with: ["Go 1.26"]`，並在該類型的 `fields` 裡列 `verified_with` |
| `replaces`、`supersedes`、`superseded_by`、`archived`、`published`、`topics`、`updated`、`source_kind`、`source_provider`、`map_kind` | 刪掉。讀者還該看到的，寫進內文 |
| 上面清單以外的任何鍵，例如計數、預估時數或工具的版本戳記 | 刪掉；讀者該看到的話，就寫進內文 |
| 上面清單裡的鍵 | 保留 |

會寫入舊鍵的腳本與範本，會把舊鍵再帶回新筆記。請在同一次修改裡一併改掉，先改 `[folders] templates` 所列的資料夾。

## 3. 檢查

```bash
yomihon check --root <vault> --format human --deny error
```

看結束碼。

- **0**：沒有嚴重度為 error 的發現，書庫已遷移完成。
- **1**：還有 error。每一個 `schema.unknown_key` 都指出是哪個鍵、哪一篇筆記還帶著它。修好之後再跑一次。
- **2**：yomihon 仍然無法使用規則檔。執行 `yomihon serve --root <vault>`：啟動時它會記下 `rules file unavailable` 與原因，原因會指出讀不懂的鍵；第一行不是 `schema_version = "2"` 時，記的是 `rules file is for another schema version`。

不屬於 `schema.*` 的發現，例如找不到目標的連結，是書庫本來就有的，遷移之前就在了。
