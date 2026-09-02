<h1><img src="assets/brand/yomihon-mark.svg" width="36" height="36" alt="" aria-hidden="true"> yomihon</h1>

[English](README.md) | 繁體中文

[![CI](https://github.com/koopa0/yomihon/actions/workflows/ci.yml/badge.svg)](https://github.com/koopa0/yomihon/actions/workflows/ci.yml)
[![授權條款：MIT](https://img.shields.io/badge/license-MIT-blue?style=flat)](LICENSE)

**一間放在本機的 Markdown 筆記閱讀室。**

[![yomihon 以繁體中文閱讀範例知識庫的一篇筆記：左側是導覽與學習路徑，中間是文章，右側是這篇筆記的狀態、章節，以及連到它的筆記](.github/media/reading-zh-TW.png)](.github/media/reading-zh-TW.png)

## 安裝

```sh
go install github.com/koopa0/yomihon/cmd/yomihon@latest
```

需要 Go 1.27 以上版本。

## 使用

```sh
yomihon serve ~/notes
```

接著開啟 <http://127.0.0.1:9610>。任何資料夾都能照原樣讀；
`yomihon serve examples/vault` 可以看見加上知識庫契約之後多了什麼。

## 它做什麼

- 把一個 Markdown 資料夾當成一本書來讀：wikilink、callout、ruby、Mermaid、
  註腳、表格與有語法標示的程式碼。
- 把學習路徑變成課程——課數、上一課與下一課，以及你讀到哪裡（[怎麼寫](AUTHORING.md)）。
- 反向連結、本頁章節與關鍵字搜尋都留在正文旁邊。
- 回報壞掉的地方——無效的 frontmatter、連不到目標的連結、兩個檔案同時應答的
  同一個名字——而且從不動手修。
- 介面說英文或繁體中文；你的筆記保留書寫時使用的語言。
- 只寫一個欄位 `status`，並且沿著你的知識庫宣告的轉換走（Windows 以外的
  平台）。除此之外不寫任何東西，也不送出任何東西：完全不發網路請求。

## 目前狀態

仍在積極開發中；第一個穩定版本推出前，產品與介面仍可能有明顯變動。缺陷請開
[Issues](https://github.com/koopa0/yomihon/issues)，安全性問題請走
[GitHub 私密漏洞回報](https://github.com/koopa0/yomihon/security/advisories/new)。

## 授權

採用 [MIT](LICENSE)。重新散布的字型與前端資產列於
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md)。
