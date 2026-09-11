package wording

// The sentence-pattern practice a lesson can carry.

var (
	SlotMachineLabel = both("句型練習", "Sentence practice")
	SlotMachineLede  = both(
		"替換詞語，句子會重新組合並朗讀。",
		"Swap a word and the sentence rebuilds and reads itself aloud.",
	)
	SlotMachineSpeak   = both("朗讀這組句子", "Read this sentence aloud")
	SlotMachineShuffle = both("隨機更換組合", "Shuffle the words")
)

// A file's own facts, for the page shown where yomihon has no reader for it.
var (
	FileInfoLabel  = both("檔案資訊", "File information")
	ByteSingular   = both("1 位元組", "1 byte")
	BytesSuffix    = both(" 位元組", " bytes")
	ByteSizeFmt    = both("%.1f %s（%s 位元組）", "%.1f %s (%s bytes)")
	FileKindSource = both("原始碼", "Source code")
	FileKindImage  = both("圖片", "Image")
)
