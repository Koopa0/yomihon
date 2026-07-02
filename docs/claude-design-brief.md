# Claude Design Brief — 蔵人 kurodo

> 用途：Koopa 帶去給 Claude design 的自足式需求。設計方回傳的交付物見文末。
> 產品脈絡一句話：單人、本機、永不對外的知識庫閱讀與裁決工作台（Go + templ server-rendered）。

## 產品與使用者

kurodo（蔵人）是一位工程師每天閱讀自己知識庫的地方：技術長文（zh-TW）、日文課文（含大量 `<ruby>` 注音）、概念筆記、課綱、系統報告。單一使用者，桌面為主，一次閱讀 20 分鐘到 2 小時。核心動作：**讀完一篇，就地按下認證鍵（status 轉移）**。氣質方向由 Koopa 當面補述——他有明確的美學語彙，請直接與他對談，不要替他預設。

## 頁面（三張關鍵頁）

1. **閱讀頁**（最重要）：三欄 docs shell——左側欄（生命週期資料夾樹＋課綱樹＋報告區，可收合）；中央 prose 內容欄；右欄＝TOC＋frontmatter/status 面板＋診斷欄。
2. **課綱頁**：一份學習路徑（部→模組→課）的樹狀導航，顯示每課 status。
3. **搜尋面板**：⌘K 呼出的 overlay（輸入框＋結果列表＋type/status 過濾 chips）。

## 元件清單

- prose 排版：H1–H3、段落、清單、GFM 表格、code block（server-side 高亮）、blockquote
- callout 兩桶：note（提示/問題/範例）與 warning（注意/警告），各含標題列＋可摺疊變體（原生 `<details>`）
- status 面板：現況 badge＋轉移鍵組；**`ready` 是唯一 primary 鍵**，要有「按下裁決」的儀式感但不浮誇
- 診斷欄：壞 YAML／斷鏈／撞名的警示卡（琥珀色系），只讀不可操作修復
- corrections ledger：課文「修正記錄」的呈現（claim → fix → source 三段式）
- badge：status（9 種值）、type、domain
- header：品牌字（蔵人）、搜尋觸發、theme toggle

## 排版硬需求（CJK 是第一公民）

- 內文 zh-TW 為主，混排日文課文與英文術語；行高 ≥1.8
- **ruby（振り仮名）規格**：`<ruby>漢字<rt>かな</rt></ruby>` 大量出現；rt 字級約 0.5em；**rt 以 `visibility` 顯隱切換，隱藏時行高與行寬不得變動（零 reflow）**——這是既有產品用血換來的規格
- 片假名詞永遠帶 ruby 且不參與顯隱切換
- 拉丁字型已有 Geist（variable woff2 已購置）；**中日文字型請提案**（含 fallback stack；本機離線可用者優先）
- dark mode：class 策略（`.dark`），所有元件雙色票

## 技術約束

- **交付以 Tailwind CSS v4 為準**：`@theme` design tokens（色彩、字級 scale、行高、圓角、陰影）＋ 2–3 張關鍵頁的 HTML mock（直接寫 Tailwind class 者最佳；高解析圖亦可）
- 無 JS 框架：互動元件必須以原生 `<details>`、`<dialog>` 呈現得體面
- 色彩對比過 WCAG AA（正文與 badge 皆是）
- 骨架可參考 Tailwind Plus「Syntax」docs 模板的資訊架構，但視覺身分必須是自己的，不能讀起來像文件站模板

## 交付物

1. `@theme` tokens（一份 CSS）
2. 閱讀頁＋課綱頁＋搜尋面板的 mock（HTML with Tailwind classes，或高解析圖）
3. ruby／callout／status 面板三個元件的特寫規格
