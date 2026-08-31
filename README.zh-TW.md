<h1><img src="assets/brand/yomihon-mark.svg" width="36" height="36" alt="" aria-hidden="true"> yomihon</h1>

[English](README.md) | 繁體中文

[![CI](https://github.com/koopa0/yomihon/actions/workflows/ci.yml/badge.svg)](https://github.com/koopa0/yomihon/actions/workflows/ci.yml)
[![授權條款：MIT](https://img.shields.io/badge/license-MIT-blue?style=flat)](LICENSE)

**專為 Markdown 知識庫打造的專注閱讀空間。**

yomihon 把一個筆記資料夾轉化成安靜、在本機運作的閱讀體驗。你可以閱讀長篇
Markdown、讓連結與脈絡留在手邊、在需要時沿著學習結構前進，並判斷一篇筆記
是否已經就緒——不必把知識搬進另一個資料庫。

[![yomihon 以繁體中文介面閱讀範例知識庫的一篇筆記：左側是導覽欄與學習路徑，中間是文章，右側是狀態與它唯一能走的轉換、本頁章節，以及連到這篇的筆記](.github/media/reading-zh-TW.png)](.github/media/reading-zh-TW.png)

*介面跟著你選的語言走——這一張是繁體中文，也是預設。你的筆記不會被翻譯：
每一篇都保留作者書寫時使用的語言。畫面來自這個 repository 隨附的範例知識庫，
可用 `make screenshots` 重新產生。*

> [!WARNING]
> yomihon 仍在積極開發中；第一個穩定版本推出前，產品與介面仍可能有明顯變動。

## 為什麼選擇 yomihon

- **不需遷移就能閱讀。** 普通 Markdown 仍是普通 Markdown。筆記、連結、圖片、
  callout、Mermaid 圖、PDF 與報告會被組合成連貫的閱讀畫面。數學記法會照原樣
  顯示、不做排版；`$…$` 內的字元依一般 Markdown 強調規則處理。
- **在對的時候看見對的結構。** 導覽、標題、反向連結與關鍵字搜尋都留在正文
  附近。漸進採用的 Study Path 表達有順序的進度，Map 表達關係，Report 仍是
  報告；它們是不同的呈現方式，不是一套包辦所有文件的標記語法。
- **最後的決定留給人。** yomihon 是閱讀器，不是編輯器。它可以依規則推進一篇
  筆記的 `status`；文章修改仍留在你原本使用的寫作工具。
- **兩種語言，一份筆記。** yomihon 用自己的聲音說的每一句話都有英文與繁體
  中文，在標題列切換。你的筆記不會被翻譯：一篇宣告了自身語言的筆記，無論
  外框的介面正在說哪一種語言，它都保持原樣。

## 開始閱讀

你需要 Git 與 Go 1.27.0 以上版本。從原始碼安裝：

```sh
git clone https://github.com/Koopa0/yomihon.git
cd yomihon
go install ./cmd/yomihon
```

接著指定一個 Markdown 資料夾：

```sh
yomihon /path/to/vault
```

開啟 `http://127.0.0.1:9610`。你也可以先進入要閱讀的資料夾，再直接執行
`yomihon`。

yomihon 有兩個環境變數：`YOMIHON_PORT` 指定本機埠號（預設 `9610`；監聽位址
永遠是 `127.0.0.1`）；`YOMIHON_EMBED_KEY` 存放你自己的 embedding 服務憑證，
供明確啟用的語意功能使用——它只用於把隱私契約允許的筆記內容轉成本機搜尋
向量，以及送出你主動發起的語意查詢文字。

普通資料夾不必修改檔案就能閱讀。若要漸進啟用受規範的中繼資料、結構化導覽與
生命週期操作，請在 `System/schemas/vault-schema.toml` 加入知識庫契約。

[`examples/vault`](examples/vault) 是一個現在就能讀的完整小型知識庫——
`yomihon examples/vault`——帶著契約、一條學習路徑、一張地圖、兩課，以及一條
刻意沒寫的連結，好讓你看見診斷長什麼樣子。要複製的就是它的
[契約](examples/vault/System/schemas/vault-schema.toml)。每次執行都有測試掃描
這個知識庫，所以這個範例不會默默失效：它正是「已經有可用知識庫的人再也不會
打開」的那種檔案。

[`AUTHORING.md`](AUTHORING.md) 記錄的是 yomihon 漸進採用的 Study Path
本文語法；Map 與 Report 仍保有各自的文件角色。

## 信任邊界

- **本機、單一使用者。** 伺服器只監聽 `127.0.0.1`，沒有遠端或多人模式。
- **唯一且狹窄的寫入。** 閱讀、呈現、搜尋與診斷都不改動知識庫內容。只有經
  授權的生命週期操作能修改 `status`——只改寫 frontmatter 中的那一行。
- **問題保持可見。** 無效的中繼資料、斷裂或語意不明的連結都會成為診斷；
  yomihon 不會猜測，也不會默默修理筆記。
- **寫入綁定你讀到的內容。** 狀態變更會帶著頁面顯示給你的那份內容的識別值。
  若筆記在這中間於磁碟上被改過，這次寫入會被拒絕，而不是套用到你從未看過的
  版本——而且在你按下任何按鈕之前，閱讀頁就會先告訴你這篇筆記已經有新版本。
- **網路功能必須主動啟用。** 一般閱讀與關鍵字搜尋留在本機。語意功能只在明確
  操作時執行，使用你自己的服務憑證，並遵守知識庫的隱私契約。

## 平台支援

| 功能 | macOS | Linux | Windows |
|---|---:|---:|---:|
| 閱讀、導覽、診斷、關鍵字搜尋 | 支援 | 支援 | 支援 |
| `status` 寫入與語意資料生成 | 支援 | 支援 | 不支援 |

## 專案資訊

問題與功能建議請到 [GitHub Issues](https://github.com/Koopa0/yomihon/issues)。
安全性問題請透過
[GitHub 私密漏洞回報](https://github.com/Koopa0/yomihon/security/advisories/new)
提交，不要開公開 issue。

貢獻以 `make verify` 為驗收閘門；`make tools` 會安裝閘門要求的固定版本 Go
分析工具。非 `go install` 產出的執行檔（例如 Homebrew 版本）沒有模組版本
戳記，無法通過精確版本檢查。其餘先決工具（Tailwind CSS 獨立 CLI、
ShellCheck、供開發期前端 lint 使用的 Node.js 與 npm、瀏覽器探針使用的
Chrome）版本固定在 Makefile 與 CI 工作流程中。

yomihon 採用 [MIT License](LICENSE)。重新散布的字型與前端資產列於
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md)。
