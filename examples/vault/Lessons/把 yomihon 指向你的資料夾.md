---
title: 把 yomihon 指向你的資料夾
type: lesson
status: ready
domain: yomihon
slug: point-at-your-own-folder
level: fundamental
created: 2026-09-04
lang: zh-Hant
---

拿你自己的資料夾跑跑看。它不建立、不改寫任何東西，資料夾維持原樣。

```sh
yomihon ~/notes
```

開啟 `http://127.0.0.1:9610`，你會看到閱讀頁、從資料夾長出來的導覽、搜尋，以及筆記之間已經接上的連結。

標頭右邊那顆 `EN` 換介面語言。換掉的只有 yomihon 自己說的話；你的字不跟著換，一篇筆記在 frontmatter 的 `lang` 寫的是哪一種語言，它就以那種語言呈現。

還沒出現的，是任何需要知道筆記語意的東西：沒有狀態按鈕，沒有學習路徑，也沒有依類型判斷的診斷。那些要先有[[知識庫契約]]。寫一份的步驟，在英文那條路徑的 [[L02 Add a contract]]。
