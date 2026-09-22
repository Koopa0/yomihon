# 範例書庫

這個書庫有兩本可以實際跟著讀的小書，以及查閱用的手冊。從 repository 根目錄執行 `yomihon examples/vault`，開啟終端印出的本機位址，在首頁的課程中選一本。

| 想做什麼 | 從哪裡開始 |
| --- | --- |
| 練習讀懂技術材料、用反例修正解釋 | [把問題讀清楚](vault/Notes/Books/把問題讀清楚.md)：五課繁中主線，一課 Go 選讀 |
| 讀兩段圖書館日文對話 | [在圖書館讀日文](vault/Notes/Books/在圖書館讀日文.md)：兩課，繁中指引、讀音、朗讀與句型替換 |
| 查契約、狀態與啟動方法 | [繁中手冊](vault/Notes/中文/讀懂%20yomihon.md)或 [English companion](vault/Notes/Reading%20yomihon.md) |
| 在副本裡試診斷與檔案更新 | [作者實驗室](vault/Notes/作者實驗室.md) |

兩本書的問題先於解析；回答寫在自己的編輯器或紙上。課程的課數、筆記的 status 和待續位置都不代表理解程度。日記與報告是明確標示的虛構閱讀案例；沒有使用統計或自動評分。

以下是維護者的功能對照，放在書庫之外；課文不必兼任功能清單。書庫內 README 是 repository 的導讀，依現有契約略過正文轉譯；在 reader 內直接由首頁的課程入口進入。

## 功能要服務的閱讀任務

| 能力與程式依據 | 教材與讀者動作 | 驗收重點 |
| --- | --- | --- |
| 課徑、封面、主線與支線：`internal/sequence`、`internal/nav`、`internal/syllabus` | 《把問題讀清楚》按提問、查證、改寫前進；R06 是 R02 的選讀支線 | 五課主線加一課 local；R02 下一課為 R03；參考書目不計課 |
| 地圖與概念：`internal/nav/map.go`、`internal/lesson/concept.go`、`internal/render/concept.go` | [閱讀的工具箱](vault/Maps/閱讀的工具箱.md)按用途回查；課文中的概念連結就地閱讀 | 三張地圖；五個概念都有掛載；一般筆記連結與 lesson 概念浮層分清楚 |
| 名稱與別名：`internal/graph/graph.go` | R02 區分檔名、title、aliases；R06 跑三分支 Go 縮影 | 不把 title 當查找鍵；不聲稱完整路徑永遠唯一；Go 輸出實跑 |
| 來源、引用與比較：`internal/snapshot/basedon.go`、`internal/snapshot/backlink.go`、`internal/note/compare.go` | [我的第一版解釋](vault/Notes/reading/我的第一版解釋.md)與來源並排，圈出過度概括；R03 用章節、區塊引用及節錄縮小證據 | 來源宣告與正文引用各有用途；draft 的單一 declared partner 提供對照；來源的多個伙伴不保證同樣入口 |
| 搜尋、篩選、摘要與章節：`internal/lexical`、`internal/search`、`internal/render/heading.go` | R02 用「候選」找回來源，再加入或移除 folder、type、status 條件；topic 找相關課文 | 讀摘要後開來源核對；條件是 AND；不承諾任意子字串都有可著陸地址 |
| 待續位置與更新：`internal/mark`、`internal/snapshot`、`internal/note/freshness.go` | R05 暫停、回首頁、接續；作者實驗室在副本用編輯器修改文章 | 一書庫一位置、裝置本地儲存；外部修改出現提示；不改 tracked vault 的 status |
| 日文讀音與朗讀：`internal/render/tts.go`、`internal/syllabus/listen.go` | J01/J02 先讀對話，再聽四個標記段落；課程朗讀重聽 | ruby 可隱藏；只標日文段落，不承諾繁中語音；實際發聲仍取決於瀏覽器的日文語音 |
| 句型卡：`internal/lesson/index.go`、`internal/lesson/slot.go`、`internal/note/handler.go` | 找書的位置與安排讀書場所，各用一張有意義的替換卡 | [location](vault/System/slots/library-location.yaml) 的九種、[reading](vault/System/slots/library-reading.yaml) 的六種組合均通順；以 slug 對應課文 |
| 日記、Markdown 與 HTML 報告：`internal/nav/nav.go`、`internal/report` | 八月的初讀與九月的再檢查；週記及 [HTML 回顧](vault/System/reports/daily-briefing/latest.html)呈現主張、反例與修正 | 兩個月份可切換；HTML 自含 CSS、不需 script 或網路；窄幅、暗色及列印可讀 |
| 設定：`internal/preference`、`internal/wording/preferences.go` | [Reading preferences](vault/Notes/Reading%20preferences.md)用同一段文章比較字級、字體、讀音 | 設定不寫進教材；無 scripting 時仍可用 Apply 儲存 |
| 診斷、生命週期、替代與隱私：`internal/judge`、`internal/status`、`internal/schema` | 作者實驗室保留三個負例、舊／新啟動教材關係，分清 check、coverage、exists | 不改契約；archived 是內容歷史；Diary 的 CLI 隱私範圍與本機可讀性不同 |
| 作者方言：`internal/render` | R02 用[本機 SVG](vault/System/assets/name-resolution.svg)釐清顯示與解析；R03 節錄；來源註腳；R05 自查；既有契約 Mermaid、haiku、callout 參考 | 保留表格、程式碼、重點、刪除線、註腳、註解、各類 callout 與既有渲染覆蓋；不把所有語法塞進每課 |

## 刻意保留的診斷

`yomihon check --root examples/vault --format json --all` 應列出以下三項。`--deny warn` 退出 1；不加 deny 則退出 0。`tools/check-fixtures.examples-vault.expected` 固定這份清單。

| 規則 | 檔案 |
| --- | --- |
| `schema.enum` | `Notes/A note with a fault in its frontmatter.md` |
| `link.broken` | `Notes/Wikilinks in this dialect.md` |
| `link.section_missing` | `Notes/Wikilinks in this dialect.md` |

一般新教材不應增加 warn 或 error。`coverage --format json` 應有五個概念全部 mounted：yomihon 四個、japanese 一個。這些數字不是整體狀況頁的總數，也不是學習完成率。

## 維護邊界

範例依據目前 repository 程式；名稱解析的來源筆記另外固定了可查證的 commit。更新程式時，要重讀依據並核對課文，不只替換版本號。Go 片段是標明省略項目的教學模型，不是產品實作。

本次內容不依賴尚未完成的精確 declared-source 預覽（#469、#652）、答案筆記／疑問記號（#557）、首頁帶回未解問題（#558）。正文的 heading/block 連結已可使用，不代表 `based_on` 也已保留同樣的精確位置。搜尋與側欄的待修行為依各自 issue 處理；範例不替產品補寫能力。

既有 L02 的 #656 修正已併入基底；三篇 published/lifecycle 說明由 #661 處理，不屬於這次內容改寫。保留檔名以維持既有 walkthrough、引用與測試；之後整合時仍需核對其文字。
