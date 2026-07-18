---
type: concept
aliases: [Browser boundary]
---
# Browser boundary

<ruby lang="ja">安全<rt lang="ja">あんぜん</rt><rp>（</rp></ruby><br>

<script>globalThis.noteScriptRan=true</script>

<button onclick="globalThis.noteEventRan=true">unsafe event</button>

<meta http-equiv="refresh" content="0;url=http://BROWSER_BOUNDARY_ATTACKER/meta-refresh">

<img src="http://BROWSER_BOUNDARY_ATTACKER/raw-image" onerror="globalThis.noteEventRan=true">

<link rel="prefetch" href="http://BROWSER_BOUNDARY_ATTACKER/prefetch">

<iframe src="http://BROWSER_BOUNDARY_ATTACKER/frame"></iframe>

<form action="http://BROWSER_BOUNDARY_ATTACKER/form"><button>submit</button></form>

<video poster="http://BROWSER_BOUNDARY_ATTACKER/poster"><source src="http://BROWSER_BOUNDARY_ATTACKER/media"></video>

<style>@import url("http://BROWSER_BOUNDARY_ATTACKER/import"); .hostile { background-image: url("http://BROWSER_BOUNDARY_ATTACKER/style") }</style>

![remote pixel](http://BROWSER_BOUNDARY_ATTACKER/markdown-image)

[explicit external link](http://BROWSER_BOUNDARY_ATTACKER/explicit-link)

BROWSER-BOUNDARY-END
