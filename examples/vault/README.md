# 先選一本書

從 repository 根目錄執行：

```sh
yomihon examples/vault
```

在終端印出的本機位址開啟首頁，選擇課程：

- [把問題讀清楚](Notes/Books/把問題讀清楚.md)：五課繁中主線，從一個過度概括的說法開始，查證、找反例，再改寫解釋。一課 Go 支線可選讀。
- [在圖書館讀日文](Notes/Books/在圖書館讀日文.md)：兩段原創對話，練習物品的位置與閱讀安排；有讀音、朗讀與句型替換。
- [讀懂 yomihon](Notes/中文/讀懂%20yomihon.md)與 [Reading yomihon](Notes/Reading%20yomihon.md)：繁中與英文查閱手冊，解釋契約與啟動方式。

[作者實驗室](Notes/作者實驗室.md)提供副本練習。不要在共用 demo 留下個人閱讀紀錄；日記和報告中的人物、日期都是教學情境。

這份範例刻意保留三項 CLI 診斷：故障 frontmatter 的 `schema.enum`，以及 Wikilinks in this dialect 中的 `link.broken`、`link.section_missing`。`check --deny warn` 和 `--deny error` 都會退出 1。它們位於查閱案例，不在新書的必讀主線。

維護教材時，請看書庫外的[功能與驗收對照](../README.md)。契約位於 `System/schemas/vault-schema.toml`；複製到自己的書庫前，逐項決定目錄、欄位、隱私與狀態規則。這份 README 依契約的 `scan.skip_basenames` 略過，供 repository 讀者查閱。
