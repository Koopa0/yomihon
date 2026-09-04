<h1><img src="assets/brand/yomihon-mark.svg" width="36" height="36" alt="" aria-hidden="true"> yomihon</h1>

[English](README.md) | 繁體中文

[![CI](https://github.com/koopa0/yomihon/actions/workflows/ci.yml/badge.svg)](https://github.com/koopa0/yomihon/actions/workflows/ci.yml)
[![Go 版本](https://img.shields.io/github/go-mod/go-version/koopa0/yomihon?style=flat)](go.mod)
[![授權條款：MIT](https://img.shields.io/badge/license-MIT-blue?style=flat)](LICENSE)

**把你整理好的 Markdown，變成一本好讀的書。**

[![yomihon 的閱讀頁：左欄是課程與目前的一課，中間是正文，右欄是本頁章節與引用它的筆記](.github/media/reading-zh-TW.png)](.github/media/reading-zh-TW.png)

yomihon 把一個 Markdown 資料夾讀成一本書。學習路徑是一門課，有章節，標出你在哪一課；每篇筆記旁邊放著它的章節，還有引用它的筆記；哪裡壞了，就在原處說明。它在你的機器上執行，不改你的任何一個字。

## 安裝

```sh
go install github.com/koopa0/yomihon/cmd/yomihon@main
```

需要 Go 1.27 以上。

## 使用

```sh
yomihon ~/notes
```

接著開啟 <http://127.0.0.1:9610>。任何資料夾都能直接讀；`yomihon examples/vault` 是一個帶契約的範例知識庫，可以看見契約多給了什麼。替 yomihon 讀的知識庫寫筆記的 agent，先讀這份 skill：[`skills/`](skills/)。

## 它做什麼

- **讀。** wikilink、callout、註腳、表格、Mermaid、程式碼、ruby，照作者寫的呈現。版面為長篇閱讀而設，中日文優先；亮暗兩種桌面，三段字級。
- **學。** 學習路徑是一門課：課數、上一課與下一課、你在哪一課。振假名可開可關；標了朗讀的段落，用它自己的語言唸。範例知識庫裡有[一條學習路徑](examples/vault/Notes/中文/讀懂%20yomihon.md)可以照著寫。
- **找。** 全文搜尋，可按資料夾篩選；反向連結與本頁章節就在正文旁邊。點一筆結果，筆記開在找到的那句話上，不是從頭開始。
- **檢查。** 整體狀況頁列出沒有目標的連結、沒有人引用的筆記、兩個檔案共用的名字；每篇筆記帶著自己的診斷；命令列是 `yomihon check`。它只回報，不替你修。
- **報告。** 放在 `System/reports/daily-briefing/` 的簡報，在同一個閱讀室裡隔離打開。
- **兩種語言。** 介面有英文與繁體中文；筆記保留寫下時的語言。
- **你的。** 只綁 `127.0.0.1`，不發任何網路請求，不動你的文字。

## 目前狀態

開發中；第一個穩定版之前，介面還會變。缺陷請開 [Issues](https://github.com/koopa0/yomihon/issues)，安全性問題請走 [GitHub 私密漏洞回報](https://github.com/koopa0/yomihon/security/advisories/new)。

## 授權

[MIT](LICENSE)。
