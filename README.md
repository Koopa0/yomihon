# yomihon v2（読本）

yomihon 是 Koopa 私有知識庫（`~/obsidian`）的本地閱讀與裁決介面。v2 是它的第二個形態：從「日文課程的閱讀器」長成「整座 vault 的閱讀與裁決介面」。

它做兩件事：

- **閱讀**：把整座 vault 變成可讀、可導航、可搜尋的介面——課文、概念、報告、課綱，全部。
- **裁決**：讓「認證一則筆記」在讀完的地方一鍵完成。`ready` 只有 Koopa 能按，而 yomihon 就是那顆按鍵。

它永遠不做的事：

- 永不對外。單人、local-only，沒有部署這回事。
- 永不代寫、永不「修好」筆記。判官是 kura，改檔的是人。
- 除了 `status` 一個欄位，永不寫入 vault。

生態位：kura 是 corpus 判官（服役至 parity）；yomihon v1（`~/go/src/github.com/koopa0/yomihon`）凍結服役至 parity，屆時由 v2 吸收；koopa0.dev 負責對外。v2 是這一切的人類終端。

**狀態**：設計完成（見 `docs/`），實作未開始。folder 暫名 `yomihon-v2`，v1 退役後回收正名。
