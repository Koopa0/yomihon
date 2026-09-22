# 範例書庫

執行 `yomihon examples/vault`。首頁提供兩本教材：

- [Go 並行入門](vault/Notes/Books/Go%20並行入門.md)：五課主線，從啟動 goroutine 到固定數量的 worker；逾時處理為選讀。
- [在圖書館讀日文](vault/Notes/Books/在圖書館讀日文.md)：兩段對話、讀音、朗讀與句型替換。

教材直接講解主題。以下對照供維護者查驗功能，不放入課文。

## 功能對照

| 功能與實作 | 書庫中的內容 | 查驗 |
| --- | --- | --- |
| 課程與支線：`internal/sequence`、`internal/nav`、`internal/syllabus` | Go 五課主線；G04 下掛 G06 選讀；日文兩課 | G04 下一課為 G05；G06 不插入主線 |
| 概念與地圖：`internal/lesson/concept.go`、`internal/render/concept.go`、`internal/nav/map.go` | goroutine 生命週期、channel 交接、取消收尾；日文地點助詞 | 課文連結開啟概念頁；五個概念都有地圖掛載 |
| 別名與搜尋：`internal/graph`、`internal/lexical`、`internal/search` | Go 課文的標題、別名與 topics；阻塞及修復兩份程式 | `取消 domain:go` 可找回材料；加 `status:draft` 找到阻塞版本 |
| 來源與比較：`internal/snapshot/basedon.go`、`internal/note/compare.go` | [提早返回的管線](vault/Notes/go/提早返回的管線.md)宣告修復版本為來源 | 從草稿開啟並排比較，核對 send 的退出方式 |
| 章節、區塊與節錄：`internal/render` | G04 引用取消筆記的具體段落 | heading/block 連結與 embed 有效；不把正文引用當成精確 declared-source 支援 |
| 圖解、程式碼、註腳：`internal/render`、`internal/asset` | 本機 [Go 管線圖](vault/System/assets/go-pipeline.svg)、完整程式、官方文件來源 | 本機圖不依賴網路；完整程式可執行 |
| 日文朗讀：`internal/render/tts.go`、`internal/syllabus/listen.go` | J01/J02 的四段日文及 ruby | 讀音可切換；語音使用 ja；實際發聲取決於瀏覽器語音 |
| 句型練習：`internal/lesson/slot.go`、`internal/note/handler.go` | 找書位置九種組合、閱讀場所六種組合 | 以 slug 對應，日文與譯文同步更新 |
| 日記與報告：`internal/nav/nav.go`、`internal/report` | 9/22 的取消案例日記、Markdown 回顧、HTML 案例 | 當日內容可開啟；HTML 自含 CSS、無 script 或外部資源 |
| 待續、更新與偏好：`internal/mark`、`internal/note/freshness.go`、`internal/preference` | 可返回的完整課文與長程式 | 保存位置後由首頁返回；副本外部修改出現提示；窄幅與大字可讀 |
| 契約、診斷、替代與隱私：`internal/schema`、`internal/judge`、`internal/status` | 中英手冊、刻意故障、舊／新啟動說明 | 保留診斷清單、supersession 關係與 Diary 隱私範圍 |

Go 教材在範例契約的 domain 清單新增 `go`。欄位、狀態與生命週期規則不變。

## 刻意保留的診斷

`yomihon check --root examples/vault --format json --all` 列出三項：

| 規則 | 檔案 |
| --- | --- |
| `schema.enum` | `Notes/A note with a fault in its frontmatter.md` |
| `link.broken` | `Notes/Wikilinks in this dialect.md` |
| `link.section_missing` | `Notes/Wikilinks in this dialect.md` |

不加 `--deny` 退出 0；`--deny warn`、`--deny error` 都退出 1。一般教材不應增加診斷。`coverage --format json` 應有五個概念全部 mounted：go 三個、japanese 一個、yomihon 一個。

## 在副本查驗

```sh
lab=$(mktemp -d)
cp -R examples/vault/. "$lab/"
yomihon check --root "$lab" --format json --all
yomihon coverage --root "$lab" --format json
yomihon "$lab"
```

開啟 G04 並留下待續位置，再用編輯器修改副本中的課文。原頁應提示更新；首頁待續入口應指出內容已變更。不要修改共享 demo 的狀態或留下個人紀錄。

精確 declared-source 定位（#469、#652）、答案筆記與疑問記號（#557）、首頁帶回問題（#558）仍屬後續功能。三篇 published/lifecycle 說明由 #661 修正。
