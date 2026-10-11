package judge_test

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

type agreementHeadingSingleDeclaration struct {
	Body, Private, Selected, HTML, PrivateHTML string
	Native, PrivateNative                      agreementNativeHeadingSource
	Tree, PrivateTree                          agreementNativeProseAddressTree
	Page, PrivatePage                          agreementHTML
	Diagnostics, PrivateDiagnostics            []render.Diagnostic
	Before, After                              []agreementFailure
	Address, PrivateAddress                    agreementNativeAddressPart
	Names                                      []string
	Addresses, PrivateAddresses                map[string]agreementNativeAddressReceipt
}

var agreementHeadingSingleDeclarationReading = sync.OnceValue(func() *agreementHeadingSingleDeclaration {
	const frozen = `{
  "Address": {
    "Native": {
      "Definitions": null,
      "Code": [
        {
          "Kind": "CodeSpan",
          "Parent": "Paragraph",
          "Span": {
            "Start": 102,
            "Stop": 118
          }
        }
      ],
      "Comments": [
        {
          "Start": 149,
          "Stop": 163
        },
        {
          "Start": 177,
          "Stop": 191
        },
        {
          "Start": 263,
          "Stop": 277
        }
      ]
    },
    "Check": {
      "Code": [
        {
          "Start": 69,
          "Stop": 74
        },
        {
          "Start": 102,
          "Stop": 118
        }
      ],
      "Comments": [
        {
          "Start": 149,
          "Stop": 163
        },
        {
          "Start": 177,
          "Stop": 191
        },
        {
          "Start": 263,
          "Stop": 277
        }
      ],
      "Fields": [
        {
          "Span": {
            "Start": 18,
            "Stop": 24
          },
          "Word": "[[A\\]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 69,
            "Stop": 74
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": true,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 107,
            "Stop": 112
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": true,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": 0
        },
        {
          "Span": {
            "Start": 143,
            "Stop": 149
          },
          "Word": "[[#A]]",
          "Targets": [],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 154,
            "Stop": 159
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": true,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 163,
            "Stop": 170
          },
          "Word": "[[A#A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 182,
            "Stop": 187
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": true,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 201,
            "Stop": 206
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 213,
            "Stop": 225
          },
          "Word": "[[A\\|alias]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 250,
            "Stop": 255
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 268,
            "Stop": 273
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": true,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        }
      ],
      "Targets": [
        "A",
        "A",
        "A",
        "A",
        "A"
      ]
    },
    "Fields": [
      {
        "Line": {
          "Start": 90,
          "Stop": 93
        },
        "Token": {
          "Start": 91,
          "Stop": 93
        },
        "Raw": " ^a",
        "Name": "^a",
        "Folded": "^a",
        "Owner": -1,
        "Nodes": null,
        "Code": false,
        "Comment": false
      }
    ],
    "Names": [
      "^a",
      "^a-2",
      "^é"
    ]
  },
  "Addresses": {
    "^a": {
      "Page": false,
      "Judge": false,
      "Found": false,
      "Cut": ""
    },
    "^a-2": {
      "Page": false,
      "Judge": false,
      "Found": false,
      "Cut": ""
    },
    "^agreement-absent-0": {
      "Page": false,
      "Judge": false,
      "Found": false,
      "Cut": ""
    },
    "^é": {
      "Page": false,
      "Judge": false,
      "Found": false,
      "Cut": ""
    }
  },
  "After": [
    {
      "Property": "P1",
      "Identity": "citation-occurrences",
      "Observation": "tuple={SourceRole: Target:A Section: State:} direction=judge-only occurrences=5",
      "Tuple": {
        "SourceRole": "",
        "Target": "A",
        "Section": "",
        "State": ""
      },
      "Direction": "judge-only",
      "Multiplicity": 5,
      "Fragment": "",
      "Cut": "",
      "PagePresent": false,
      "JudgeAccepted": false,
      "ExcerptFound": false
    },
    {
      "Property": "P1",
      "Identity": "citation-occurrences",
      "Observation": "tuple={SourceRole: Target:A\\ Section: State:} direction=page-only occurrences=1",
      "Tuple": {
        "SourceRole": "",
        "Target": "A\\",
        "Section": "",
        "State": ""
      },
      "Direction": "page-only",
      "Multiplicity": 1,
      "Fragment": "",
      "Cut": "",
      "PagePresent": false,
      "JudgeAccepted": false,
      "ExcerptFound": false
    },
    {
      "Property": "P4",
      "Identity": "literal-heading-id",
      "Observation": "id=\"q\" check=missing",
      "Tuple": {
        "SourceRole": "",
        "Target": "",
        "Section": "",
        "State": ""
      },
      "Direction": "page-only",
      "Multiplicity": 1,
      "Fragment": "q",
      "Cut": "",
      "PagePresent": true,
      "JudgeAccepted": false,
      "ExcerptFound": false
    }
  ],
  "Before": [
    {
      "Property": "P1",
      "Identity": "citation-occurrences",
      "Observation": "tuple={SourceRole: Target:A Section: State:} direction=judge-only occurrences=5",
      "Tuple": {
        "SourceRole": "",
        "Target": "A",
        "Section": "",
        "State": ""
      },
      "Direction": "judge-only",
      "Multiplicity": 5,
      "Fragment": "",
      "Cut": "",
      "PagePresent": false,
      "JudgeAccepted": false,
      "ExcerptFound": false
    },
    {
      "Property": "P1",
      "Identity": "citation-occurrences",
      "Observation": "tuple={SourceRole: Target:A\\ Section: State:} direction=page-only occurrences=1",
      "Tuple": {
        "SourceRole": "",
        "Target": "A\\",
        "Section": "",
        "State": ""
      },
      "Direction": "page-only",
      "Multiplicity": 1,
      "Fragment": "",
      "Cut": "",
      "PagePresent": false,
      "JudgeAccepted": false,
      "ExcerptFound": false
    },
    {
      "Property": "P4",
      "Identity": "literal-heading-id",
      "Observation": "id=\"a\" check=missing",
      "Tuple": {
        "SourceRole": "",
        "Target": "",
        "Section": "",
        "State": ""
      },
      "Direction": "page-only",
      "Multiplicity": 1,
      "Fragment": "a",
      "Cut": "",
      "PagePresent": true,
      "JudgeAccepted": false,
      "ExcerptFound": false
    }
  ],
  "Body": "- > [!note] title\n[[A\\]]## <em>A</em>\n  \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 ## <em>A</em>\n ^a\n0\u0060open\n[[A]]\nclose\u0060ref[^n]\n> [!note] title\n[[#A]]<!--\n[[A]]\n-->[[A#A]] ^é\n\n\n<!--\n[[A]]\n-->> [!note] [[A]]\nA\n---\n[[A\\|alias]]É\n| a | b |\n|---|---|\n| [[A]] | ^a |\n<!--\n[[A]]\n-->",
  "Diagnostics": [
    {
      "Kind": "wikilink-broken",
      "Target": "A\\",
      "Message": "wikilink \"A\\\\\" does not resolve to any note or file",
      "Section": "",
      "Block": ""
    },
    {
      "Kind": "risky-fence",
      "Target": "",
      "Message": "wikilink/callout/table syntax found inside a fenced code block; left untouched",
      "Section": "",
      "Block": ""
    }
  ],
  "HTML": "<ul>\n<li>\n<blockquote>\n<p>[!note] title\n<span class=\"wikilink-broken\" title=\"There is no note called &#34;A\\\\&#34; yet\">A\\<span class=\"y-offscreen\"> (There is no note called &#34;A\\\\&#34; yet)</span></span>## &lt;em&gt;A&lt;/em&gt;</p>\n</blockquote>\n<pre class=\"chroma\"><code></code></pre></li>\n</ul>\n<p><a href=\"https://example.invalid/%60%5B%5BA%5D%5D%60\" target=\"_blank\" rel=\"external noopener noreferrer\" referrerpolicy=\"no-referrer\">https://example.invalid/\u0060[[A]]\u0060<span class=\"y-offscreen\"> (opens in a new tab)</span></a> ## &lt;em&gt;A&lt;/em&gt;\n^a\n0<code>open [[A]] close</code>ref[^n]</p>\n<blockquote>\n<p>[!note] title\n[[#A]][[A]]\n--&gt;[[A#A]] ^é</p>\n</blockquote>\n[[A]]\n--&gt;&gt; [!note] [[A]]\n<h3 id=\"a\" data-level=\"2\">A</h3>\n<p>[[A|alias]]É</p>\n<div class=\"y-tablewrap\"><table>\n<thead>\n<tr>\n<th>a</th>\n<th>b</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td>[[A]]</td>\n<td>^a</td>\n</tr>\n</tbody>\n</table></div>\n[[A]]\n--&gt;",
  "Names": [
    "^a",
    "^a-2",
    "^agreement-absent-0",
    "^é"
  ],
  "Native": {
    "Plain": {
      "Bases": [
        "a"
      ],
      "Checks": [
        "a"
      ],
      "Headings": [
        {
          "Start": 207,
          "Stop": 208
        }
      ],
      "Code": null,
      "Fields": null,
      "Eligible": true
    },
    "Page": {
      "Bases": [
        "a"
      ],
      "Checks": [
        "a"
      ],
      "Headings": [
        {
          "Start": 207,
          "Stop": 208
        }
      ],
      "Code": null,
      "Fields": null,
      "Eligible": true
    },
    "Native": {
      "Definitions": null,
      "Code": [
        {
          "Kind": "CodeSpan",
          "Parent": "Paragraph",
          "Span": {
            "Start": 102,
            "Stop": 118
          }
        }
      ],
      "Comments": [
        {
          "Start": 149,
          "Stop": 163
        },
        {
          "Start": 177,
          "Stop": 191
        },
        {
          "Start": 263,
          "Stop": 277
        }
      ]
    },
    "Check": {
      "Code": [
        {
          "Start": 69,
          "Stop": 74
        },
        {
          "Start": 102,
          "Stop": 118
        }
      ],
      "Comments": [
        {
          "Start": 149,
          "Stop": 163
        },
        {
          "Start": 177,
          "Stop": 191
        },
        {
          "Start": 263,
          "Stop": 277
        }
      ],
      "Fields": [
        {
          "Span": {
            "Start": 18,
            "Stop": 24
          },
          "Word": "[[A\\]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 69,
            "Stop": 74
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": true,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 107,
            "Stop": 112
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": true,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": 0
        },
        {
          "Span": {
            "Start": 143,
            "Stop": 149
          },
          "Word": "[[#A]]",
          "Targets": [],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 154,
            "Stop": 159
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": true,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 163,
            "Stop": 170
          },
          "Word": "[[A#A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 182,
            "Stop": 187
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": true,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 201,
            "Stop": 206
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 213,
            "Stop": 225
          },
          "Word": "[[A\\|alias]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 250,
            "Stop": 255
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 268,
            "Stop": 273
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": true,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        }
      ],
      "Targets": [
        "A",
        "A",
        "A",
        "A",
        "A"
      ]
    },
    "Surface": "- > [!note] title\n[[A\\]]## <em>A</em>\n  \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 ## <em>A</em>\n ^a\n0\u0060open\n[[A]]\nclose\u0060ref[^n]\n> [!note] title\n[[#A]]\n\n[[A#A]] ^é\n\n\n\n\n> [!note] [[A]]\nA\n---\n[[A\\|alias]]É\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\n\n",
    "Parsed": null,
    "Generous": [
      {
        "Span": {
          "Start": 167,
          "Stop": 188
        },
        "Raw": "> [!note] [[A]]\nA\n---",
        "Words": "[!note] [[A]]\nA",
        "Name": "note-a-a",
        "Level": 2
      }
    ],
    "Names": {
      "note-a-a": true
    },
    "Allocated": [
      "a"
    ],
    "Selected": {
      "a": true
    }
  },
  "Page": {
    "Citations": [
      {
        "SourceRole": "",
        "Target": "A\\",
        "Section": "",
        "State": "wikilink-broken"
      }
    ],
    "CitationsInCode": 0,
    "Blocks": null,
    "Headings": [
      "a"
    ],
    "Failures": null,
    "CodeCitations": null,
    "CalloutTitles": null
  },
  "Private": "- > [!note] title\n[[A\\]]## <em>A</em>\n  \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 ## <em>A</em>\n ^a\n0\u0060open\n[[A]]\nclose\u0060ref[^n]\n> [!note] title\n[[#A]]<!--\n[[A]]\n-->[[A#A]] ^é\n\n\n<!--\n[[A]]\n-->> [!note] [[A]]\nq\n---\n[[A\\|alias]]É\n| a | b |\n|---|---|\n| [[A]] | ^a |\n<!--\n[[A]]\n-->",
  "PrivateAddress": {
    "Native": {
      "Definitions": null,
      "Code": [
        {
          "Kind": "CodeSpan",
          "Parent": "Paragraph",
          "Span": {
            "Start": 102,
            "Stop": 118
          }
        }
      ],
      "Comments": [
        {
          "Start": 149,
          "Stop": 163
        },
        {
          "Start": 177,
          "Stop": 191
        },
        {
          "Start": 263,
          "Stop": 277
        }
      ]
    },
    "Check": {
      "Code": [
        {
          "Start": 69,
          "Stop": 74
        },
        {
          "Start": 102,
          "Stop": 118
        }
      ],
      "Comments": [
        {
          "Start": 149,
          "Stop": 163
        },
        {
          "Start": 177,
          "Stop": 191
        },
        {
          "Start": 263,
          "Stop": 277
        }
      ],
      "Fields": [
        {
          "Span": {
            "Start": 18,
            "Stop": 24
          },
          "Word": "[[A\\]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 69,
            "Stop": 74
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": true,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 107,
            "Stop": 112
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": true,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": 0
        },
        {
          "Span": {
            "Start": 143,
            "Stop": 149
          },
          "Word": "[[#A]]",
          "Targets": [],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 154,
            "Stop": 159
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": true,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 163,
            "Stop": 170
          },
          "Word": "[[A#A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 182,
            "Stop": 187
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": true,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 201,
            "Stop": 206
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 213,
            "Stop": 225
          },
          "Word": "[[A\\|alias]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 250,
            "Stop": 255
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 268,
            "Stop": 273
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": true,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        }
      ],
      "Targets": [
        "A",
        "A",
        "A",
        "A",
        "A"
      ]
    },
    "Fields": [
      {
        "Line": {
          "Start": 90,
          "Stop": 93
        },
        "Token": {
          "Start": 91,
          "Stop": 93
        },
        "Raw": " ^a",
        "Name": "^a",
        "Folded": "^a",
        "Owner": -1,
        "Nodes": null,
        "Code": false,
        "Comment": false
      }
    ],
    "Names": [
      "^a",
      "^a-2",
      "^é"
    ]
  },
  "PrivateAddresses": {
    "^a": {
      "Page": false,
      "Judge": false,
      "Found": false,
      "Cut": ""
    },
    "^a-2": {
      "Page": false,
      "Judge": false,
      "Found": false,
      "Cut": ""
    },
    "^agreement-absent-0": {
      "Page": false,
      "Judge": false,
      "Found": false,
      "Cut": ""
    },
    "^é": {
      "Page": false,
      "Judge": false,
      "Found": false,
      "Cut": ""
    }
  },
  "PrivateDiagnostics": [
    {
      "Kind": "wikilink-broken",
      "Target": "A\\",
      "Message": "wikilink \"A\\\\\" does not resolve to any note or file",
      "Section": "",
      "Block": ""
    },
    {
      "Kind": "risky-fence",
      "Target": "",
      "Message": "wikilink/callout/table syntax found inside a fenced code block; left untouched",
      "Section": "",
      "Block": ""
    }
  ],
  "PrivateHTML": "<ul>\n<li>\n<blockquote>\n<p>[!note] title\n<span class=\"wikilink-broken\" title=\"There is no note called &#34;A\\\\&#34; yet\">A\\<span class=\"y-offscreen\"> (There is no note called &#34;A\\\\&#34; yet)</span></span>## &lt;em&gt;A&lt;/em&gt;</p>\n</blockquote>\n<pre class=\"chroma\"><code></code></pre></li>\n</ul>\n<p><a href=\"https://example.invalid/%60%5B%5BA%5D%5D%60\" target=\"_blank\" rel=\"external noopener noreferrer\" referrerpolicy=\"no-referrer\">https://example.invalid/\u0060[[A]]\u0060<span class=\"y-offscreen\"> (opens in a new tab)</span></a> ## &lt;em&gt;A&lt;/em&gt;\n^a\n0<code>open [[A]] close</code>ref[^n]</p>\n<blockquote>\n<p>[!note] title\n[[#A]][[A]]\n--&gt;[[A#A]] ^é</p>\n</blockquote>\n[[A]]\n--&gt;&gt; [!note] [[A]]\n<h3 id=\"q\" data-level=\"2\">q</h3>\n<p>[[A|alias]]É</p>\n<div class=\"y-tablewrap\"><table>\n<thead>\n<tr>\n<th>a</th>\n<th>b</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td>[[A]]</td>\n<td>^a</td>\n</tr>\n</tbody>\n</table></div>\n[[A]]\n--&gt;",
  "PrivateNative": {
    "Plain": {
      "Bases": [
        "q"
      ],
      "Checks": [
        "q"
      ],
      "Headings": [
        {
          "Start": 207,
          "Stop": 208
        }
      ],
      "Code": null,
      "Fields": null,
      "Eligible": true
    },
    "Page": {
      "Bases": [
        "q"
      ],
      "Checks": [
        "q"
      ],
      "Headings": [
        {
          "Start": 207,
          "Stop": 208
        }
      ],
      "Code": null,
      "Fields": null,
      "Eligible": true
    },
    "Native": {
      "Definitions": null,
      "Code": [
        {
          "Kind": "CodeSpan",
          "Parent": "Paragraph",
          "Span": {
            "Start": 102,
            "Stop": 118
          }
        }
      ],
      "Comments": [
        {
          "Start": 149,
          "Stop": 163
        },
        {
          "Start": 177,
          "Stop": 191
        },
        {
          "Start": 263,
          "Stop": 277
        }
      ]
    },
    "Check": {
      "Code": [
        {
          "Start": 69,
          "Stop": 74
        },
        {
          "Start": 102,
          "Stop": 118
        }
      ],
      "Comments": [
        {
          "Start": 149,
          "Stop": 163
        },
        {
          "Start": 177,
          "Stop": 191
        },
        {
          "Start": 263,
          "Stop": 277
        }
      ],
      "Fields": [
        {
          "Span": {
            "Start": 18,
            "Stop": 24
          },
          "Word": "[[A\\]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 69,
            "Stop": 74
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": true,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 107,
            "Stop": 112
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": true,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": 0
        },
        {
          "Span": {
            "Start": 143,
            "Stop": 149
          },
          "Word": "[[#A]]",
          "Targets": [],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 154,
            "Stop": 159
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": true,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 163,
            "Stop": 170
          },
          "Word": "[[A#A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 182,
            "Stop": 187
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": true,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 201,
            "Stop": 206
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 213,
            "Stop": 225
          },
          "Word": "[[A\\|alias]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 250,
            "Stop": 255
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": false,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        },
        {
          "Span": {
            "Start": 268,
            "Stop": 273
          },
          "Word": "[[A]]",
          "Targets": [
            "A"
          ],
          "Code": false,
          "Comment": true,
          "Escaped": false,
          "DefinitionOwner": -1,
          "CodeOwner": -1
        }
      ],
      "Targets": [
        "A",
        "A",
        "A",
        "A",
        "A"
      ]
    },
    "Surface": "- > [!note] title\n[[A\\]]## <em>A</em>\n  \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 ## <em>A</em>\n ^a\n0\u0060open\n[[A]]\nclose\u0060ref[^n]\n> [!note] title\n[[#A]]\n\n[[A#A]] ^é\n\n\n\n\n> [!note] [[A]]\nq\n---\n[[A\\|alias]]É\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\n\n",
    "Parsed": null,
    "Generous": [
      {
        "Span": {
          "Start": 167,
          "Stop": 188
        },
        "Raw": "> [!note] [[A]]\nq\n---",
        "Words": "[!note] [[A]]\nq",
        "Name": "note-a-q",
        "Level": 2
      }
    ],
    "Names": {
      "note-a-q": true
    },
    "Allocated": [
      "q"
    ],
    "Selected": {
      "q": true
    }
  },
  "PrivatePage": {
    "Citations": [
      {
        "SourceRole": "",
        "Target": "A\\",
        "Section": "",
        "State": "wikilink-broken"
      }
    ],
    "CitationsInCode": 0,
    "Blocks": null,
    "Headings": [
      "q"
    ],
    "Failures": null,
    "CodeCitations": null,
    "CalloutTitles": null
  },
  "PrivateTree": {
    "Trees": [
      [
        {
          "Kind": "Document",
          "Depth": 0,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "List",
          "Depth": 1,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "ListItem",
          "Depth": 2,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Blockquote",
          "Depth": 3,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Paragraph",
          "Depth": 4,
          "Lines": [
            {
              "Start": 4,
              "Stop": 18
            },
            {
              "Start": 18,
              "Stop": 37
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 4,
              "Stop": 5
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 5,
              "Stop": 11
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 11,
              "Stop": 17
            }
          ],
          "Raw": false,
          "Soft": true,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 18,
              "Stop": 19
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 19,
              "Stop": 20
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 20,
              "Stop": 27
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "RawHTML",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 27,
              "Stop": 31
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 31,
              "Stop": 32
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "RawHTML",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 32,
              "Stop": 37
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "FencedCodeBlock",
          "Depth": 3,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Paragraph",
          "Depth": 1,
          "Lines": [
            {
              "Start": 44,
              "Stop": 90
            },
            {
              "Start": 91,
              "Stop": 94
            },
            {
              "Start": 94,
              "Stop": 107
            },
            {
              "Start": 107,
              "Stop": 113
            },
            {
              "Start": 113,
              "Stop": 126
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "AutoLink",
          "Depth": 2,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 75,
              "Stop": 79
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "RawHTML",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 79,
              "Stop": 83
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 83,
              "Stop": 84
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "RawHTML",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 84,
              "Stop": 89
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 89,
              "Stop": 89
            }
          ],
          "Raw": false,
          "Soft": true,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 91,
              "Stop": 93
            }
          ],
          "Raw": false,
          "Soft": true,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 94,
              "Stop": 101
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "CodeSpan",
          "Depth": 2,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 102,
              "Stop": 107
            }
          ],
          "Raw": true,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 107,
              "Stop": 113
            }
          ],
          "Raw": true,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 113,
              "Stop": 118
            }
          ],
          "Raw": true,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 119,
              "Stop": 123
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 123,
              "Stop": 125
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 125,
              "Stop": 126
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Blockquote",
          "Depth": 1,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Paragraph",
          "Depth": 2,
          "Lines": [
            {
              "Start": 129,
              "Stop": 143
            },
            {
              "Start": 143,
              "Stop": 154
            },
            {
              "Start": 154,
              "Stop": 160
            },
            {
              "Start": 160,
              "Stop": 174
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 129,
              "Stop": 130
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 130,
              "Stop": 136
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 136,
              "Stop": 142
            }
          ],
          "Raw": false,
          "Soft": true,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 143,
              "Stop": 144
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 144,
              "Stop": 145
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 145,
              "Stop": 149
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "RawHTML",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 149,
              "Stop": 154
            },
            {
              "Start": 154,
              "Stop": 160
            },
            {
              "Start": 160,
              "Stop": 163
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 163,
              "Stop": 164
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 164,
              "Stop": 165
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 165,
              "Stop": 170
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 170,
              "Stop": 174
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "HTMLBlock",
          "Depth": 1,
          "Lines": [
            {
              "Start": 177,
              "Stop": 182
            },
            {
              "Start": 182,
              "Stop": 188
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Heading",
          "Depth": 1,
          "Lines": [
            {
              "Start": 207,
              "Stop": 208
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 207,
              "Stop": 208
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Paragraph",
          "Depth": 1,
          "Lines": [
            {
              "Start": 213,
              "Stop": 227
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 213,
              "Stop": 214
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 214,
              "Stop": 215
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 215,
              "Stop": 224
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 224,
              "Stop": 227
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Table",
          "Depth": 1,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "TableHeader",
          "Depth": 2,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "TableCell",
          "Depth": 3,
          "Lines": [
            {
              "Start": 230,
              "Stop": 231
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 4,
          "Lines": null,
          "Text": [
            {
              "Start": 230,
              "Stop": 231
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "TableCell",
          "Depth": 3,
          "Lines": [
            {
              "Start": 234,
              "Stop": 235
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 4,
          "Lines": null,
          "Text": [
            {
              "Start": 234,
              "Stop": 235
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "TableRow",
          "Depth": 2,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "TableCell",
          "Depth": 3,
          "Lines": [
            {
              "Start": 250,
              "Stop": 255
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 4,
          "Lines": null,
          "Text": [
            {
              "Start": 250,
              "Stop": 251
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 4,
          "Lines": null,
          "Text": [
            {
              "Start": 251,
              "Stop": 252
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 4,
          "Lines": null,
          "Text": [
            {
              "Start": 252,
              "Stop": 254
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 4,
          "Lines": null,
          "Text": [
            {
              "Start": 254,
              "Stop": 255
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "TableCell",
          "Depth": 3,
          "Lines": [
            {
              "Start": 258,
              "Stop": 260
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 4,
          "Lines": null,
          "Text": [
            {
              "Start": 258,
              "Stop": 260
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "HTMLBlock",
          "Depth": 1,
          "Lines": [
            {
              "Start": 263,
              "Stop": 268
            },
            {
              "Start": 268,
              "Stop": 274
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        }
      ]
    ],
    "Definitions": null,
    "Code": [
      {
        "Kind": "CodeSpan",
        "Parent": "Paragraph",
        "Span": {
          "Start": 102,
          "Stop": 118
        }
      }
    ],
    "Comments": [
      {
        "Start": 149,
        "Stop": 163
      },
      {
        "Start": 177,
        "Stop": 191
      },
      {
        "Start": 263,
        "Stop": 277
      }
    ],
    "Unanchorable": null,
    "Valid": true
  },
  "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
  "Tree": {
    "Trees": [
      [
        {
          "Kind": "Document",
          "Depth": 0,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "List",
          "Depth": 1,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "ListItem",
          "Depth": 2,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Blockquote",
          "Depth": 3,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Paragraph",
          "Depth": 4,
          "Lines": [
            {
              "Start": 4,
              "Stop": 18
            },
            {
              "Start": 18,
              "Stop": 37
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 4,
              "Stop": 5
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 5,
              "Stop": 11
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 11,
              "Stop": 17
            }
          ],
          "Raw": false,
          "Soft": true,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 18,
              "Stop": 19
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 19,
              "Stop": 20
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 20,
              "Stop": 27
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "RawHTML",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 27,
              "Stop": 31
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 31,
              "Stop": 32
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "RawHTML",
          "Depth": 5,
          "Lines": null,
          "Text": [
            {
              "Start": 32,
              "Stop": 37
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "FencedCodeBlock",
          "Depth": 3,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Paragraph",
          "Depth": 1,
          "Lines": [
            {
              "Start": 44,
              "Stop": 90
            },
            {
              "Start": 91,
              "Stop": 94
            },
            {
              "Start": 94,
              "Stop": 107
            },
            {
              "Start": 107,
              "Stop": 113
            },
            {
              "Start": 113,
              "Stop": 126
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "AutoLink",
          "Depth": 2,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 75,
              "Stop": 79
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "RawHTML",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 79,
              "Stop": 83
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 83,
              "Stop": 84
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "RawHTML",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 84,
              "Stop": 89
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 89,
              "Stop": 89
            }
          ],
          "Raw": false,
          "Soft": true,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 91,
              "Stop": 93
            }
          ],
          "Raw": false,
          "Soft": true,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 94,
              "Stop": 101
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "CodeSpan",
          "Depth": 2,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 102,
              "Stop": 107
            }
          ],
          "Raw": true,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 107,
              "Stop": 113
            }
          ],
          "Raw": true,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 113,
              "Stop": 118
            }
          ],
          "Raw": true,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 119,
              "Stop": 123
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 123,
              "Stop": 125
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 125,
              "Stop": 126
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Blockquote",
          "Depth": 1,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Paragraph",
          "Depth": 2,
          "Lines": [
            {
              "Start": 129,
              "Stop": 143
            },
            {
              "Start": 143,
              "Stop": 154
            },
            {
              "Start": 154,
              "Stop": 160
            },
            {
              "Start": 160,
              "Stop": 174
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 129,
              "Stop": 130
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 130,
              "Stop": 136
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 136,
              "Stop": 142
            }
          ],
          "Raw": false,
          "Soft": true,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 143,
              "Stop": 144
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 144,
              "Stop": 145
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 145,
              "Stop": 149
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "RawHTML",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 149,
              "Stop": 154
            },
            {
              "Start": 154,
              "Stop": 160
            },
            {
              "Start": 160,
              "Stop": 163
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 163,
              "Stop": 164
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 164,
              "Stop": 165
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 165,
              "Stop": 170
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 3,
          "Lines": null,
          "Text": [
            {
              "Start": 170,
              "Stop": 174
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "HTMLBlock",
          "Depth": 1,
          "Lines": [
            {
              "Start": 177,
              "Stop": 182
            },
            {
              "Start": 182,
              "Stop": 188
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Heading",
          "Depth": 1,
          "Lines": [
            {
              "Start": 207,
              "Stop": 208
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 207,
              "Stop": 208
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Paragraph",
          "Depth": 1,
          "Lines": [
            {
              "Start": 213,
              "Stop": 227
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 213,
              "Stop": 214
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 214,
              "Stop": 215
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 215,
              "Stop": 224
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 2,
          "Lines": null,
          "Text": [
            {
              "Start": 224,
              "Stop": 227
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Table",
          "Depth": 1,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "TableHeader",
          "Depth": 2,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "TableCell",
          "Depth": 3,
          "Lines": [
            {
              "Start": 230,
              "Stop": 231
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 4,
          "Lines": null,
          "Text": [
            {
              "Start": 230,
              "Stop": 231
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "TableCell",
          "Depth": 3,
          "Lines": [
            {
              "Start": 234,
              "Stop": 235
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 4,
          "Lines": null,
          "Text": [
            {
              "Start": 234,
              "Stop": 235
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "TableRow",
          "Depth": 2,
          "Lines": null,
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "TableCell",
          "Depth": 3,
          "Lines": [
            {
              "Start": 250,
              "Stop": 255
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 4,
          "Lines": null,
          "Text": [
            {
              "Start": 250,
              "Stop": 251
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 4,
          "Lines": null,
          "Text": [
            {
              "Start": 251,
              "Stop": 252
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 4,
          "Lines": null,
          "Text": [
            {
              "Start": 252,
              "Stop": 254
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 4,
          "Lines": null,
          "Text": [
            {
              "Start": 254,
              "Stop": 255
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "TableCell",
          "Depth": 3,
          "Lines": [
            {
              "Start": 258,
              "Stop": 260
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "Text",
          "Depth": 4,
          "Lines": null,
          "Text": [
            {
              "Start": 258,
              "Stop": 260
            }
          ],
          "Raw": false,
          "Soft": false,
          "Hard": false
        },
        {
          "Kind": "HTMLBlock",
          "Depth": 1,
          "Lines": [
            {
              "Start": 263,
              "Stop": 268
            },
            {
              "Start": 268,
              "Stop": 274
            }
          ],
          "Text": null,
          "Raw": false,
          "Soft": false,
          "Hard": false
        }
      ]
    ],
    "Definitions": null,
    "Code": [
      {
        "Kind": "CodeSpan",
        "Parent": "Paragraph",
        "Span": {
          "Start": 102,
          "Stop": 118
        }
      }
    ],
    "Comments": [
      {
        "Start": 149,
        "Stop": 163
      },
      {
        "Start": 177,
        "Stop": 191
      },
      {
        "Start": 263,
        "Stop": 277
      }
    ],
    "Unanchorable": null,
    "Valid": true
  }
}`
	var r agreementHeadingSingleDeclaration
	if json.Unmarshal([]byte(frozen), &r) != nil {
		return nil
	}
	return &r
})

func agreementHeadingSingleEligible(body string, f *agreementFailure, page *agreementHTML) bool {
	if f == nil || page == nil || f.Property != "P4" || f.Identity != "literal-heading-id" || f.Direction != "page-only" || f.Multiplicity != 1 || f.Tuple != (agreementCitation{}) || !f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Cut != "" {
		return false
	}
	native := agreementNativeHeadingReading(body)
	return native.Plain.Eligible && native.Page.Eligible && cmp.Equal(native.Plain, native.Page) && cmp.Equal(native.Allocated, page.Headings) && native.Selected[f.Fragment]
}
func agreementHeadingSingleProjection(r *agreementHeadingSingleDeclaration, before, after []agreementFailure) bool {
	if r == nil || !maps.Equal(agreementMixedCurrent(before), agreementMixedCurrent(r.Before)) || !maps.Equal(agreementMixedCurrent(after), agreementMixedCurrent(r.After)) {
		return false
	}
	want := maps.Clone(agreementMixedCurrent(before))
	count := 0
	for fi := range before {
		f := before[fi]
		if agreementSignature(&f) != r.Selected {
			continue
		}
		count++
		delete(want, r.Selected)
		f.Fragment = "q"
		want[agreementSignature(&f)]++
	}
	return count == 1 && maps.Equal(want, agreementMixedCurrent(after))
}
func agreementHeadingSingleDifference(c agreementCase, f *agreementFailure, raw string, page *agreementHTML, diagnostics []render.Diagnostic, failures []agreementFailure) (kind, authority, wrong string) {
	r := agreementHeadingSingleDeclarationReading()
	if r == nil || c.Body != r.Body || c.Title != "" || len(c.Companions) != 0 || !agreementHeadingSingleEligible(c.Body, f, page) || agreementSignature(f) != r.Selected {
		return "", "", ""
	}
	if raw != r.HTML || !cmp.Equal(*page, r.Page) || !cmp.Equal(diagnostics, r.Diagnostics) || !cmp.Equal(agreementNativeHeadingReading(c.Body), r.Native) || !cmp.Equal(agreementNativeProseAddressTreeReading(c.Body, nil), r.Tree) || !maps.Equal(agreementMixedCurrent(failures), agreementMixedCurrent(r.Before)) {
		return "", "", ""
	}
	return "debt", "#1011 stage 8", "judge"
}

func TestAgreementHeadingSingleOwnedSource(t *testing.T) {
	r := agreementHeadingSingleDeclarationReading()
	if r == nil {
		t.Fatal("caught: complete single heading declaration")
	}
	if len(r.Native.Page.Headings) != 1 || !cmp.Equal(r.Native.Plain, r.Native.Page) || len(r.Native.Allocated) != 1 || r.Native.Allocated[0] != "a" || r.Native.Names["a"] || !r.Native.Selected["a"] {
		t.Fatal("caught: full original native heading namespace")
	}
	span := r.Native.Page.Headings[0]
	if span != (graph.Span{Start: 207, Stop: 208}) || r.Body[span.Start:span.Stop] != "A" {
		t.Fatal("caught: complete physical heading declaration")
	}
	private := []byte(r.Body)
	copy(private[span.Start:span.Stop], "q")
	if string(private) != r.Private {
		t.Fatal("caught: exact single heading private source projection")
	}
	if !cmp.Equal(agreementNativeHeadingReading(r.Body), r.Native) || !cmp.Equal(agreementNativeHeadingReading(r.Private), r.PrivateNative) || !cmp.Equal(agreementNativeProseAddressTreeReading(r.Body, nil), r.Tree) || !cmp.Equal(agreementNativeProseAddressTreeReading(r.Private, nil), r.PrivateTree) || !cmp.Equal(r.Tree, r.PrivateTree) || !cmp.Equal(r.Native.Check, r.PrivateNative.Check) {
		t.Fatal("caught: complete original and private heading reader declarations")
	}
	if len(r.PrivateNative.Allocated) != 1 || r.PrivateNative.Allocated[0] != "q" || r.PrivateNative.Names["q"] || !r.PrivateNative.Selected["q"] || !cmp.Equal(r.Native.Page.Headings, r.PrivateNative.Page.Headings) {
		t.Fatal("caught: full private native heading namespace and unchanged source coordinates")
	}
}
func TestAgreementHeadingSingleOwnedPublic(t *testing.T) {
	r := agreementHeadingSingleDeclarationReading()
	if r == nil {
		t.Fatal("caught: complete single heading declaration")
	}
	raw, page, before := agreementProseP0Failures(t, r.Body)
	pr, pp, after := agreementProseP0Failures(t, r.Private)
	if raw.HTML != r.HTML || pr.HTML != r.PrivateHTML || !cmp.Equal(page, r.Page) || !cmp.Equal(pp, r.PrivatePage) || !cmp.Equal(raw.Diagnostics, r.Diagnostics) || !cmp.Equal(pr.Diagnostics, r.PrivateDiagnostics) || !cmp.Equal(raw.Diagnostics, pr.Diagnostics) || !agreementHeadingSingleProjection(r, before, after) {
		t.Fatal("caught: complete original and private heading public signed ledgers")
	}
	const tag = `<h3 id="a" data-level="2">A</h3>`
	if strings.Count(raw.HTML, tag) != 1 || strings.Replace(raw.HTML, tag, `<h3 id="q" data-level="2">q</h3>`, 1) != pr.HTML {
		t.Fatal("caught: exact heading-only public byte projection")
	}
	expected := page
	expected.Headings = []string{"q"}
	if !cmp.Equal(expected, pp) {
		t.Fatal("caught: whole private heading public ledger")
	}
	seen := 0
	for fi := range before {
		f := before[fi]
		if agreementSignature(&f) == r.Selected {
			seen++
			k, a, w := agreementHeadingSingleDifference(agreementCase{Body: r.Body}, &f, raw.HTML, &page, raw.Diagnostics, before)
			if k != "debt" || a != "#1011 stage 8" || w != "judge" {
				t.Fatal("caught: exact source-bound heading authority")
			}
		}
	}
	if seen != 1 {
		t.Fatal("caught: unique original heading signature")
	}
}
func TestAgreementHeadingSingleOwnedRefusals(t *testing.T) {
	r := agreementHeadingSingleDeclarationReading()
	if r == nil {
		t.Fatal("caught: complete single heading declaration")
	}
	raw, page, fs := agreementProseP0Failures(t, r.Body)
	var selected agreementFailure
	seen := 0
	for _, f := range fs {
		if agreementSignature(&f) == r.Selected {
			selected = f
			seen++
		}
	}
	if seen != 1 {
		t.Fatal("selected heading setup")
	}
	refuse := func(c agreementCase, f *agreementFailure, html string, p *agreementHTML, ds []render.Diagnostic, failures []agreementFailure) {
		t.Helper()
		k, a, w := agreementHeadingSingleDifference(c, f, html, p, ds, failures)
		if k != "" || a != "" || w != "" {
			t.Fatal("caught: exact heading membership and complete context")
		}
	}
	for _, c := range []agreementCase{{Body: r.Body + "!"}, {Body: r.Body, Title: "outside"}, {Body: r.Body, Companions: capturedBodies{"outside.md": "outside"}}} {
		refuse(c, &selected, raw.HTML, &page, raw.Diagnostics, fs)
	}
	refuse(agreementCase{Body: r.Body}, nil, raw.HTML, &page, raw.Diagnostics, fs)
	refuse(agreementCase{Body: r.Body}, &selected, raw.HTML, nil, raw.Diagnostics, fs)
	refuse(agreementCase{Body: r.Body}, &selected, raw.HTML+"!", &page, raw.Diagnostics, fs)
	changes := []func(*agreementFailure){func(f *agreementFailure) { f.Property = "P1" }, func(f *agreementFailure) { f.Identity = "outside" }, func(f *agreementFailure) { f.Direction = "judge-only" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Tuple.Target = "A" }, func(f *agreementFailure) { f.PagePresent = false }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true }, func(f *agreementFailure) { f.Cut = "outside" }, func(f *agreementFailure) { f.Fragment = "q" }}
	for _, change := range changes {
		f := selected
		change(&f)
		refuse(agreementCase{Body: r.Body}, &f, raw.HTML, &page, raw.Diagnostics, fs)
	}
	changedPage := page
	changedPage.Blocks = append(slices.Clone(page.Blocks), "^outside")
	refuse(agreementCase{Body: r.Body}, &selected, raw.HTML, &changedPage, raw.Diagnostics, fs)
	changedDiags := append(slices.Clone(raw.Diagnostics), render.Diagnostic{Kind: render.DiagWikilinkBroken, Target: "outside"})
	refuse(agreementCase{Body: r.Body}, &selected, raw.HTML, &page, changedDiags, fs)
	for i := range fs {
		refuse(agreementCase{Body: r.Body}, &selected, raw.HTML, &page, raw.Diagnostics, slices.Delete(slices.Clone(fs), i, i+1))
	}
	for i := range r.Before {
		if agreementHeadingSingleProjection(r, slices.Delete(slices.Clone(r.Before), i, i+1), r.After) {
			t.Fatal("caught: every original heading signed receipt retained")
		}
	}
	for i := range r.After {
		if agreementHeadingSingleProjection(r, r.Before, slices.Delete(slices.Clone(r.After), i, i+1)) {
			t.Fatal("caught: every private heading signed receipt retained")
		}
	}
}
func TestAgreementHeadingSingleOwnedAddresses(t *testing.T) {
	r := agreementHeadingSingleDeclarationReading()
	if r == nil {
		t.Fatal("caught: complete single heading declaration")
	}
	_, page := agreementIsolatedPage(t, agreementCase{Body: r.Body})
	_, pp := agreementIsolatedPage(t, agreementCase{Body: r.Private})
	a, b := agreementNativeAddressPartReading(r.Body), agreementNativeAddressPartReading(r.Private)
	if !cmp.Equal(a, r.Address) || !cmp.Equal(b, r.PrivateAddress) {
		t.Fatal("caught: whole original and private heading address declarations")
	}
	names := append(append(append(append([]string(nil), a.Names...), b.Names...), page.Blocks...), pp.Blocks...)
	absent := "^agreement-absent-0"
	for strings.Contains(r.Body, absent) || strings.Contains(r.Private, absent) || slices.Contains(names, absent) {
		absent += "x"
	}
	names = append(names, absent)
	slices.Sort(names)
	names = slices.Compact(names)
	if !slices.Equal(names, r.Names) || !cmp.Equal(agreementNativeAddressPublic(t, r.Body, &page, names), r.Addresses) || !cmp.Equal(agreementNativeAddressPublic(t, r.Private, &pp, names), r.PrivateAddresses) {
		t.Fatal("caught: whole heading address namespace and public receipts")
	}
}
func TestAgreementHeadingSingleOwnedCensus(t *testing.T) {
	const frozen = `[
  {
    "Body": "    \u0060\u0060\u0060\n- [x] [[B]]\n> [!note] one\n> [!note] two\n> [!note] three\n章節\n> [!note] title\nA\n===\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "    \u0060\u0060\u0060\n\t  - <!-- [[A]] -->[[A#A]]    \u0060\u0060\u0060\n[^n]: [[A]]\n\n    [[B]]\n[[A#A]]^absent-prose %%[[A]]%%> [!unknown] title\n\u0060\u0060\u0060 [[A]]\n[[A#^a]][[image.png]]\u0060![[image.png]]-->[^n]: [[A]]\n\n    [[B]]\n ^a\nA\n===\n\u0060\u0060\u0060\u0060\n[[B|alias]]ref[^n]\n0    \u0060\u0060\u0060\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\t## [[A|alias]]\nA\n---\n章節\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"aliasref1-0\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n<!-- [[A]] --># A\n## A-2\n## A\n## A\nref[^n]\nÉ\n## A\n## A\n0 \n\n## A\n- > [!note] title\n# A\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]] ^é\n章節\n> [!note] one\n> [!note] two\n> [!note] three\n  > [!note] title\n## A-2\n## A\n## A\n\u0060open\n[[A]]\nclose\u0060章節\n<!--\n[[A]]\n--># A\nB\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-10\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n<!-- [[A]] --># A\n## A-2\n## A\n## A\nref[^n]\nÉ\n## A\n## A\n0 \n\n## A\n- > [!note] title\n# A\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]] ^é\n章節\n> [!note] one\n> [!note] two\n> [!note] three\n  > [!note] title\n## A-2\n## A\n## A\n\u0060open\n[[A]]\nclose\u0060章節\n<!--\n[[A]]\n--># A\nB\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-11\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n<!-- [[A]] --># A\n## A-2\n## A\n## A\nref[^n]\nÉ\n## A\n## A\n0 \n\n## A\n- > [!note] title\n# A\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]] ^é\n章節\n> [!note] one\n> [!note] two\n> [!note] three\n  > [!note] title\n## A-2\n## A\n## A\n\u0060open\n[[A]]\nclose\u0060章節\n<!--\n[[A]]\n--># A\nB\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n<!-- [[A]] --># A\n## A-2\n## A\n## A\nref[^n]\nÉ\n## A\n## A\n0 \n\n## A\n- > [!note] title\n# A\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]] ^é\n章節\n> [!note] one\n> [!note] two\n> [!note] three\n  > [!note] title\n## A-2\n## A\n## A\n\u0060open\n[[A]]\nclose\u0060章節\n<!--\n[[A]]\n--># A\nB\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n<!-- [[A]] --># A\n## A-2\n## A\n## A\nref[^n]\nÉ\n## A\n## A\n0 \n\n## A\n- > [!note] title\n# A\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]] ^é\n章節\n> [!note] one\n> [!note] two\n> [!note] three\n  > [!note] title\n## A-2\n## A\n## A\n\u0060open\n[[A]]\nclose\u0060章節\n<!--\n[[A]]\n--># A\nB\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n<!-- [[A]] --># A\n## A-2\n## A\n## A\nref[^n]\nÉ\n## A\n## A\n0 \n\n## A\n- > [!note] title\n# A\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]] ^é\n章節\n> [!note] one\n> [!note] two\n> [!note] three\n  > [!note] title\n## A-2\n## A\n## A\n\u0060open\n[[A]]\nclose\u0060章節\n<!--\n[[A]]\n--># A\nB\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n<!-- [[A]] --># A\n## A-2\n## A\n## A\nref[^n]\nÉ\n## A\n## A\n0 \n\n## A\n- > [!note] title\n# A\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]] ^é\n章節\n> [!note] one\n> [!note] two\n> [!note] three\n  > [!note] title\n## A-2\n## A\n## A\n\u0060open\n[[A]]\nclose\u0060章節\n<!--\n[[A]]\n--># A\nB\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n<!-- [[A]] --># A\n## A-2\n## A\n## A\nref[^n]\nÉ\n## A\n## A\n0 \n\n## A\n- > [!note] title\n# A\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]] ^é\n章節\n> [!note] one\n> [!note] two\n> [!note] three\n  > [!note] title\n## A-2\n## A\n## A\n\u0060open\n[[A]]\nclose\u0060章節\n<!--\n[[A]]\n--># A\nB\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-6\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n<!-- [[A]] --># A\n## A-2\n## A\n## A\nref[^n]\nÉ\n## A\n## A\n0 \n\n## A\n- > [!note] title\n# A\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]] ^é\n章節\n> [!note] one\n> [!note] two\n> [!note] three\n  > [!note] title\n## A-2\n## A\n## A\n\u0060open\n[[A]]\nclose\u0060章節\n<!--\n[[A]]\n--># A\nB\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-7\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n<!-- [[A]] --># A\n## A-2\n## A\n## A\nref[^n]\nÉ\n## A\n## A\n0 \n\n## A\n- > [!note] title\n# A\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]] ^é\n章節\n> [!note] one\n> [!note] two\n> [!note] three\n  > [!note] title\n## A-2\n## A\n## A\n\u0060open\n[[A]]\nclose\u0060章節\n<!--\n[[A]]\n--># A\nB\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-8\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n<!-- [[A]] --># A\n## A-2\n## A\n## A\nref[^n]\nÉ\n## A\n## A\n0 \n\n## A\n- > [!note] title\n# A\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]] ^é\n章節\n> [!note] one\n> [!note] two\n> [!note] three\n  > [!note] title\n## A-2\n## A\n## A\n\u0060open\n[[A]]\nclose\u0060章節\n<!--\n[[A]]\n--># A\nB\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-9\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\\]]<!--\n[[A]]\n-->![[A#A]]\t\u0060\u0060\u0060\n  > [!note] title\n[[A\nB]] ^a\n![[A]]A\n===\n  - ![[image.png]]  -     \u0060\u0060\u0060\nÉ\n \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060   > [!note] title\n[^n]: [[A]]\n\n    [[B]]\n-->\n\n\u0060\u0060[[image.png]]A\n===\n\\[[A]]~~~\n| a | b |\n|---|---|\n| [[A]] | ^a |\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-b-a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] title\n[[A]] [[A]]> [!note] title\n ^é\n ^a-2\n\n| a | b |\n|---|---|\n| [[A]] | ^a |\n![[image.png]]<!--\n[[A]]\n-->\u0060\u0060\u0060 [[A]]\nref[^n]\n\t<!-- ^é\n\u0060\u0060\u0060\n## A\n[[A#^a]]\n\u0060[[A]]\u0060\t\u0060\u0060\u0060\n![[A#A]]-->## [[A|alias]]\n1. [[image.png]][[A\nB]]%%",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^é\n## A\n## A\n[[A]] [[A]][^unused]: [[A]]\n## !\n ^A\n\u0060open\n[[A]]\nclose\u0060## !\n- <!--[^n]: [[A]]\n\n    [[B]]\n\n\n~~~~\n ^é\n  \u0060\u0060\u0060\n# A\n    \u0060\u0060\u0060\n[[A\nB]]![[A#A]]\ntext ^absent-prose > [[A\\|alias]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060open\n[[A]]\nclose\u0060\u0060\u0060\u0060\u0060\n<div>\n[[A]]\n</div>\n- item\n\n      ^absent-prose ^absent-prose     \u0060\u0060\u0060\u0060 go [[A]]\n## A\n- [ ] [[A]]\n## !\n# A\n[[A\\|alias]]É\n[[A\nB]]\u0060## A\n## A\n<!-- [[A]] -->[[#A]]<!-- [[A]] -->A\n===\n<!-- [[A]] -->  - <!--\n[[A]]\n-->![[A#A]][[A#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060open\n[[A]]\nclose\u0060\u0060\u0060\u0060\u0060\n<div>\n[[A]]\n</div>\n- item\n\n      ^absent-prose ^absent-prose     \u0060\u0060\u0060\u0060 go [[A]]\n## A\n- [ ] [[A]]\n## !\n# A\n[[A\\|alias]]É\n[[A\nB]]\u0060## A\n## A\n<!-- [[A]] -->[[#A]]<!-- [[A]] -->A\n===\n<!-- [[A]] -->  - <!--\n[[A]]\n-->![[A#A]][[A#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "-->%%[[A]]%%ref[^n]\nA\n---\n> [!note] [[A]]\n[^n]: [[A]]\n\n    [[B]]\n<!-- [[A]] -->A\n===\n- [ ] [[A]]\n\n\n<!--\n[[A]]\n-->B\n ^A\n\u0060\u0060## [[A|alias]]\n\\[[A]]<div>\n[[A]]\n</div>\n## A\n[[A\\|alias]]  - [[A#A]]~~~~\n> > \t\u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 %%[[A]]%%  - ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"ref1-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- ![[A]]# A\n[[B|alias]]## [[A|alias]]\n\n\nhttps://example.invalid/\u0060[[A]]\u0060 [[#A]]ref[^n]\n\u0060open\n[[A]]\nclose\u0060# A\n[^unused]: [[A]]\n[[B|alias]]\n\n\n\t    \u0060\u0060\u0060\u0060 go [[A]]\n<!-- [[A]] -->## A-2\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "É\n[[A\\|alias]]A\n===\n ^é\nhttps://example.invalid/\u0060[[A]]\u0060 ## !\n ^é\n## <em>A</em>\n![[A#A]] \u0060\u0060\u0060\n<!-- [[A]] -->~~~~\n## A\n## A\n![[A]] ^é\n~~~~\n## [[A|alias]]\n[[A#A]]É\nhttps://example.invalid/\u0060[[A]]\u0060 章節\n\u0060\u0060\u0060\n\u0060\u0060\u0060\u0060 go [[A]]\n## A\n## A\n- [x] [[B]]\n\u0060\u0060\u0060\u0060\n  - A\n---\nA\n## <em>A</em>\n## [[A|alias]]\n[[A\\]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"alias-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "É\n[[A\\|alias]]A\n===\n ^é\nhttps://example.invalid/\u0060[[A]]\u0060 ## !\n ^é\n## <em>A</em>\n![[A#A]] \u0060\u0060\u0060\n<!-- [[A]] -->~~~~\n## A\n## A\n![[A]] ^é\n~~~~\n## [[A|alias]]\n[[A#A]]É\nhttps://example.invalid/\u0060[[A]]\u0060 章節\n\u0060\u0060\u0060\n\u0060\u0060\u0060\u0060 go [[A]]\n## A\n## A\n- [x] [[B]]\n\u0060\u0060\u0060\u0060\n  - A\n---\nA\n## <em>A</em>\n## [[A|alias]]\n[[A\\]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"em-a-em-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]]0![[image.png]]0\t\u0060\u0060\u0060\nA\n===\n> [!note] title\n> [!note] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a0-image-png-0-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- 章節\n- [ ] [[A]]\n## A-2\n## A\n## A\n[[A#^a]]\u0060open\n[[A]]\nclose\u0060É\n\n\n![[A]]~~~~\n[[A\\]][[A]]\u0060[[A\nB]][[B|alias]]~~~\n\t\u0060\u0060\u0060\n%%   \u0060\u0060\u0060\n## <em>A</em>\n[^unused]: [[A]]\n[[A#A]][[A\\|alias]]> [!unknown] title\n[[image.png]][[A]]\u0060open\n[[A]]\nclose\u0060B\n\n\n1. ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n ^A\n# A\n ^é\nÉ\n  > [!note] title\n<div>\n[[A]]\n</div>\n      \u0060\u0060\u0060\n<!--[[image.png]]- [ ] [[A]]\n[[A\\]]![[A]]0- item\n\n      \\[[A]]\u0060open\n[[A]]\nclose\u0060## A\n## A\n   \u0060\u0060\u0060\n~~~~\n\u0060\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n ^A\n# A\n ^é\nÉ\n  > [!note] title\n<div>\n[[A]]\n</div>\n      \u0060\u0060\u0060\n<!--[[image.png]]- [ ] [[A]]\n[[A\\]]![[A]]0- item\n\n      \\[[A]]\u0060open\n[[A]]\nclose\u0060## A\n## A\n   \u0060\u0060\u0060\n~~~~\n\u0060\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n[[A#^a]]É\n<div>\n[[A]]\n</div>\n> [!note] title\n0[[image.png]]## A-2\n## A\n## A\n\u00600> \u0060\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n[[A#^a]]É\n<div>\n[[A]]\n</div>\n> [!note] title\n0[[image.png]]## A-2\n## A\n## A\n\u00600> \u0060\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%[[A]]%%## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n    \u0060\u0060\u0060\n\u00600<!-- ^é\nref[^n]\n## <em>A</em>\n\u0060\u0060\u0060\u0060 go [[A]]\n[[A]]- item\n\n      text > [!note] title\n> A\n===\n[^n]: [[A]]\n\n    [[B]]\n> > [[A\\|alias]]~~~\n     \u0060\u0060\u0060\n- [x] [[B]]\n## A\n## A\n[[A]] [[A]]\\\u0060 ^A\n%%[[A]]%%\\\u0060 ^A\n~~~~\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "章節\n\t\u0060\u0060\u0060\nB\n%%text  ref[^n]\n[[#A]] ^A\n\n\n\t\u0060\u0060\u0060\n  - <!--\n[[A]]\n-->B\n%%text \n[[A\\]]- > [!note] title\n  > [!note] title\n0<!-- [[A]] --> ^é\n[[A#A]]\u0060\u0060\u0060\nA\n===\n\u0060open\n[[A]]\nclose\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0-é-a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n> [!note] one\n> [!note] two\n> [!note] three\n1. [[A\\]]B\n\u0060open\n[[A]]\nclose\u0060[[A]] [[A]]- > [!note] title\n## A\n- item\n\n      - [ ] [[A]]\n\t> > ![[image.png]]\t\u0060\u0060\u0060\n> [!note] [[A]]\n  - \u0060open\n[[A]]\nclose\u0060- > [!note] title\n[[#A]]É\n  > [!note] title\n\u0060open\n[[A]]\nclose\u0060 ^é\n\n\n<!--\n[[A]]\n-->## A\n- item\n\n      ## [[A|alias]]\n\u0060[[A]]\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n> [!note] one\n> [!note] two\n> [!note] three\n1. [[A\\]]B\n\u0060open\n[[A]]\nclose\u0060[[A]] [[A]]- > [!note] title\n## A\n- item\n\n      - [ ] [[A]]\n\t> > ![[image.png]]\t\u0060\u0060\u0060\n> [!note] [[A]]\n  - \u0060open\n[[A]]\nclose\u0060- > [!note] title\n[[#A]]É\n  > [!note] title\n\u0060open\n[[A]]\nclose\u0060 ^é\n\n\n<!--\n[[A]]\n-->## A\n- item\n\n      ## [[A|alias]]\n\u0060[[A]]\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n> [!note] one\n> [!note] two\n> [!note] three\n1. [[A\\]]B\n\u0060open\n[[A]]\nclose\u0060[[A]] [[A]]- > [!note] title\n## A\n- item\n\n      - [ ] [[A]]\n\t> > ![[image.png]]\t\u0060\u0060\u0060\n> [!note] [[A]]\n  - \u0060open\n[[A]]\nclose\u0060- > [!note] title\n[[#A]]É\n  > [!note] title\n\u0060open\n[[A]]\nclose\u0060 ^é\n\n\n<!--\n[[A]]\n-->## A\n- item\n\n      ## [[A|alias]]\n\u0060[[A]]\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n<!--- 0É\n\n\n\\\\[[A]][[A\\|alias]][[A\\]]É\n## A\n## A\n    \u0060\u0060\u0060\n- [x] [[B]]\n~~~~\n> [!unknown] title\n> <!-- [[A]] -->A\n---\n[^n]: [[A]]\n\n    [[B]]\n\u0060\u0060## A-2\n## A\n## A\n0\t\u0060\u0060\u0060\u0060 go [[A]]\n    \u0060\u0060\u0060\n<!--\n[[A]]\n-->[[A\nB]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A]]\u0060 ^é\n\u0060open\n[[A]]\nclose\u0060É\n ^é\nA\n===\n<!--text B\nÉ\n> [!note] title\n\u0060\u0060\u0060\u0060 go [[A]]\n> > \u0060open\n[[A]]\nclose\u0060\u0060open\n[[A]]\nclose\u0060\u0060\u0060## A\nA\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-éopen-a-close-é-é-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[image.png]]## A\n## A\n<!--\n[[A]]\n-->A\n---\n<!--\n[[A]]\n-->ref[^n]\n- > [!note] title\n## A\n## A\n\\\\[[A]]<div>\n[[A]]\n</div>\n![[A]]~~~\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A#A]]> [!unknown] title\n<!-- [[A]] -->  - A\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[image.png]]## A\n## A\n<!--\n[[A]]\n-->A\n---\n<!--\n[[A]]\n-->ref[^n]\n- > [!note] title\n## A\n## A\n\\\\[[A]]<div>\n[[A]]\n</div>\n![[A]]~~~\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A#A]]> [!unknown] title\n<!-- [[A]] -->  - A\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[image.png]]## A\n## A\n<!--\n[[A]]\n-->A\n---\n<!--\n[[A]]\n-->ref[^n]\n- > [!note] title\n## A\n## A\n\\\\[[A]]<div>\n[[A]]\n</div>\n![[A]]~~~\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A#A]]> [!unknown] title\n<!-- [[A]] -->  - A\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] [[A]]\nA\n===\nhttps://example.invalid/\u0060[[A]]\u0060 [[A\\]]# A\n~~~\n[[A]][[B|alias]]  - ~~~~\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  > [!note] title\nA\n===\n[[A#A]]B\n<!-- [[A]] -->    \u0060\u0060\u0060\n    \u0060\u0060\u0060\n> [!unknown] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060open\n[[A]]\nclose\u0060A\n---\n%%[[A]]%%## A\n## A\n![[A#A]][^n]: [[A]]\n\n    [[B]]\n> [!note] [[A]]\n[[B|alias]]## A-2\n## A\n## A\n\\\\[[A]]- [x] [[B]]\n[^n]: [[A]]\n\n    [[B]]\n \u0060\u0060\u0060\n  - > [!note] [[A]]\n## A\n## A\n[[B|alias]]> [!note] [[A]]\n## A\n## !\ntext - [ ] [[A]]\n<!-- [[A]] -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060open\n[[A]]\nclose\u0060A\n---\n%%[[A]]%%## A\n## A\n![[A#A]][^n]: [[A]]\n\n    [[B]]\n> [!note] [[A]]\n[[B|alias]]## A-2\n## A\n## A\n\\\\[[A]]- [x] [[B]]\n[^n]: [[A]]\n\n    [[B]]\n \u0060\u0060\u0060\n  - > [!note] [[A]]\n## A\n## A\n[[B|alias]]> [!note] [[A]]\n## A\n## !\ntext - [ ] [[A]]\n<!-- [[A]] -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060open\n[[A]]\nclose\u0060A\n---\n%%[[A]]%%## A\n## A\n![[A#A]][^n]: [[A]]\n\n    [[B]]\n> [!note] [[A]]\n[[B|alias]]## A-2\n## A\n## A\n\\\\[[A]]- [x] [[B]]\n[^n]: [[A]]\n\n    [[B]]\n \u0060\u0060\u0060\n  - > [!note] [[A]]\n## A\n## A\n[[B|alias]]> [!note] [[A]]\n## A\n## !\ntext - [ ] [[A]]\n<!-- [[A]] -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060open\n[[A]]\nclose\u0060A\n---\n%%[[A]]%%## A\n## A\n![[A#A]][^n]: [[A]]\n\n    [[B]]\n> [!note] [[A]]\n[[B|alias]]## A-2\n## A\n## A\n\\\\[[A]]- [x] [[B]]\n[^n]: [[A]]\n\n    [[B]]\n \u0060\u0060\u0060\n  - > [!note] [[A]]\n## A\n## A\n[[B|alias]]> [!note] [[A]]\n## A\n## !\ntext - [ ] [[A]]\n<!-- [[A]] -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"open-a-closea\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- [x] [[B]]\nA\n===\n  > [!note] title\n%%[[A]]%%%%[[A]]%%## A-2\n## A\n## A\n ^a\n[[#A]] ^A\n\n\n\t%%[[A]]%%## A\n\u0060[[A]]\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A]]~~~\n ^a-2\n\t[[A]] [[A]]## !\n- > [!note] title\n章節\n[[A#A]]\\\u0060> [!unknown] title\n  - %%[[A]]%% ^a\n> [!note] [[A]]\nA\n===\n[[A\\]] ^a\n> [!note] title\n[^unused]: [[A]]\n![[A#A]]> >  ^a\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n![[image.png]]- [x] [[B]]\n## A\n  > [!note] title\nA\n===\n-  ^é\n> [!unknown] title\n![[A]]  - ## A\n<div>\n[[A]]\n</div>\n> [!unknown] title\n[[A#A]]\\\\[[A]] ^é\n> > - item\n\n      ![[A]]1. # A\n> > -->[[A\\]]> [!note] title\n> [!note] one\n> [!note] two\n> [!note] three\n    ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->- [x] [[B]]\n\t![[A]]\u0060\u0060\u0060\u0060\n\n\n## !\n ^é\n  > [!note] title\n> [!note] [[A]]\n ^é\n[[B|alias]]<div>\n[[A]]\n</div>\n[[A#^a]]É\n[[A\\|alias]]    \n%%[[A]]%%## !\n\u0060 ^a-2\n> [!note] one\n> [!note] two\n> [!note] three\n  > [!note] title\nA\n---\n%%A\n---\nA\n---\n[[A\\]]É\n    ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\t    \\\u0060[[A\\|alias]]    \u0060\u0060\u0060\n[[A\nB]] ^a\n> [!note] [[A]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n \u0060\u0060\u0060\n ^A\n  > [!note] title\nÉ\n   \u0060\u0060\u0060\n    ## [[A|alias]]\n ^a\nB\n![[A#A]][[A#^a]]![[A#A]][[B|alias]]\u0060open\n[[A]]\nclose\u0060![[A#A]]A\n===\n%%[[A]]%%[[A\\]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-b-a-a-a-a-a-a-aliasopen-a-close-a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!unknown] title\n[[A]] [[A]]%%[[A]]%%\t## <em>A</em>\n[[A\\|alias]]A\n---\n[[#A]]1.  \u0060\u0060\u0060\n\\[[A]]ref[^n]\n<!--- [^unused]: [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-em-a-em-aliasa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<div>\n[[A]]\n</div>\n> [!unknown] title\n- > [!note] title\n  \u0060\u0060\u0060\n0- > [!note] title\n ^é\n1. -->https://example.invalid/\u0060[[A]]\u0060 [[A]]  > [!note] title\n ^é\n## [[A|alias]]\n-->  -  ^A\n\\[[A]]B\n \u0060\u0060\u0060\nA\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-alias\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->\n\n## [[A|alias]]\n章節\n  > [!note] title\nA\n---\n# A\n## [[A|alias]]\nA\n===\nref[^n]\n<!-- [[A]] -->## A\nA\n---\nA\n===\n\u0060\n\n\n\n ^A\n%%[[A]]%%> [!unknown] title\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->\n\n## [[A|alias]]\n章節\n  > [!note] title\nA\n---\n# A\n## [[A|alias]]\nA\n===\nref[^n]\n<!-- [[A]] -->## A\nA\n---\nA\n===\n\u0060\n\n\n\n ^A\n%%[[A]]%%> [!unknown] title\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->\n\n## [[A|alias]]\n章節\n  > [!note] title\nA\n---\n# A\n## [[A|alias]]\nA\n===\nref[^n]\n<!-- [[A]] -->## A\nA\n---\nA\n===\n\u0060\n\n\n\n ^A\n%%[[A]]%%> [!unknown] title\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->\n\n## [[A|alias]]\n章節\n  > [!note] title\nA\n---\n# A\n## [[A|alias]]\nA\n===\nref[^n]\n<!-- [[A]] -->## A\nA\n---\nA\n===\n\u0060\n\n\n\n ^A\n%%[[A]]%%> [!unknown] title\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->\n\n## [[A|alias]]\n章節\n  > [!note] title\nA\n---\n# A\n## [[A|alias]]\nA\n===\nref[^n]\n<!-- [[A]] -->## A\nA\n---\nA\n===\n\u0060\n\n\n\n ^A\n%%[[A]]%%> [!unknown] title\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-6\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->\n\n## [[A|alias]]\n章節\n  > [!note] title\nA\n---\n# A\n## [[A|alias]]\nA\n===\nref[^n]\n<!-- [[A]] -->## A\nA\n---\nA\n===\n\u0060\n\n\n\n ^A\n%%[[A]]%%> [!unknown] title\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"alias-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060%%[[A]]%%A\n[[A\\|alias]]%%[[A]]%%> [!note] title\n> [!note] title\n        \u0060\u0060\u0060\n ^A\n\\\\[[A]]A\n---\n[[A]]## A-2\n## A\n## A\n- > [!note] title\n[[image.png]]  > [!note] title\n## [[A|alias]]\n# A\n[^n]: [[A]]\n\n    [[B]]\nref[^n]\n[[B|alias]]https://example.invalid/\u0060[[A]]\u0060 - [ ] [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060%%[[A]]%%A\n[[A\\|alias]]%%[[A]]%%> [!note] title\n> [!note] title\n        \u0060\u0060\u0060\n ^A\n\\\\[[A]]A\n---\n[[A]]## A-2\n## A\n## A\n- > [!note] title\n[[image.png]]  > [!note] title\n## [[A|alias]]\n# A\n[^n]: [[A]]\n\n    [[B]]\nref[^n]\n[[B|alias]]https://example.invalid/\u0060[[A]]\u0060 - [ ] [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060%%[[A]]%%A\n[[A\\|alias]]%%[[A]]%%> [!note] title\n> [!note] title\n        \u0060\u0060\u0060\n ^A\n\\\\[[A]]A\n---\n[[A]]## A-2\n## A\n## A\n- > [!note] title\n[[image.png]]  > [!note] title\n## [[A|alias]]\n# A\n[^n]: [[A]]\n\n    [[B]]\nref[^n]\n[[B|alias]]https://example.invalid/\u0060[[A]]\u0060 - [ ] [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%[[A]]%%    \u0060\u0060\u0060\n> [!note] one\n> [!note] two\n> [!note] three\nA\n---\n> > ## A-2\n## A\n## A\n<!--\n\n\u0060[[A]]\u0060| a | b |\n|---|---|\n| [[A]] | ^a |\n ^é\nA\n===\n## !\n[[B|alias]]\u0060\u0060~~~~\n  - \n0\u0060- item\n\n      [[A#^a]] ^A\n ^A\n    -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%[[A]]%%    \u0060\u0060\u0060\n> [!note] one\n> [!note] two\n> [!note] three\nA\n---\n> > ## A-2\n## A\n## A\n<!--\n\n\u0060[[A]]\u0060| a | b |\n|---|---|\n| [[A]] | ^a |\n ^é\nA\n===\n## !\n[[B|alias]]\u0060\u0060~~~~\n  - \n0\u0060- item\n\n      [[A#^a]] ^A\n ^A\n    -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060[[A]]\u0060[[A]] [[A]]\t\u0060\u0060\u0060\n%%## A\n> [!note] [[A]]\n> [!unknown] title\n\\\\[[A]]   \u0060\u0060\u0060\n~~~~\n    \u0060\u0060\u0060\n ^é\n%%> ## A\n## A\n  - [[A\\|alias]]\u0060 ^é\n## [[A|alias]]\n    ## A-2\n## A\n## A\n章節\n\t\u0060\u0060\u0060\n![[image.png]]ref[^n]\n<!-- [[A]] -->[[A]]\u0060\u0060\u0060 [[A]]\n## <em>A</em>\n0~~~\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060[[A]]\u0060[[A]] [[A]]\t\u0060\u0060\u0060\n%%## A\n> [!note] [[A]]\n> [!unknown] title\n\\\\[[A]]   \u0060\u0060\u0060\n~~~~\n    \u0060\u0060\u0060\n ^é\n%%> ## A\n## A\n  - [[A\\|alias]]\u0060 ^é\n## [[A|alias]]\n    ## A-2\n## A\n## A\n章節\n\t\u0060\u0060\u0060\n![[image.png]]ref[^n]\n<!-- [[A]] -->[[A]]\u0060\u0060\u0060 [[A]]\n## <em>A</em>\n0~~~\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%%% ^é\n1. text [[A#^a]]<div>\n[[A]]\n</div>\n[[A#^a]] ^A\n0\\\u0060\u0060\u0060\u0060\nref[^n]\n[[A#A]]~~~\nA\n![[A]]> [!note] one\n> [!note] two\n> [!note] three\n## A\n## A\n\u0060\u0060[^n]: [[A]]\n\n    [[B]]\n\u0060\u0060\u0060\u0060\n[^unused]: [[A]]\nA\n---\n# A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\t# A\n> [!unknown] title\n## !\nref[^n]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\n[[image.png]]A\n===\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-pnga\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^a-2\n%%[[A]]%%## A\n## A\n0[[#A]][^n]: [[A]]\n\n    [[B]]\n \u0060\u0060\u0060\n> [!note] [[A]]\n## A\n## A\n ^é\nhttps://example.invalid/\u0060[[A]]\u0060 ## A\n ^é\n章節\n[[B|alias]]~~~\nref[^n]\n[[A\nB]]  > [!note] title\n[[A\nB]]A\n---\n    \u0060\u0060\u0060\ntext [[#A]]  \u0060\u0060\u0060\n> [!note] title\n> > \u0060[[A]]\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A]] [[A]]- > [!note] title\n[[image.png]]-->~~~\n%%[[A]]%%  > [!note] title\n-->[[image.png]]A\n===\n\u0060\u0060\u0060 [[A]]\n ^é\n- item\n\n      %%- [x] [[B]]\n\\[[A]]| a | b |\n|---|---|\n| [[A]] | ^a |\n ^a\n> [!note] title\n<!--\n[[A]]\n--> ^a-2\n> [!note] title\n  - 0![[A#A]]- ## <em>A</em>\nA\n===\n[[image.png]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-pnga\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## <em>A</em>\n1. <!--%%[[A]]%%  > [!note] title\n## A\n## A\nÉ\nA\n\u0060\u0060> [!note] title\n  > [!note] title\n## [[A|alias]]\n\u0060A\n===\n![[image.png]]<!-- [[A]] -->\u0060\u0060\u0060\u0060 go [[A]]\n%%[[A]]%% \u0060\u0060\u0060\n[[B|alias]]  - ## <em>A</em>\n# A\nÉ\n ^é\n## A\n0  - A\n===\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## <em>A</em>\n1. <!--%%[[A]]%%  > [!note] title\n## A\n## A\nÉ\nA\n\u0060\u0060> [!note] title\n  > [!note] title\n## [[A|alias]]\n\u0060A\n===\n![[image.png]]<!-- [[A]] -->\u0060\u0060\u0060\u0060 go [[A]]\n%%[[A]]%% \u0060\u0060\u0060\n[[B|alias]]  - ## <em>A</em>\n# A\nÉ\n ^é\n## A\n0  - A\n===\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\n<!--\n[[A]]\n-->## A-2\n## A\n## A\n ^a-2\n- ~~~~\n\nA\n1. [[A\nB]] ^é\n- > [!note] title\n[[A\nB]]<!--\n[[A]]\n--> B\n\t\u0060[[A]]\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\tA\n [^n]: [[A]]\n\n    [[B]]\n%%[[A]]%%## A\n## A\n ^é\n[[A]]ref[^n]\n\u0060\u0060\u0060\n\t ^é\nA\n===\n## [[A|alias]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[^unused]: [[A]]\n ^é\n- item\n\n      [[A#^a]]A\n===\nA\n---\nA\n===\n\u0060\\[[A]] ^a-2\n章節\n[[B|alias]]# A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n ^A\n## [[A|alias]]\n%%[[A]]%%## <em>A</em>\n\\\u0060- [ ] [[A]]\n\\\u0060  - ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] title\n0 - # A\n- [x] [[B]]\n![[A]]# A\n[[A]] [[A]][[A]] [[A]]<!--\n[[A]]\n-->\t\u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060     --> ^é\n章節\n0- > [!note] title\n%%[[A]]%%> [!note] [[A]]\ntext \u0060[[A]]\u0060![[A#A]]> [!note] one\n> [!note] two\n> [!note] three\nA\n---\n- item\n\n      ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060open\n[[A]]\nclose\u00601. A\n---\n\u0060open\n[[A]]\nclose\u0060[[A]] [[A]]A\n---\n\n\n[[A\\|alias]]- [ ] [[A]]\n   \u0060\u0060\u0060\n ^é\n\u0060## A\n## A\n\\\\[[A]][^n]: [[A]]\n\n    [[B]]\n[[A]] [[A]]## [[A|alias]]\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"open-a-close1-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060open\n[[A]]\nclose\u00601. A\n---\n\u0060open\n[[A]]\nclose\u0060[[A]] [[A]]A\n---\n\n\n[[A\\|alias]]- [ ] [[A]]\n   \u0060\u0060\u0060\n ^é\n\u0060## A\n## A\n\\\\[[A]][^n]: [[A]]\n\n    [[B]]\n[[A]] [[A]]## [[A|alias]]\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"open-a-closea-aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%[[A]]%%## A\n## A\n ^é\nA\n---\n ^a-2\n章節\n## A\n[[A\\]]# A\n\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%[[A]]%%## A\n## A\n ^é\nA\n---\n ^a-2\n章節\n## A\n[[A\\]]# A\n\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "01. ## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[image.png]]\u0060\u0060\u0060\u0060\n[[image.png]]## [[A|alias]]\n<!-- [[A]] -->[[image.png]][[A]]\\[[A]][[image.png]]| a | b |\n|---|---|\n| [[A]] | ^a |\n ^A\nÉ\n[[#A]]![[image.png]]A\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-png-image-png-alias-image-pnga-a-image-png-a-b-a-a-a-é-a-image-png-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "B\n\n\n[[A\nB]]A\n## A-2\n## A\n## A\n> text   - \u0060[[A]]%% ^é\n## A\n%%[[A]]%%# A\nA\n---\n> [!unknown] title\n- ## <em>A</em>\n~~~~\n> [!note] [[A]]\n\u0060 \\\\[[A]][[#A]]> [!unknown] title\n\u0060open\n[[A]]\nclose\u0060 ^a-2\n\\\\[[A]]## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "0[[A\\|alias]]   \u0060\u0060\u0060\n<div>\n[[A]]\n</div>\n[[A\\]]0--><!--\n[[A]]\n-->[[A\nB]]## A-2\n## A\n## A\n ^é\n    %%[[A]]%%## A-2\n## A\n## A\n<!--\n[[A]]\n-->É\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "0[[A\\|alias]]   \u0060\u0060\u0060\n<div>\n[[A]]\n</div>\n[[A\\]]0--><!--\n[[A]]\n-->[[A\nB]]## A-2\n## A\n## A\n ^é\n    %%[[A]]%%## A-2\n## A\n## A\n<!--\n[[A]]\n-->É\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "--><!--B\n> [!note] [[A]]\n- item\n\n         \u0060\u0060\u0060\n   \u0060\u0060\u0060\n<!-- [[A]] --> \u0060\u0060\u0060\n<!--> [!note] one\n> [!note] two\n> [!note] three\n\t~~~\ntext [[A\\|alias]][[A\\|alias]][[#A]]> [!note] title\n[[A\nB]] ^A\n   \u0060\u0060\u0060\n[[image.png]]\u0060open\n[[A]]\nclose\u0060 A\n===\n ^é\n## [[A|alias]]\n[[A\\]]\n\n[^unused]: [[A]]\nA\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-pngopen-a-close-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%[[A]]%%| a | b |\n|---|---|\n| [[A]] | ^a |\n## A\n## A\n  > [!note] title\nA\n===\n> [!note] [[A]]\n1. 章節\n[[image.png]]\u0060open\n[[A]]\nclose\u0060## A\n## A\n> [!unknown] title\n\u0060open\n[[A]]\nclose\u0060[[A#^a]]> > [[B|alias]][[A\\]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%[[A]]%%| a | b |\n|---|---|\n| [[A]] | ^a |\n## A\n## A\n  > [!note] title\nA\n===\n> [!note] [[A]]\n1. 章節\n[[image.png]]\u0060open\n[[A]]\nclose\u0060## A\n## A\n> [!unknown] title\n\u0060open\n[[A]]\nclose\u0060[[A#^a]]> > [[B|alias]][[A\\]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%[[A]]%%| a | b |\n|---|---|\n| [[A]] | ^a |\n## A\n## A\n  > [!note] title\nA\n===\n> [!note] [[A]]\n1. 章節\n[[image.png]]\u0060open\n[[A]]\nclose\u0060## A\n## A\n> [!unknown] title\n\u0060open\n[[A]]\nclose\u0060[[A#^a]]> > [[B|alias]][[A\\]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\nA\n---\n ^a\n章節\n\\\u0060 ^a-2\n\u0060open\n[[A]]\nclose\u0060 ^é\n[^n]: [[A]]\n\n    [[B]]\n[[A\\|alias]]\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "0> [!unknown] title\n\\[[A]]1. - [[A]]![[image.png]]\u0060\u0060\u0060 [[A]]\n[[A\\|alias]]> >  ^a\n![[image.png]]É\n- <!--<div>\n[[A]]\n</div>\n%%A\n<!--\n[[A]]\n-->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0-unknown-title-a-1-a-image-png-a-alias-a-image-png-é\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- [x] [[B]]\n## A-2\n## A\n## A\n0[[A\\|alias]] ^a\n ^a-2\n[[#A]]~~~~\n- item\n\n      \u0060\u0060\u0060\u0060\n[[image.png]] ^A\n\\\\[[A]]\u0060\u0060\u0060\n[[A\\]]<!--\n[[A]]\n-->%%[[A]]%%>   \u0060\u0060\u0060\n\u0060\u0060[[A]] [[A]]%%[[A]]%%\u0060A\n===\n[[A\\]][[A\\|alias]]É\n- [ ] [[A]]\n~~~\n![[A#A]]0",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- [x] [[B]]\n## A-2\n## A\n## A\n0[[A\\|alias]] ^a\n ^a-2\n[[#A]]~~~~\n- item\n\n      \u0060\u0060\u0060\u0060\n[[image.png]] ^A\n\\\\[[A]]\u0060\u0060\u0060\n[[A\\]]<!--\n[[A]]\n-->%%[[A]]%%>   \u0060\u0060\u0060\n\u0060\u0060[[A]] [[A]]%%[[A]]%%\u0060A\n===\n[[A\\]][[A\\|alias]]É\n- [ ] [[A]]\n~~~\n![[A#A]]0",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-png-a-aa-a-a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[#A]]<!--\n[[A]]\n-->## A\n## A\nÉ\n\u0060\u0060\u0060\u0060\n~~~~\n> [!unknown] title\n[[A]]## <em>A</em>\n    0# A\n[[A#^a]]%%  \u0060\u0060\u0060\nÉ\n- > [!note] title\n![[A#A]]\u0060\u0060\u0060\u0060\n ^a-2\n[[A]] [[A]] ^é\n## A\n~~~~\n<div>\n[[A]]\n</div>\n- [ ] [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "--><div>\n[[A]]\n</div>\n[[A]]- > [!note] title\n[[A\\]]<!--\n[[A]]\n-->> [!unknown] title\n## <em>A</em>\n  > [!note] title\n## A\n\u0060[[A#A]]> [!note] [[A]]\nA\n  - \\[[A]]A\n[[A#^a]][^n]: [[A]]\n\n    [[B]]\n- > [!note] title\n-->\u0060<!--\n[[A]]\n-->0[^n]: [[A]]\n\n    [[B]]\n[[A#A]]0A\n---\n~~~\n![[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\[[A]]01. -->- [ ] [[A]]\nA\n===\n\n\n![[image.png]]\u0060[[A]]\u0060\u0060\u0060\u0060\u0060\n![[A]][^n]: [[A]]\n\n    [[B]]\n> [!note] title\n## A\n~~~\n ^a\n- > [!note] title\n> [!note] [[A]]\n## [[A|alias]]\n[[A#^a]]\u0060open\n[[A]]\nclose\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-01-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "0[[A#^a]]<!-- [[A]] -->\u0060\u0060\u0060\u0060\nA\n---\n## <em>A</em>\n# A\n \u0060\u0060\u0060\n\t",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!unknown] title\n  - - [[A#^a]]## A\n\t \u0060\u0060\u0060 [[A]]\ntext   > [!note] title\n> [!note] title\nA\n---\n ^A\n   \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## !\n[^unused]: [[A]]\n\n\n\u0060\u0060\u0060 [[A]]\n0  \u0060\u0060\u0060\n[[A#A]]> > [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060\u0060\n  > [!note] title\n[[B|alias]]\u0060\u0060 ^é\n0\u0060\u0060\\\\[[A]]   \u0060\u0060\u0060\n[[A\\|alias]]A\n===\n> [!note] one\n> [!note] two\n> [!note] three\n1. A\n===\n\u0060text -    \u0060\u0060\u0060\n## A\n> [!note] one\n> [!note] two\n> [!note] three\n[[image.png]][[image.png]]![[A#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"alias-é-0-a-aliasa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#A]]![[image.png]]^absent-prose ~~~~\n ^a-2\nA\n---\n\u0060<!-- [[A]] -->\n ^é\nA\n<!-- [[A]] -->\u0060\u0060\u0060\u0060 go [[A]]\n\n\n## A\n## A\n> [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060\u0060\u0060 go [[A]]\n![[image.png]]![[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-image-png-absent-prose-a-2-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "# A\n<!-- [[A]] -->A\n===\n## A\n## A\n\t<div>\n[[A]]\n</div>\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- > [!note] title\n^absent-prose 0> [!note] [[A]]\n<!-- [[A]] -->## [[A|alias]]\n> - <!--![[A#A]]<!-- [[A]] -->   \u0060\u0060\u0060\n[^unused]: [[A]]\n%%[[A]]%% ^a-2\n[[#A]]## A-2\n## A\n## A\n\u0060\u0060\u0060\u0060\u0060\u0060 go [[A]]\n- [ ] [[A]]\n## !\n[[A#^a]]\\\\[[A]]> > --> ^A\n![[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^a\nB\n章節\n## A-2\n## A\n## A\nÉ\n\u0060\u0060| a | b |\n|---|---|\n| [[A]] | ^a |\n ^a\nB\n![[A]]<!--\u0060## A\n## A\n## A\n- [x] [[B]]\n[[A\\]]\u0060\u0060\u0060\n\u0060\u0060![[A#A]]- [ ] [[A]]\n[[A\\|alias]]~~~~\n[[B|alias]]  \u0060\u0060\u0060\n![[A]]-->\u0060open\n[[A]]\nclose\u0060%%[[A]]%%\t\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n> [!unknown] title\n![[A]]## A\n## A\n\u0060\u00600[[A]] [[A]][[B|alias]] ^é\n[[image.png]]A\n===\n[[A#A]]\n \u0060open\n[[A]]\nclose\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0a-aalias-é-image-pnga\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n> [!unknown] title\n![[A]]## A\n## A\n\u0060\u00600[[A]] [[A]][[B|alias]] ^é\n[[image.png]]A\n===\n[[A#A]]\n \u0060open\n[[A]]\nclose\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#A]]> [!note] title\n<div>\n[[A]]\n</div>\n<div>\n[[A]]\n</div>\ntext \u0060[[A]]\u0060-->## A\nÉ\n\\\u0060- [[A]] [[A]]> <!--\n[[A]]\n--> ^a-2\n## [[A|alias]]\nB\n<!--\n[[A]]\n-->[[A#^a]]ref[^n]\nÉ\n## A-2\n## A\n## A\n\u0060\u0060\u0060\u0060\n- > [!note] title\n- > [!note] title\nA\n---\n[[#A]]1. ![[image.png]]\t\u0060\u0060\u0060\n\u0060\u0060\u0060\u0060 go [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [[A\\]]  > [!note] title\n- > [!note] title\n> > [!note] title\n1. ## [[A|alias]]\n  > [!note] title\nA\n---\n%%[[A]]%%[[B|alias]]0<!--![[image.png]][[#A]][[A\\|alias]] ^a\n![[A#A]][[#A]]\u0060\u0060## A\n> [!note] [[A]]\n## A-2\n## A\n## A\n\u0060\u0060\u0060\u0060\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> ## A\n## <em>A</em>\n[[A]] ^é\n  \u0060\u0060\u0060\n ^a-2\n[[A\\|alias]]   \u0060\u0060\u0060\n  \u0060\u0060\u0060\ntext [[A\\|alias]]- [x] [[B]]\n## A\n> [!note] title\nA\n---\n[[A#^a]]^absent-prose \n\nhttps://example.invalid/\u0060[[A]]\u0060 1. [[A\\|alias]]![[A]]- item\n\n      \n\n ^a\nA\n---\n[[A\nB]]\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> ## A\n## <em>A</em>\n[[A]] ^é\n  \u0060\u0060\u0060\n ^a-2\n[[A\\|alias]]   \u0060\u0060\u0060\n  \u0060\u0060\u0060\ntext [[A\\|alias]]- [x] [[B]]\n## A\n> [!note] title\nA\n---\n[[A#^a]]^absent-prose \n\nhttps://example.invalid/\u0060[[A]]\u0060 1. [[A\\|alias]]![[A]]- item\n\n      \n\n ^a\nA\n---\n[[A\nB]]\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^é\n# A\n\u0060\u0060<!--\t~~~\n[[A#A]][[B|alias]]https://example.invalid/\u0060[[A]]\u0060 - [x] [[B]]\n[^unused]: [[A]]\nhttps://example.invalid/\u0060[[A]]\u0060 | a | b |\n|---|---|\n| [[A]] | ^a |\n\\\u0060É\n<!--\n[[A]]\n-->A\n---\nÉ\n\t> [!note] [[A]]\n- [x] [[B]]\n\u0060[[A]]\u0060    - [x] [[B]]\nÉ\n> [!note] title\n<!-- [[A]] -->> [!unknown] title\n\u0060    \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#A]][[A]]B\n^absent-prose [[A\\]] ^é\n> [!unknown] title\n> [!note] one\n> [!note] two\n> [!note] three\n    \u0060\u0060\u0060\n  > [!note] title\n%%[[A]]%%## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n ^a-2\n0[[A\\]] ^é\nhttps://example.invalid/\u0060[[A]]\u0060  \u0060\u0060\u0060\n[^n]: [[A]]\n\n    [[B]]\n> [!unknown] title\nA\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\\|alias]]É\n- A\n---\n> [!note] [[A]]\n[[#A]]![[A]][[A#A]]A\n---\n- [x] [[B]]\n\\[[A]][[A]] [[A]]## A-2\n## A\n## A\n> [!note] [[A]]\n\\\u0060A\n[[B|alias]]É\n\\\u0060<div>\n[[A]]\n</div>\n ^a-2\n0\u0060open\n[[A]]\nclose\u0060 \u0060\u0060\u0060\n## [[A|alias]]\n  > [!note] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\\|alias]]É\n- A\n---\n> [!note] [[A]]\n[[#A]]![[A]][[A#A]]A\n---\n- [x] [[B]]\n\\[[A]][[A]] [[A]]## A-2\n## A\n## A\n> [!note] [[A]]\n\\\u0060A\n[[B|alias]]É\n\\\u0060<div>\n[[A]]\n</div>\n ^a-2\n0\u0060open\n[[A]]\nclose\u0060 \u0060\u0060\u0060\n## [[A|alias]]\n  > [!note] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-a-aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] [[A]]\n\\[[A]]- [ ] [[A]]\n \\[[A]]https://example.invalid/\u0060[[A]]\u0060   \u0060\u0060\u0060\nA\n---\n \u0060\u0060\u0060\n~~~\n## A\n## A\n\t\u0060\u0060\u0060\n0\n\\\u00600",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-a-https-example-invalid-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## !\n[[image.png]]https://example.invalid/\u0060[[A]]\u0060 A\n===\n0## A-2\n## A\n## A\ntext É\n%%\u0060\u0060![[image.png]] ^A\nB\n  - text [[A\\]]## A\n## A\n\\[[A]]- [ ] [[A]]\n\u0060%%[[A]]%%## <em>A</em>\n<!-- [[A]] -->- \\\u0060## A\n\\[[A]][[A\\]]~~~\n\u0060\u0060\u0060\u0060\n- [x] [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060[[A]]\u0060> [!note] title\n章節\n![[A#A]]\u0060## [[A|alias]]\n\u0060\u0060\u0060 [[A]]\n ^a\ntext ## A\n> [!note] title\n## A\n## A\n~~~~\n   \u0060\u0060\u0060\nA\n===\n[^n]: [[A]]\n\n    [[B]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!unknown] title\n\u0060\t~~~\n# A\n%%[[A]]%%<!--![[A]]É\n## [[A|alias]]\n- item\n\n      É\n> [!note] [[A]]\n   \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "00[[A\nB]]    \u0060\u0060\u0060\n\\\u0060A\n===\ntext ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"00-a-b-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\nA\n---\n章節\n<!--\n[[A]]\n-->## A\n## A\n![[image.png]]章節\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "# A\nÉ\n\\\\[[A]]> [!unknown] title\n[[A#^a]]\u0060\u0060\u0060 [[A]]\n\u0060open\n[[A]]\nclose\u0060# A\n\u0060open\n[[A]]\nclose\u0060A\n===\n[^n]: [[A]]\n\n    [[B]]\nA\n---\n## <em>A</em>\n[[A\\]]![[image.png]]| a | b |\n|---|---|\n| [[A]] | ^a |\ntext - > [!note] title\n<div>\n[[A]]\n</div>\n[[#A]]A\n<!-- [[A]] --><div>\n[[A]]\n</div>\n![[A#A]]![[A]]0[[A\\|alias]]  > [!note] title\n\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"é-a-unknown-title-a-a-a-open-a-close-a-open-a-closea\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060\u0060\u0060\n\u0060\u0060\u0060\u0060\n ^a\n ^a-2\nÉ\n[[A]] [[A]]章節\n- ref[^n]\n \u0060\u0060\u0060\n| a | b |\n|---|---|\n| [[A]] | ^a |\n-  ^é\n\u0060\u0060\u0060\n![[image.png]]## A\n## A\nhttps://example.invalid/\u0060[[A]]\u0060 > [!note] one\n> [!note] two\n> [!note] three\nA\n---\nÉ\n~~~\n## A\n> <!--",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%[[A]]%%[[A\\]]\u0060\u0060\u0060\n\\[[A]][[A\nB]]\u0060\u0060\u0060\u0060 go [[A]]\n0A\n===\n\u0060open\n[[A]]\nclose\u00600- [ ] [[A]]\n> [!note] [[A]]\n\t\u0060\u0060\u0060\n1.  ^A\n<!--\n[[A]]\n-->    \u0060\u0060\u0060\n## A\n[[A#^a]]\t\u0060\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-a-b-go-a-0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\t\u0060\u0060\u0060\n\nB\n\\\\[[A]]\u0060open\n[[A]]\nclose\u0060[[A\nB]]    \u0060\u0060\u0060\n- <!--\n[[A]]\n--><!--\n[[A]]\n-->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"b-aopen-a-close-a-b\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "https://example.invalid/\u0060[[A]]\u0060 \n\n![[image.png]]ref[^n]\n\\[[A]]  - \u0060open\n[[A]]\nclose\u0060[[A#A]]> [!unknown] title\n\u0060\u0060- > [!note] title\nhttps://example.invalid/\u0060[[A]]\u0060 A\n===\n> [!note] title\n\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-png-ref-n-a-open-a-closea-a-unknown-title-note-title-https-example-invalid-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "   \u0060\u0060\u0060\n ^é\n> [!note] [[A]]\n\u0060\u0060\u0060\u0060 go [[A]]\n~~~~\n ^é\n>  ^é\n ^é\n ^a\n\u0060\u0060\u0060\u0060\n%%[[A]]%%<!-- [[A]] -->    A\n---\n![[image.png]]A\n===\n~~~\n<!--\n[[A]]\n-->\u0060\u0060\u0060 [[A]]\n[[#A]][[A#^a]][[#A]]ref[^n]\n[[#A]]0[[A\\|alias]]https://example.invalid/\u0060[[A]]\u0060 ## A\n## A\n    \u0060\u0060\u0060\n> [!note] [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-png-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "章節\n<!--\n[[A]]\n-->A\n===\n \u0060\u0060\u0060\nA\n===\nA\n---\n\u0060\u0060\u0060 [[A]]\n- > [!note] title\n-->![[image.png]]  - | a | b |\n|---|---|\n| [[A]] | ^a |\n ^a\n[[B|alias]]^absent-prose https://example.invalid/\u0060[[A]]\u0060 > [!note] [[A]]\n  \u0060\u0060\u0060\n ^a\n[[#A]]https://example.invalid/\u0060[[A]]\u0060 A\n===\n# A\n \u0060\u0060\u0060\n  > [!note] title\n1. \n\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n[[A\\|alias]]> [!note] one\n> [!note] two\n> [!note] three\n%%[[A]]%%A\n===\n- ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n\n\n> [!unknown] title\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\\[[A]]\u0060> [!unknown] title\nA\n---\nref[^n]\n%%[[A]]%%> [!note] title\n^absent-prose A\n===\n<!--\n[[A]]\n-->[^unused]: [[A]]\n## A\n1. A\n---\n[[A\nB]] ^é\n\u0060[[A]]\u0060## A\n## A\n01. ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n\n\n> [!unknown] title\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\\[[A]]\u0060> [!unknown] title\nA\n---\nref[^n]\n%%[[A]]%%> [!note] title\n^absent-prose A\n===\n<!--\n[[A]]\n-->[^unused]: [[A]]\n## A\n1. A\n---\n[[A\nB]] ^é\n\u0060[[A]]\u0060## A\n## A\n01. ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"absent-prose-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^a-2\n## [[A|alias]]\n[[#A]] ^é\nA\n===\n%%[[A]]%%## A\n## A\n\t    \u0060\u0060\u0060\n ^a\n<!-- \u0060\u0060\u0060\n   \u0060\u0060\u0060\n## A-2\n## A\n## A\n\n## A-2\n## A\n## A\n   \u0060\u0060\u0060\n> [!note] one\n> [!note] two\n> [!note] three\nÉ\n> > <!-- [[A]] -->\t",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!unknown] title\n--># A\n\u0060É\n  > [!note] title\nÉ\n\\\\[[A]]\u0060\u0060\u0060\u0060 go [[A]]\nA\n---\n0É\n> > ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"é-a-go-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!note] [[A]]\n![[A]][[A]] [[A]]\u0060\u0060\u0060 [[A]]\n\\\\[[A]]\u0060\u0060\u0060 [[A]]\n 章節\n![[A]]A\n===\n> [!note] [[A]]\n~~~~\n> ![[A]]É\n[[A\\]]\\\\[[A]]\\[[A]]~~~~\ntext ## A-2\n## A\n## A\n0\u0060[[A]]\u0060\u0060open\n[[A]]\nclose\u0060[[#A]]   \u0060\u0060\u0060\n![[A]][[#A]] https://example.invalid/\u0060[[A]]\u0060 \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-a-a-a-a-章節-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n---\n## !\n章節\n## A-2\n## A\n## A\n ^a\n[^unused]: [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\nÉ\n ^é\n- [x] [[B]]\n\t[[#A]]%%É\n[[image.png]]0 ^a\n| a | b |\n|---|---|\n| [[A]] | ^a |\nA\n![[A#A]][[A#A]]<div>\n[[A]]\n</div>\n\nA\n---\n## A\n    <div>\n[[A]]\n</div>\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n---\n## !\n章節\n## A-2\n## A\n## A\n ^a\n[^unused]: [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\nÉ\n ^é\n- [x] [[B]]\n\t[[#A]]%%É\n[[image.png]]0 ^a\n| a | b |\n|---|---|\n| [[A]] | ^a |\nA\n![[A#A]][[A#A]]<div>\n[[A]]\n</div>\n\nA\n---\n## A\n    <div>\n[[A]]\n</div>\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A]] [[A]]%%![[A#A]]## A\n## A\n%%text   > [!note] title\nÉ\n%%[[A]]%%A\n---\n[^n]: [[A]]\n\n    [[B]]\n![[A#A]][^unused]: [[A]]\nA\n> [!unknown] title\n![[image.png]]https://example.invalid/\u0060[[A]]\u0060 \t-->A\n===\n[[A\\|alias]]\\\\[[A]]\\[[A]]![[A]] ^é\n \u0060\u0060\u0060\n# A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-png-https-example-invalid-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060\u0060[[#A]]\u0060open\n[[A]]\nclose\u0060É\n    \u0060\u0060\u0060\nA\n===\n    %%[[A]]%%## <em>A</em>\n    \u0060<!-- [[A]] -->章節\n\t\u0060\u0060\u0060\n-->[[A\\]]    \u0060\u0060\u0060\u0060 go [[A]]\n \u0060\u0060\u0060\n ^a\nÉ\n[[A#^a]]> [!note] title\n\n![[image.png]] ^A\n[[#A]]<!--",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"aopen-a-closeé-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[image.png]]\u0060\u0060\u0060 [[A]]\n\\\u0060<div>\n[[A]]\n</div>\n> [!note] title\n## [[A|alias]]\n[[B|alias]]## <em>A</em>\n-->\u0060open\n[[A]]\nclose\u0060\\[[A]][[A]] [[A]]![[A#A]]  \u0060\u0060\u0060\nA\n===\n[[image.png]]<!--[[A\nB]]  \u0060\u0060\u0060\n章節\n## A\n    -->[[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"alias-em-a-em-open-a-close-a-a-a-a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\\\[[A]]--><!--\n[[A]]\n-->## A-2\n## A\n## A\n1. text [[#A]]~~~~\n[[A\\|alias]]| a | b |\n|---|---|\n| [[A]] | ^a |\n[^n]: [[A]]\n\n    [[B]]\n![[image.png]]https://example.invalid/\u0060[[A]]\u0060 \u0060\u0060\u0060\n[[A\nB]]  > [!note] title\n- > [!note] title\n![[A]]  - \nA\n[[A]] [[A]] ^é\n[[A\\]]A\n===\n\\\u0060\u0060\u0060\u0060\u0060 go [[A]]\n\t\u0060\u0060\u0060\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\\\[[A]]--><!--\n[[A]]\n-->## A-2\n## A\n## A\n1. text [[#A]]~~~~\n[[A\\|alias]]| a | b |\n|---|---|\n| [[A]] | ^a |\n[^n]: [[A]]\n\n    [[B]]\n![[image.png]]https://example.invalid/\u0060[[A]]\u0060 \u0060\u0060\u0060\n[[A\nB]]  > [!note] title\n- > [!note] title\n![[A]]  - \nA\n[[A]] [[A]] ^é\n[[A\\]]A\n===\n\\\u0060\u0060\u0060\u0060\u0060 go [[A]]\n\t\u0060\u0060\u0060\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n  > [!note] title\n<!-- [[A]] -->## <em>A</em>\n\u0060\u0060\u0060 [[A]]\n ^a-2\n![[A]]- [x] [[B]]\nhttps://example.invalid/\u0060[[A]]\u0060     \u0060\u0060\u0060\n# A\n<!-- [[A]] -->\u0060\u0060\u0060\n[[A\\]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n\n[[B|alias]]~~~\n<div>\n[[A]]\n</div>\n   \u0060\u0060\u0060\n   \u0060\u0060\u0060\n> [!note] one\n> [!note] two\n> [!note] three\n[^unused]: [[A]]\n\u0060open\n[[A]]\nclose\u0060 ^a\nA\n---\nÉ\n## A-2\n## A\n## A\n\t\u0060\u0060\u0060\n\u0060\u0060  > [!note] title\n\u0060\u0060\u0060\u0060\n> [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060\u0060 [[A]]\n\\[[A]]<!-- [[A]] -->[[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## !\nA\n===\n[[A\nB]]\u0060\u0060\u0060\u0060 go [[A]]\n[[A\\]]> [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060    \\\\[[A]]  - | a | b |\n|---|---|\n| [[A]] | ^a |\n ^a-2\n- > [!note] title\n<!-- [[A]] -->![[image.png]]> [!note] [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n<!--\n[[A]]\n-->## A-2\n## A\n## A\n\\\\[[A]]\n\n<!-- [[A]] -->> [!note] [[A]]\n ^é\n## !\n0\u0060\u0060^absent-prose ![[A#A]]A\n<div>\n[[A]]\n</div>\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## !\nA\n===\n[[A\nB]]\u0060\u0060\u0060\u0060 go [[A]]\n[[A\\]]> [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060    \\\\[[A]]  - | a | b |\n|---|---|\n| [[A]] | ^a |\n ^a-2\n- > [!note] title\n<!-- [[A]] -->![[image.png]]> [!note] [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n<!--\n[[A]]\n-->## A-2\n## A\n## A\n\\\\[[A]]\n\n<!-- [[A]] -->> [!note] [[A]]\n ^é\n## !\n0\u0060\u0060^absent-prose ![[A#A]]A\n<div>\n[[A]]\n</div>\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## !\nA\n===\n[[A\nB]]\u0060\u0060\u0060\u0060 go [[A]]\n[[A\\]]> [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060    \\\\[[A]]  - | a | b |\n|---|---|\n| [[A]] | ^a |\n ^a-2\n- > [!note] title\n<!-- [[A]] -->![[image.png]]> [!note] [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n<!--\n[[A]]\n-->## A-2\n## A\n## A\n\\\\[[A]]\n\n<!-- [[A]] -->> [!note] [[A]]\n ^é\n## !\n0\u0060\u0060^absent-prose ![[A#A]]A\n<div>\n[[A]]\n</div>\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"section-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\n## !\n<!--\n[[A]]\n-->## !\ntext <!--\n[[A]]\n-->\\\\[[A]]\\[[A]]- [ ] [[A]]\n\n[^unused]: [[A]]\n[^n]: [[A]]\n\n    [[B]]\n- [x] [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"section-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060 ^é\nÉ\n    \u0060\u0060\u0060\n章節\n![[A#A]]> https://example.invalid/\u0060[[A]]\u0060 %% ^a-2\n%%## A-2\n## A\n## A\n\u0060open\n[[A]]\nclose\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- > [!note] title\nref[^n]\n## A\n## A\n%%[[A]]%%> [!note] [[A]]\nhttps://example.invalid/\u0060[[A]]\u0060 A\n---\n  \u0060\u0060\u0060\n%%ref[^n]\n    \u0060\u0060\u0060\n~~~~\n\ttext   \u0060\u0060\u0060\n\u0060\u0060\u0060 [[A]]\n        [[A\nB]]\u0060\u0060\u0060 [[A]]\nref[^n]\n\n\n\u0060open\n[[A]]\nclose\u0060A\n===\n\u0060ref[^n]\n\t\u0060\u0060\u0060\n  \u0060\u0060\u0060\n\u0060\u0060\u0060\u0060 go [[A]]\n\u0060\u0060\u0060 [[A]]\n ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- > [!note] title\nref[^n]\n## A\n## A\n%%[[A]]%%> [!note] [[A]]\nhttps://example.invalid/\u0060[[A]]\u0060 A\n---\n  \u0060\u0060\u0060\n%%ref[^n]\n    \u0060\u0060\u0060\n~~~~\n\ttext   \u0060\u0060\u0060\n\u0060\u0060\u0060 [[A]]\n        [[A\nB]]\u0060\u0060\u0060 [[A]]\nref[^n]\n\n\n\u0060open\n[[A]]\nclose\u0060A\n===\n\u0060ref[^n]\n\t\u0060\u0060\u0060\n  \u0060\u0060\u0060\n\u0060\u0060\u0060\u0060 go [[A]]\n\u0060\u0060\u0060 [[A]]\n ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"https-example-invalid-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]][[#A]]0\u0060open\n[[A]]\nclose\u0060[[A\nB]]\\[[A]]\\\\[[A]] ^A\n[[#A]]    [[image.png]]章節\n章節\n\u0060[[A]]\u0060\u0060open\n[[A]]\nclose\u0060A\n---\n    \u0060\u0060\u0060\n ^A\n ^a-2\n~~~\n\\\u0060É\n[[A]]ref[^n]\n\u0060-->\u0060B\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-a0open-a-close-a-b-a-a-a-a-image-png章節-章節-a-open-a-closea\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "0## A-2\n## A\n## A\nhttps://example.invalid/\u0060[[A]]\u0060 %%~~~~\n[[A\\|alias]]1. %%[[A]]%%## A-2\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n%%   \u0060\u0060\u0060\n[[B|alias]]## !\n<!-- [[A]] -->  \u0060\u0060\u0060\n ^A\n[[image.png]]\\\\[[A]]>   - [^n]: [[A]]\n\n    [[B]]\n\t[^unused]: [[A]]\nA\n===\n<!--\n[[A]]\n--> [[A]] [[A]]  > [!note] title\n    ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n%%   \u0060\u0060\u0060\n[[B|alias]]## !\n<!-- [[A]] -->  \u0060\u0060\u0060\n ^A\n[[image.png]]\\\\[[A]]>   - [^n]: [[A]]\n\n    [[B]]\n\t[^unused]: [[A]]\nA\n===\n<!--\n[[A]]\n--> [[A]] [[A]]  > [!note] title\n    ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] title\n\u0060[[A#^a]]^absent-prose https://example.invalid/\u0060[[A]]\u0060 \u0060^absent-prose > [!note] title\n## [[A|alias]]\n- <!-- [[A]] -->## [[A|alias]]\n\u0060\u0060\u0060\ntext ## [[A|alias]]\n![[A#A]][[image.png]]> ~~~~\nhttps://example.invalid/\u0060[[A]]\u0060 ![[A#A]]\t[[A\\|alias]]%%## A-2\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"alias-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n~~~~\n- item\n\n      %%[[A]]%%%%<!--\n[[A]]\n-->> [!note] title\n~~~~\n## A\n## A\nref[^n]\n ^é\nA\n===\n<div>\n[[A]]\n</div>\n> [!unknown] title\n# A\n- > [!note] title\n\u0060[[A]]\u0060## A-2\n## A\n## A\n[[A\\]]> ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n~~~~\n- item\n\n      %%[[A]]%%%%<!--\n[[A]]\n-->> [!note] title\n~~~~\n## A\n## A\nref[^n]\n ^é\nA\n===\n<div>\n[[A]]\n</div>\n> [!unknown] title\n# A\n- > [!note] title\n\u0060[[A]]\u0060## A-2\n## A\n## A\n[[A\\]]> ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n~~~~\n- item\n\n      %%[[A]]%%%%<!--\n[[A]]\n-->> [!note] title\n~~~~\n## A\n## A\nref[^n]\n ^é\nA\n===\n<div>\n[[A]]\n</div>\n> [!unknown] title\n# A\n- > [!note] title\n\u0060[[A]]\u0060## A-2\n## A\n## A\n[[A\\]]> ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n~~~~\n- item\n\n      %%[[A]]%%%%<!--\n[[A]]\n-->> [!note] title\n~~~~\n## A\n## A\nref[^n]\n ^é\nA\n===\n<div>\n[[A]]\n</div>\n> [!unknown] title\n# A\n- > [!note] title\n\u0060[[A]]\u0060## A-2\n## A\n## A\n[[A\\]]> ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A]] [[A]]![[A]] ^é\n0[[image.png]] ^a\n> [!unknown] title\n0A\n===\n ^é\n\\\u0060- item\n\n      ~~~\n%%[[A]]%%![[A]][[B|alias]]## !\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\\]]1. B\n[^unused]: [[A]]\ntext [^unused]: [[A]]\n[[A#^a]][[#A]][[A#A]]<!-- [[A]] -->> [!note] one\n> [!note] two\n> [!note] three\n\n\n> [!note] [[A]]\n0<!-- [[A]] -->A\n---\n> [!note] title\n[[A\\]]\u0060\u0060\u0060\u0060 go [[A]]\n- [ ] [[A]]\n ^a\n> [[A#^a]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  > [!note] title\nA\n===\nÉ\n[[A\nB]]## !\n## [[A|alias]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n## A-2\n## A\n## A\n[^unused]: [[A]]\n%%<!--\n[[A]]\n-->--># A\n0![[image.png]]# A\n## [[A|alias]]\n- [x] [[B]]\nref[^n]\n ^é\n\u0060\u0060\u0060\u0060 go [[A]]\n![[image.png]]text  \u0060\u0060\u0060\n- > [!note] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## [[A|alias]]\n\u0060\u0060<!--\n[[A]]\n-->[^unused]: [[A]]\n- > [!note] title\n[[B|alias]]ref[^n]\n^absent-prose A\n===\n[[A#A]]## A\n> [!unknown] title\nA\n===\n> [!note] [[A]]\n\u0060\u0060^absent-prose text ^absent-prose   - ## A-2\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## [[A|alias]]\n\u0060\u0060<!--\n[[A]]\n-->[^unused]: [[A]]\n- > [!note] title\n[[B|alias]]ref[^n]\n^absent-prose A\n===\n[[A#A]]## A\n> [!unknown] title\nA\n===\n> [!note] [[A]]\n\u0060\u0060^absent-prose text ^absent-prose   - ## A-2\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^é\n<!--## [[A|alias]]\n<!-- [[A]] -->> [!unknown] title\n\\\\[[A]]\u0060\u0060\u0060\u0060\n0[[A\\]]  > [!note] title\n\\\\[[A]]> [!note] title\nA\n===\n<!--\n[[A]]\n-->\\[[A]]%%[[A]]%%\u0060\n\n\t \u0060\u0060\u0060\ntext ![[A#A]]> [!note] [[A]]\n> > [^n]: [[A]]\n\n    [[B]]\n[[A]] [[A]]> [!note] one\n> [!note] two\n> [!note] three\nA\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^é\n<!--## [[A|alias]]\n<!-- [[A]] -->> [!unknown] title\n\\\\[[A]]\u0060\u0060\u0060\u0060\n0[[A\\]]  > [!note] title\n\\\\[[A]]> [!note] title\nA\n===\n<!--\n[[A]]\n-->\\[[A]]%%[[A]]%%\u0060\n\n\t \u0060\u0060\u0060\ntext ![[A#A]]> [!note] [[A]]\n> > [^n]: [[A]]\n\n    [[B]]\n[[A]] [[A]]> [!note] one\n> [!note] two\n> [!note] three\nA\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-0a-note-title-a-note-title-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n\n\n[[image.png]]0\n[[image.png]]\u0060\u0060\u0060 [[A]]\n ^é\n0^absent-prose A\n===\n%%text ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-png0-image-png-a-é-0-absent-prose-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\n\u0060[[A]]\u0060~~~~\n\u0060open\n[[A]]\nclose\u0060# A\n<!-- [[A]] -->0\\[[A]]## <em>A</em>\n[[A\\|alias]]A\n===\n ## <em>A</em>\n# A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-open-a-close-a-0-a-em-a-em-aliasa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[image.png]][[image.png]]0- > [!note] title\n章節\n[[A\\]][[A\\]]> - [ ] [[A]]\n> [!note] title\n> [!unknown] title\n[^unused]: [[A]]\nA\n===\n<!--[[#A]]\\[[A]]-->## A\n[[B|alias]]## !\n\\\u0060章節\n[[B|alias]]章節\n## A-2\n## A\n## A\n- [x] [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[image.png]][[image.png]]0- > [!note] title\n章節\n[[A\\]][[A\\]]> - [ ] [[A]]\n> [!note] title\n> [!unknown] title\n[^unused]: [[A]]\nA\n===\n<!--[[#A]]\\[[A]]-->## A\n[[B|alias]]## !\n\\\u0060章節\n[[B|alias]]章節\n## A-2\n## A\n## A\n- [x] [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^é\n ^a\n\\\\[[A]]## <em>A</em>\n\u0060open\n[[A]]\nclose\u0060[[A#^a]]A\n===\n ^é\n   \u0060\u0060\u0060\n\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"é-a-a-em-a-em-open-a-closea-aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\t\u0060\u0060\u0060\n![[A]] \u0060\u0060\u0060\n<!--\n[[A]]\n-->> [!note] one\n> [!note] two\n> [!note] three\n    \u0060\u0060\u0060 [[A]]\n# A\n%%[[A]]%% ^a-2\n\u0060open\n[[A]]\nclose\u0060\u0060[[A]]\u0060A\n===\n\t\u0060\u0060\u0060\n  \u0060\u0060\u0060\n## A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n## A\n0\u0060[[A]]\u0060> [!unknown] title\n\u0060[[A]]\u0060https://example.invalid/\u0060[[A]]\u0060 1.  ^é\n\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2-open-a-close-aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## !\n## A\n## A\nref[^n]\n\u0060\u0060> [!unknown] title\nA\n===\n- \u0060ref[^n]\n[[A]]^absent-prose  ^é\nA\n---\n ^A\nÉ\n%%[[A]]%%> [[#A]]%%\t\u0060\u0060\u0060\n[[#A]]\n\n[[A\\|alias]]ref[^n]\nA\n===\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060[[A]]\u0060[[A\\]]> [!note] one\n> [!note] two\n> [!note] three\n\\[[A]]É\n- [[A\\]][^n]: [[A]]\n\n    [[B]]\n0\n\nA\n===\ntext %%[[A]]%%\t\u0060\u0060\u0060\n![[A#A]]\t\u0060\u0060\u0060\n<div>\n[[A]]\n</div>\n- item\n\n       \u0060\u0060\u0060\n[[A\\|alias]]- > [!note] title\n0A\n---\n![[image.png]]![[A]]\u0060\u0060![[image.png]][^n]: [[A]]\n\n    [[B]]\n-->  > [!note] title\n - > [!note] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"alias-note-title-0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "É\n[[#A]]\\\\[[A]]> [[A#^a]][[A#^a]]- [x] [[B]]\n\\\\[[A]]0A\n---\nhttps://example.invalid/\u0060[[A]]\u0060 ^absent-prose - > [!note] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"é-a-a-a-aa-a-x-b-a0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- [x] [[B]]\n- > [!note] title\n## A\n  - ## [[A|alias]]\n<!--\n[[A]]\n-->## [[A|alias]]\n[[A]] [[A]]\u0060- > [!note] title\n~~~~\n\n\n> [!note] one\n> [!note] two\n> [!note] three\nA\n---\n\u0060[[image.png]]  \u0060\u0060\u0060\n%%É\n~~~~\n^absent-prose  \u0060\u0060\u0060\n> > [[A#A]]ref[^n]\n\\[[A]]<div>\n[[A]]\n</div>\n- item\n\n      \u0060\u0060[[A]]> [!note] title\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"alias-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "^absent-prose  ^é\n> [!note] one\n> [!note] two\n> [!note] three\n^absent-prose    \u0060\u0060\u0060\nA\n---\n\u0060open\n[[A]]\nclose\u0060> [!note] [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"absent-prose-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n![[A#A]] ^é\n%%[[A]]%% ![[A]]A\n---\n[[B|alias]]\n> 0https://example.invalid/\u0060[[A]]\u0060  0text É\n\n[[image.png]]\n## A-2\n## A\n## A\n\\\\[[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n![[A#A]] ^é\n%%[[A]]%% ![[A]]A\n---\n[[B|alias]]\n> 0https://example.invalid/\u0060[[A]]\u0060  0text É\n\n[[image.png]]\n## A-2\n## A\n## A\n\\\\[[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n![[A#A]] ^é\n%%[[A]]%% ![[A]]A\n---\n[[B|alias]]\n> 0https://example.invalid/\u0060[[A]]\u0060  0text É\n\n[[image.png]]\n## A-2\n## A\n## A\n\\\\[[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-é-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "章節\n<!--\n[[A]]\n--> ^é\n    \u0060\u0060\u0060\n\u0060\t\u0060\u0060\u0060\n  > [!note] title\n-->\\\u0060[[A\\]]<!-- [[A]] -->[[B|alias]]\u0060open\n[[A]]\nclose\u0060[[A\nB]]ref[^n]\n[[B|alias]]1. A\n===\n ^a-2\n ^A\n- > [!note] title\n<!--\n[[A]]\n-->> [!unknown] title\n> [!unknown] title\n> B\n> > > [!note] one\n> [!note] two\n> [!note] three\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!unknown] title\n- item\n\n      ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-b-alias-open-a-close-a-b-ref-n-alias1-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\nhttps://example.invalid/\u0060[[A]]\u0060  ^A\n[[B|alias]][^n]: [[A]]\n\n    [[B]]\n\u0060open\n[[A]]\nclose\u0060\u0060[[A]]\u0060\u0060open\n[[A]]\nclose\u0060\u0060\u0060<!--\n[[A]]\n-->> [!unknown] title\nA\n===\n    <!--> [!note] title\nB\nA\n===\n0\u0060\u0060\u0060 [[A]]\n~~~~\n\u0060\u0060 ~~~~\nref[^n]\n章節\n## A\n0[[B|alias]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"b-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^é\n    \u0060\u0060\u0060\n[[A#A]] ^a\n[[A]]%%[[A]]%%\u0060\u0060\u0060\n0![[image.png]]~~~\n  > [!note] title\n## !\n## A\n## A\n<!--\n[[A]]\n-->## !\n[[A]]ref[^n]\n~~~~\n[[A\\|alias]] ^é\n\\\\[[A]]![[image.png]]- [ ] [[A]]\n- [ ] [[A]]\n[[A#^a]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"section-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060\u0060ref[^n]\n0- > [!note] title\n    \u0060\u0060\u0060\n## <em>A</em>\n[[A\\|alias]]        \n\n## <em>A</em>\nÉ\n ^é\n%%\u0060\u0060\u0060 [[A]]\n[[A\\]]- [x] [[B]]\n\u0060open\n[[A]]\nclose\u0060![[A#A]]![[image.png]]<!--> [!unknown] title\n# A\n[^n]: [[A]]\n\n    [[B]]\n## !\n[^unused]: [[A]]\n0[^unused]: [[A]]\n## A\n# A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"em-a-em-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n---\nÉ\n ^é\n![[image.png]]> > ## !\n%%[[A]]%%## A\n<div>\n[[A]]\n</div>\n  - ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->## A-2\n## A\n## A\nA\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  - B\n[[image.png]][[A#A]][[A#^a]]A\n^absent-prose \\[[A]][[A#^a]]   \u0060\u0060\u0060\n## A\n## A\n\u0060\u0060\u0060 [[A]]\n\u0060\u0060\u0060\u0060\n0[[image.png]]<!--\n[[A]]\n-->%%| a | b |\n|---|---|\n| [[A]] | ^a |\nB\n    \u0060\u0060\u0060\n## [[A|alias]]\n\u0060open\n[[A]]\nclose\u0060\u0060\u0060\u0060\n- [ ] [[A]]\n  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  - ref[^n]\n%%[[A]]%%B\n[[A]] [[A]]\\[[A]]# A\n> [!note] [[A]]\nA\n===\n\\\u0060A\n  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- item\n\n       ^a\n\u0060\u0060[[A#^a]]\u0060É\n  - ## [[A|alias]]\n## <em>A</em>\n## <em>A</em>\n\u0060<!--## A\n## A\n- [x] [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"em-a-em-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[^n]: [[A]]\n\n    [[B]]\n  > [!note] title\n## [[A|alias]]\n> [!unknown] title\n[[A\\]]ref[^n]\n[[B|alias]]## !\nÉ\n[[A#A]]> [!note] title\n> <!-- [[A]] -->[[A\\]]\u0060\u0060\u0060\u0060 go [[A]]\n\u0060[[A#A]]A\n===\n| a | b |\n|---|---|\n| [[A]] | ^a |\n## A-2\n## A\n## A\n ^é\n## !\n## A\n## A\n ^é\n[[A#A]]- > [!note] title\n^absent-prose \u0060\u0060[[A]]\u0060- > [!note] title\n\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[^n]: [[A]]\n\n    [[B]]\n  > [!note] title\n## [[A|alias]]\n> [!unknown] title\n[[A\\]]ref[^n]\n[[B|alias]]## !\nÉ\n[[A#A]]> [!note] title\n> <!-- [[A]] -->[[A\\]]\u0060\u0060\u0060\u0060 go [[A]]\n\u0060[[A#A]]A\n===\n| a | b |\n|---|---|\n| [[A]] | ^a |\n## A-2\n## A\n## A\n ^é\n## !\n## A\n## A\n ^é\n[[A#A]]- > [!note] title\n^absent-prose \u0060\u0060[[A]]\u0060- > [!note] title\n\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[^n]: [[A]]\n\n    [[B]]\n  > [!note] title\n## [[A|alias]]\n> [!unknown] title\n[[A\\]]ref[^n]\n[[B|alias]]## !\nÉ\n[[A#A]]> [!note] title\n> <!-- [[A]] -->[[A\\]]\u0060\u0060\u0060\u0060 go [[A]]\n\u0060[[A#A]]A\n===\n| a | b |\n|---|---|\n| [[A]] | ^a |\n## A-2\n## A\n## A\n ^é\n## !\n## A\n## A\n ^é\n[[A#A]]- > [!note] title\n^absent-prose \u0060\u0060[[A]]\u0060- > [!note] title\n\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A]] [[A]]## A-2\n## A\n## A\n0<div>\n[[A]]\n</div>\n  - \\\u0060[[image.png]]> [!note] one\n> [!note] two\n> [!note] three\n ^é\nA\nA\n---\n    \u0060\u0060\u0060\n![[A#A]]A\n===\n ^é\n\t\u0060\u0060\u0060\n\n<!--  > [!note] title\n> [!note] [[A]]\n[[#A]] ^a\n- [ ] [[A]]\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]]\u0060[[A]]\u0060\n ^a-2\n0",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A]] [[A]]## A-2\n## A\n## A\n0<div>\n[[A]]\n</div>\n  - \\\u0060[[image.png]]> [!note] one\n> [!note] two\n> [!note] three\n ^é\nA\nA\n---\n    \u0060\u0060\u0060\n![[A#A]]A\n===\n ^é\n\t\u0060\u0060\u0060\n\n<!--  > [!note] title\n> [!note] [[A]]\n[[#A]] ^a\n- [ ] [[A]]\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]]\u0060[[A]]\u0060\n ^a-2\n0",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A]] [[A]]## A-2\n## A\n## A\n0<div>\n[[A]]\n</div>\n  - \\\u0060[[image.png]]> [!note] one\n> [!note] two\n> [!note] three\n ^é\nA\nA\n---\n    \u0060\u0060\u0060\n![[A#A]]A\n===\n ^é\n\t\u0060\u0060\u0060\n\n<!--  > [!note] title\n> [!note] [[A]]\n[[#A]] ^a\n- [ ] [[A]]\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]]\u0060[[A]]\u0060\n ^a-2\n0",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"é-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n0[[A\\]]-->## A\nA\n===\n\n%% ^A\n[[A#A]]%%\u0060[[A]]\u0060~~~~\nref[^n]\n<!--\n[[A]]\n--><div>\n[[A]]\n</div>\n ^a-2\n ^a\n ^é\n章節\n\u0060[[A]]\u0060> [!note] title\n[[A]] [[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "章節\nA\n---\n<div>\n[[A]]\n</div>\n  > [!note] title\n## A\n## A\ntext ~~~\n[[A\\|alias]] ^a-2\n    text \u0060open\n[[A]]\nclose\u0060- [x] [[B]]\n[[A#^a]][[A\nB]]\u0060open\n[[A]]\nclose\u00601. 0^absent-prose   -  ^é\n![[A#A]]\n> \u0060\u0060\u0060\u0060\n-->[^n]: [[A]]\n\n    [[B]]\n## <em>A</em>\n[[B|alias]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n# A\n[^n]: [[A]]\n\n    [[B]]\n> >  ^a\n## A\n## A\n- [x] [[B]]\n  > [!note] title\n-->![[A]]text 章節\n0![[image.png]]> [!unknown] title\nA\n---\n \u0060\u0060\u0060\n\u0060\u0060\t[[B|alias]]\u0060\u0060\u0060 [[A]]\n\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n# A\n[^n]: [[A]]\n\n    [[B]]\n> >  ^a\n## A\n## A\n- [x] [[B]]\n  > [!note] title\n-->![[A]]text 章節\n0![[image.png]]> [!unknown] title\nA\n---\n \u0060\u0060\u0060\n\u0060\u0060\t[[B|alias]]\u0060\u0060\u0060 [[A]]\n\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n# A\n[^n]: [[A]]\n\n    [[B]]\n> >  ^a\n## A\n## A\n- [x] [[B]]\n  > [!note] title\n-->![[A]]text 章節\n0![[image.png]]> [!unknown] title\nA\n---\n \u0060\u0060\u0060\n\u0060\u0060\t[[B|alias]]\u0060\u0060\u0060 [[A]]\n\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n# A\n[^n]: [[A]]\n\n    [[B]]\n> >  ^a\n## A\n## A\n- [x] [[B]]\n  > [!note] title\n-->![[A]]text 章節\n0![[image.png]]> [!unknown] title\nA\n---\n \u0060\u0060\u0060\n\u0060\u0060\t[[B|alias]]\u0060\u0060\u0060 [[A]]\n\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-text-章節-0-image-png-unknown-title-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "      > [!note] title\n> [!note] title\n\\[[A]]A\n---\n[^unused]: [[A]]\nref[^n]\n\u0060\u0060\u0060\u0060\n  \u0060\u0060\u0060\n~~~~\n> ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A-2\n## A\n## A\n章節\n> [!note] one\n> [!note] two\n> [!note] three\n[[A\nB]][[A]] [[A]]\u0060\u0060\u0060 [[A]]\n\\\\[[A]] - [ ] [[A]]\n[^n]: [[A]]\n\n    [[B]]\n> [!note] title\n    %%## <em>A</em>\n[[A\\]]%%[[A]]%%\u0060\u0060\u0060\u0060 go [[A]]\n## !\n[[#A]]![[A]][[A]] [[A]]    ![[image.png]]~~~~\n- item\n\n      %%- > [!note] [[A]]\n- item\n\n      - item\n\n      [[A\\]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->  \u0060\u0060\u0060\n\\\\[[A]]A\n> [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060[[#A]][^unused]: [[A]]\n- > [!note] title\n\u0060\u0060\u0060\u0060\nA\n[[A\\|alias]]## [[A|alias]]\nÉ\nA\n---\n## !\n[[#A]]## A-2\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^a-2\n    <!--\n[[A]]\n-->## !\n\n\n\u0060\u0060## A-2\n## A\n## A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[B|alias]]| a | b |\n|---|---|\n| [[A]] | ^a |\n\u0060- [x] [[B]]\n[[B|alias]]text [^unused]: [[A]]\n[^unused]: [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->## A\n<!-- [[A]] -->\u0060\u0060A\n---\n![[A]]   \u0060\u0060\u0060\n\n\n[[#A]]\u0060open\n[[A]]\nclose\u0060> [!note] title\n章節\n<!-- ^é\n<!-- [[A]] -->\n\n## A\n[[A]][[#A]]> [!note] one\n> [!note] two\n> [!note] three\n[[A]] [[A]][^n]: [[A]]\n\n    [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->## A\n<!-- [[A]] -->\u0060\u0060A\n---\n![[A]]   \u0060\u0060\u0060\n\n\n[[#A]]\u0060open\n[[A]]\nclose\u0060> [!note] title\n章節\n<!-- ^é\n<!-- [[A]] -->\n\n## A\n[[A]][[#A]]> [!note] one\n> [!note] two\n> [!note] three\n[[A]] [[A]][^n]: [[A]]\n\n    [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\\\[[A]]> [!note] [[A]]\n- item\n\n      \\[[A]]- > [!note] title\n[^unused]: [[A]]\n  > [!note] title\n\t\u0060\u0060\u0060\n[[B|alias]]A\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"aliasa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "章節\nA\n\\[[A]]## A-2\n## A\n## A\nA\n---\nB\n\t\u0060\u0060\u0060\n  > [!note] title\nref[^n]\n  - ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "章節\nA\n\\[[A]]## A-2\n## A\n## A\nA\n---\nB\n\t\u0060\u0060\u0060\n  > [!note] title\nref[^n]\n  - ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "章節\nA\n\\[[A]]## A-2\n## A\n## A\nA\n---\nB\n\t\u0060\u0060\u0060\n  > [!note] title\nref[^n]\n  - ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"ref-n\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- > [!note] title\n<div>\n[[A]]\n</div>\nA\n> > \u0060\u0060\u0060\u0060\u0060\nB\n0<div>\n[[A]]\n</div>\n- > [!note] title\n![[A]]\t\u0060\u0060\u0060\n- - > [!note] title\n<div>\n[[A]]\n</div>\n> [!unknown] title\n[[A\\|alias]]^absent-prose ![[A#A]][[A]]\n## A-2\n## A\n## A\n[[image.png]]\u0060\u0060\u0060\n> > É\n ^a\n## A-2\n## A\n## A\n> [!note] title\n ^é\n[[A]][[B|alias]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- > [!note] title\n<div>\n[[A]]\n</div>\nA\n> > \u0060\u0060\u0060\u0060\u0060\nB\n0<div>\n[[A]]\n</div>\n- > [!note] title\n![[A]]\t\u0060\u0060\u0060\n- - > [!note] title\n<div>\n[[A]]\n</div>\n> [!unknown] title\n[[A\\|alias]]^absent-prose ![[A#A]][[A]]\n## A-2\n## A\n## A\n[[image.png]]\u0060\u0060\u0060\n> > É\n ^a\n## A-2\n## A\n## A\n> [!note] title\n ^é\n[[A]][[B|alias]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- > [!note] title\n<div>\n[[A]]\n</div>\nA\n> > \u0060\u0060\u0060\u0060\u0060\nB\n0<div>\n[[A]]\n</div>\n- > [!note] title\n![[A]]\t\u0060\u0060\u0060\n- - > [!note] title\n<div>\n[[A]]\n</div>\n> [!unknown] title\n[[A\\|alias]]^absent-prose ![[A#A]][[A]]\n## A-2\n## A\n## A\n[[image.png]]\u0060\u0060\u0060\n> > É\n ^a\n## A-2\n## A\n## A\n> [!note] title\n ^é\n[[A]][[B|alias]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- > [!note] title\n<div>\n[[A]]\n</div>\nA\n> > \u0060\u0060\u0060\u0060\u0060\nB\n0<div>\n[[A]]\n</div>\n- > [!note] title\n![[A]]\t\u0060\u0060\u0060\n- - > [!note] title\n<div>\n[[A]]\n</div>\n> [!unknown] title\n[[A\\|alias]]^absent-prose ![[A#A]][[A]]\n## A-2\n## A\n## A\n[[image.png]]\u0060\u0060\u0060\n> > É\n ^a\n## A-2\n## A\n## A\n> [!note] title\n ^é\n[[A]][[B|alias]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- [[A#^a]]![[A]]0[[#A]]É\n    \u0060\u0060\u0060\n## A-2\n## A\n## A\n\u0060[[A]]\u0060%%| a | b |\n|---|---|\n| [[A]] | ^a |\n1. \n\n[[A\nB]]\n\n[[A\\]]~~~\n  - ~~~~\n[[B|alias]][^unused]: [[A]]\n\u0060[[A]]\u0060text \\\u0060[[B|alias]]     ^é\n ^a\n## [[A|alias]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] title\nA\n===\n[[A#A]] - item\n\n      ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "0![[A#A]]  \u0060\u0060\u0060\n> [!unknown] title\n> [!note] title\n\u0060open\n[[A]]\nclose\u0060A\n===\n\u0060\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"open-a-closea\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\n    \u0060\u0060\u0060\n![[A#A]] ^a\nhttps://example.invalid/\u0060[[A]]\u0060 ^absent-prose > [!unknown] title\n> > > \\\u0060\u0060\u0060\u0060\n![[A]]\u0060\u0060\u0060 [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n%%[[A]]%%[[A\\]]A\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] [[A]]\n> [!note] [[A]]\nA\n---\n    [^unused]: [[A]]\n ^é\n# A\n> > \u0060[[A]]\u0060[[A\\|alias]]| a | b |\n|---|---|\n| [[A]] | ^a |\n  \u0060\u0060\u0060\n[[A\\]]<!--    \u0060\u0060\u0060\n\u0060\u0060> [!note] one\n> [!note] two\n> [!note] three\n   \u0060\u0060\u0060\n[[A]][[A#^a]]\u0060\u0060\u0060\u0060 go [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> - item\n\n      - [[A#^a]]  - [[#A]]章節\n> [!note] one\n> [!note] two\n> [!note] three\n<!-- [[A]] -->![[A]]A\n---\n\u0060\u0060\u0060\n\u0060\u0060\u0060 [[A]]\n ^a-2\n^absent-prose [[image.png]]## <em>A</em>\n\u0060\t\u0060\u0060\u0060\n\u0060\u0060\u0060\n- > [!note] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  \u0060\u0060\u0060\nref[^n]\n \u0060\u0060\u0060\n  > [!note] title\nref[^n]\n## A\n<div>\n[[A]]\n</div>\nhttps://example.invalid/\u0060[[A]]\u0060 ![[A]]~~~\n- [ ] [[A]]\n1. https://example.invalid/\u0060[[A]]\u0060 00> [!note] one\n> [!note] two\n> [!note] three\n## A-2\n## A\n## A\n1. ## !\n  - 章節\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  \u0060\u0060\u0060\nref[^n]\n \u0060\u0060\u0060\n  > [!note] title\nref[^n]\n## A\n<div>\n[[A]]\n</div>\nhttps://example.invalid/\u0060[[A]]\u0060 ![[A]]~~~\n- [ ] [[A]]\n1. https://example.invalid/\u0060[[A]]\u0060 00> [!note] one\n> [!note] two\n> [!note] three\n## A-2\n## A\n## A\n1. ## !\n  - 章節\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\n \\\\[[A]]| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A\\|alias]][[A#^a]]| a | b |\n|---|---|\n| [[A]] | ^a |\nA\n===\n0- 0\\[[A]][[A#^a]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-b-a-a-aliasa-a-a-b-a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060\u00601. | a | b |\n|---|---|\n| [[A]] | ^a |\n\u0060open\n[[A]]\nclose\u0060> [!note] one\n> [!note] two\n> [!note] three\n\\\u0060 ^é\nA\n---\n\t\u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 [[A\\|alias]]\u0060open\n[[A]]\nclose\u0060É\n ^a-2\n\t\u0060\u0060\u0060\n%%    https://example.invalid/\u0060[[A]]\u0060 \u0060[[A]]\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"é-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n![[A#A]][^n]: [[A]]\n\n    [[B]]\n   \u0060\u0060\u0060\n%%https://example.invalid/\u0060[[A]]\u0060 > [!note] title\n\u0060\u0060\u0060\u0060\n\u0060\u0060[[A\\|alias]]0[[image.png]]\u0060- [[A\nB]]A\n===\n<!--\u0060open\n[[A]]\nclose\u0060[[A\\]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"alias0image-png-a-b-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n\nA\n\n> [!unknown] title\nA\n===\n[[image.png]][[A#A]] ^é\n%%[[A]]%%[[A]]  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> > [[A\\]][[A#A]]> [!note] one\n> [!note] two\n> [!note] three\n[[A]][[A]]text   \u0060\u0060\u0060\nÉ\n[[A]][[#A]][[image.png]]  - \u0060\u0060\u0060 [[A]]\nA\n===\n  - [[A\\|alias]]- > [!note] title\n<!--%%[[A]]## A\n\n\n\t- [ ] [[A]]\n<!--## A-2\n## A\n## A\n\u0060\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"aatext-é-a-aimage-png-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\\\[[A]]> [!note] [[A]]\n0text  ^a-2\n[[A\\|alias]]> [!unknown] title\n        \u0060\u0060\u0060\n[[A]]\u0060\u0060\u0060 [[A]]\n[[A#^a]]A\n---\n ^A\n章節\n  \u0060\u0060\u0060\n\\\u0060%%1. ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-note-a-0text-a-2-alias-unknown-title-a-a-a-aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] --> ^a-2\n\\[[A]]<!-- [[A]] -->A\n===\n[[#A]] ^é\n> [!unknown] title\n1.    \u0060\u0060\u0060\n章節\n\t\u0060\u0060\u0060\n ^é\n%%- [ ] [[A]]\n  \u0060\u0060\u0060\n![[image.png]]<!-- [[A]] -->## A\n ^a\n\\\\[[A]]- [x] [[B]]\n\u0060A\n---\n<!--https://example.invalid/\u0060[[A]]\u0060 > [!unknown] title\n\t\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "É\n\u0060\u0060> > ## A-2\n## A\n## A\n\u0060open\n[[A]]\nclose\u0060[[image.png]][[#A]][[A#A]]text \u0060open\n[[A]]\nclose\u0060# A\nA\n===\n\u0060\u0060 ^a-2\n~~~~\n# A\n\\\u0060[[A\nB]]- [x] [[B]]\n[[A#^a]]^absent-prose \u0060\u0060\u0060\u0060 go [[A]]\n[[B|alias]]章節\n\t     ^é\n ^A\n\u0060\u0060\u0060\u0060\n\u0060\u0060 ^a\n\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"open-a-close-image-png-a-a-a-text-open-a-close-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^é\n\\[[A]]https://example.invalid/\u0060[[A]]\u0060 \u0060[[B|alias]]| a | b |\n|---|---|\n| [[A]] | ^a |\n  - ~~~\n## A-2\n## A\n## A\n ![[A]]%%[[A]]%%## A\n## A\nÉ\n> [!note] one\n> [!note] two\n> [!note] three\n![[A]]0%%## A\n> [!note] title\n## A-2\n## A\n## A\n\\[[A]]![[image.png]]<!-- [[A]] --> ^a\n \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^é\n\\[[A]]https://example.invalid/\u0060[[A]]\u0060 \u0060[[B|alias]]| a | b |\n|---|---|\n| [[A]] | ^a |\n  - ~~~\n## A-2\n## A\n## A\n ![[A]]%%[[A]]%%## A\n## A\nÉ\n> [!note] one\n> [!note] two\n> [!note] three\n![[A]]0%%## A\n> [!note] title\n## A-2\n## A\n## A\n\\[[A]]![[image.png]]<!-- [[A]] --> ^a\n \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[^n]: [[A]]\n\n    [[B]]\n> [!note] [[A]]\n> [[A\nB]]A\n===\nÉ\n%%## [[A|alias]]\n\u0060\u0060[[A\nB]] ^é\n\t\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"b-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^a\n[[A\\|alias]]![[A]]<!--[^n]: [[A]]\n\n    [[B]]\nA\n===\n> [[A]] [[A]]## !\n^absent-prose > [!unknown] title\n\n\nref[^n]\n## A-2\n## A\n## A\n  \u0060\u0060\u0060\n ^é\n> > - > [!note] title\n<!-- [[A]] -->![[A#A]]A\n---\n ^A\n~~~\n\\\\[[A]]<!-- [[A]] -->[[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n## A\n> [!unknown] title\n![[image.png]]<!-- [[A]] -->\u0060open\n[[A]]\nclose\u0060[[A#^a]]A\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n## A\n> [!unknown] title\n![[image.png]]<!-- [[A]] -->\u0060open\n[[A]]\nclose\u0060[[A#^a]]A\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-png-open-a-closea-aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\nA\n---\n ^é\n\n\n- item\n\n      > [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060\u0060\n1.    \u0060\u0060\u0060\n \u0060\u0060\u0060\n<!--\n[[A]]\n-->  -  ^a-2\n^absent-prose > [!note] [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n# A\n![[A#A]][[A]] [[A]]-->- [ ] [[A]]\n\u0060    ![[image.png]][[A]]É\n> [!note] [[A]]\n> [!unknown] title\n> [!note] [[A]]\n~~~\n## [[A|alias]]\ntext ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]]text %%[[A]]%%<!--\n[[A]]\n--> \u0060\u0060\u0060\n   \u0060\u0060\u0060\n-->[^unused]: [[A]]\n- item\n\n      \u0060open\n[[A]]\nclose\u0060## <em>A</em>\n## A\n> [!note] title\n## !\n-->\u0060open\n[[A]]\nclose\u0060\n\nA\n===\n> > \u0060\u0060\u0060 [[A]]\n[[A#A]]## !\n<!-- [[A]] -->https://example.invalid/\u0060[[A]]\u0060 - - item\n\n      <!--## [[A|alias]]\n   \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->## A\n[[A]] ^a-2\n## A\n## A\nÉ\n- [x] [[B]]\n[[B|alias]]0  - >  ^a\n-   - A\n---\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[image.png]][[A\\|alias]][[A\\|alias]]-->![[image.png]]| a | b |\n|---|---|\n| [[A]] | ^a |\n## <em>A</em>\n<!-- [[A]] -->## <em>A</em>\ntext ## A-2\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"em-a-em-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] [[A]]\n## A-2\n## A\n## A\n> [!note] one\n> [!note] two\n> [!note] three\n## A\n> [!unknown] title\nA\n===\n## <em>A</em>\n[[#A]]章節\n0- [x] [[B]]\n![[image.png]][[A#^a]]1. text    \u0060\u0060\u0060\n- [x] [[B]]\n[[B|alias]]<!--",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] [[A]]\n## A-2\n## A\n## A\n> [!note] one\n> [!note] two\n> [!note] three\n## A\n> [!unknown] title\nA\n===\n## <em>A</em>\n[[#A]]章節\n0- [x] [[B]]\n![[image.png]][[A#^a]]1. text    \u0060\u0060\u0060\n- [x] [[B]]\n[[B|alias]]<!--",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] [[A]]\n## A-2\n## A\n## A\n> [!note] one\n> [!note] two\n> [!note] three\n## A\n> [!unknown] title\nA\n===\n## <em>A</em>\n[[#A]]章節\n0- [x] [[B]]\n![[image.png]][[A#^a]]1. text    \u0060\u0060\u0060\n- [x] [[B]]\n[[B|alias]]<!--",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- item\n\n      \\[[A]][^unused]: [[A]]\n%%[[A]]%%[[A]][[A]]> >  ^a-2\n<!-- [[A]] -->## A-2\n## A\n## A\n^absent-prose  ^é\n[[image.png]]章節\nÉ\n## !\n## [[A|alias]]\n<div>\n[[A]]\n</div>\n\u0060\u0060\u0060\n    A\n    > [!note] title\n## !\nA\n> > B\n%%[[#A]]~~~~\n   \u0060\u0060\u0060\n ^é\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] [[A]]\n0  \u0060\u0060\u0060\n- [ ] [[A]]\n- [x] [[B]]\n\\\u0060## [[A|alias]]\n[[A#A]]> [!note] title\n## [[A|alias]]\n\u0060\u0060# A\n> [!note] [[A]]\n> [!note] [[A]]\nA\n---\n  - É\n\t\u0060\u0060\u0060\n[^unused]: [[A]]\nÉ\nA\nÉ\n\u0060[[A]]\u00600\t[^n]: [[A]]\n\n    [[B]]\n  \u0060\u0060\u0060\n1. [[A]]![[A#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n> [!note] one\n> [!note] two\n> [!note] three\n![[image.png]]B\n<div>\n[[A]]\n</div>\n[[B|alias]]A\n[[A\\|alias]]章節\n> [!note] [[A]]\n## A\n## A\n[[A\\]]\n\n\t%%[[B|alias]]  > [!note] title\n> [!unknown] title\n![[image.png]]\n~~~~\n## [[A|alias]]\n- [ ] [[A]]\n ^a\n  \u0060\u0060\u0060\n[[B|alias]]A\n---\n# A\n ^A\n\t\u0060\u0060\u0060\n\u0060\u0060\u0060\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n> [!note] one\n> [!note] two\n> [!note] three\n![[image.png]]B\n<div>\n[[A]]\n</div>\n[[B|alias]]A\n[[A\\|alias]]章節\n> [!note] [[A]]\n## A\n## A\n[[A\\]]\n\n\t%%[[B|alias]]  > [!note] title\n> [!unknown] title\n![[image.png]]\n~~~~\n## [[A|alias]]\n- [ ] [[A]]\n ^a\n  \u0060\u0060\u0060\n[[B|alias]]A\n---\n# A\n ^A\n\t\u0060\u0060\u0060\n\u0060\u0060\u0060\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^a\nB\n<!-- [[A]] -->## A\n## A\n## A\n> É\n\u0060\u0060\u0060 [[A]]\n    \u0060\u0060\u0060\n    É\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A]]## <em>A</em>\n[[A\\]][[A\\]]\u0060open\n[[A]]\nclose\u0060[^unused]: [[A]]\n  -   ^a-2\n![[A#A]]\t\u0060\u0060\u0060\nÉ\n\t\u0060\u0060\u0060\n\u0060\u0060  -   - \n> [!note] title\n![[image.png]]-->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "-     ## A\n## A\n> [!note] [[A]]\n\\[[A]]\u0060[[A]]\\[[A]]\u0060[[A]]\u0060> [!unknown] title\n\\[[A]][[A\\|alias]]    \u0060\u0060\u0060\n0 ^a-2\nA\n===\n[^n]: [[A]]\n\n    [[B]]\n   \u0060\u0060\u0060\n章節\n<!--## <em>A</em>\nref[^n]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-a-a-unknown-title-a-alias-0-a-2-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> > > [!note] one\n> [!note] two\n> [!note] three\nÉ\n[[A\\]]A\n---\n    \u0060\u0060\u0060\nB\n<div>\n[[A]]\n</div>\n ^A\nref[^n]\n\n\n\u0060\u0060\u0060\u0060 go [[A]]\n ^é\n- > [!note] title\n- item\n\n      - > [!note] title\nhttps://example.invalid/\u0060[[A]]\u0060 ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"é-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n%%\t> > - [x] [[B]]\n> ## A\n## A\n ^é\n[[A]] [[A]]## A-2\n## A\n## A\n~~~~\n## A-2\n## A\n## A\n\n[[A]]## A-2\n## A\n## A\n    \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 0## [[A|alias]]\n[[A\nB]]- [x] [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\\u00601. <!-- [[A]] -->\u0060\u0060\u0060\n## A-2\n## A\n## A\n\u0060[[#A]]\\\u0060[[A]] [[A]][[B|alias]]\u0060\u0060\u0060[[A]]\u0060[[A\nB]]A\n===\n\\[[A]]## A-2\n## A\n## A\n <!-- [[A]] -->A\n===\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-6\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\\u00601. <!-- [[A]] -->\u0060\u0060\u0060\n## A-2\n## A\n## A\n\u0060[[#A]]\\\u0060[[A]] [[A]][[B|alias]]\u0060\u0060\u0060[[A]]\u0060[[A\nB]]A\n===\n\\[[A]]## A-2\n## A\n## A\n <!-- [[A]] -->A\n===\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-aalias-a-a-b-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "^absent-prose 0A\n---\nB\n ^a\n\u0060\u0060- > [!note] title\n0\u0060[[A]]\u0060%%[[A]]%%\n  > [!note] title\nA\n---\n ^A\n  - ## [[A|alias]]\nA\n---\n\u0060\u0060## !\nhttps://example.invalid/\u0060[[A]]\u0060 \n <div>\n[[A]]\n</div>\n1. ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "^absent-prose 0A\n---\nB\n ^a\n\u0060\u0060- > [!note] title\n0\u0060[[A]]\u0060%%[[A]]%%\n  > [!note] title\nA\n---\n ^A\n  - ## [[A|alias]]\nA\n---\n\u0060\u0060## !\nhttps://example.invalid/\u0060[[A]]\u0060 \n <div>\n[[A]]\n</div>\n1. ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"absent-prose-0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  > [!note] title\n[[B|alias]][[A]] [[A]]  - - > [!note] title\n<!-- [[A]] -->A\n---\n\u0060\u0060- [x] [[B]]\n    \u0060open\n[[A]]\nclose\u0060章節\n## !\n[[B|alias]][[A#^a]]- [[A]] [[A]][[A\\|alias]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"aliasa-a-note-title-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060open\n[[A]]\nclose\u0060  - 0[[A#A]]ref[^n]\nA\n===\n[[A\\|alias]]1. [[A#^a]]\u0060\u0060 \u0060\u0060\u0060\n> [!note] title\n[[A\\]] \u0060\u0060\u0060\n[^n]: [[A]]\n\n    [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"open-a-close-0a-aref1-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[#A]] ^é\n<!--\n[[A]]\n-->## A\n## A\n[[A\\]]\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n---\n## A-2\n## A\n## A\n## [[A|alias]]\n- [x] [[B]]\n- [x] [[B]]\n[[A\nB]] ^a\n> [!unknown] title\n\u0060\u0060~~~~\n\\\\[[A]]0 A\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n---\n## A-2\n## A\n## A\n## [[A|alias]]\n- [x] [[B]]\n- [x] [[B]]\n[[A\nB]] ^a\n> [!unknown] title\n\u0060\u0060~~~~\n\\\\[[A]]0 A\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n---\n## A-2\n## A\n## A\n## [[A|alias]]\n- [x] [[B]]\n- [x] [[B]]\n[[A\nB]] ^a\n> [!unknown] title\n\u0060\u0060~~~~\n\\\\[[A]]0 A\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a0-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]]É\n> [!unknown] title\n\u0060\u0060text 1. A\n===\n## A\n## A\nA\n---\n[[A\\|alias]]https://example.invalid/\u0060[[A]]\u0060 ![[A#A]]~~~\n1. > [!note] [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n\u0060<!--\n[[A]]\n-->- item\n\n       ^A\n ^é\n[[A#A]] ^é\n~~~\n  - - item\n\n      text ![[A#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]]É\n> [!unknown] title\n\u0060\u0060text 1. A\n===\n## A\n## A\nA\n---\n[[A\\|alias]]https://example.invalid/\u0060[[A]]\u0060 ![[A#A]]~~~\n1. > [!note] [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n\u0060<!--\n[[A]]\n-->- item\n\n       ^A\n ^é\n[[A#A]] ^é\n~~~\n  - - item\n\n      text ![[A#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]]É\n> [!unknown] title\n\u0060\u0060text 1. A\n===\n## A\n## A\nA\n---\n[[A\\|alias]]https://example.invalid/\u0060[[A]]\u0060 ![[A#A]]~~~\n1. > [!note] [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n\u0060<!--\n[[A]]\n-->- item\n\n       ^A\n ^é\n[[A#A]] ^é\n~~~\n  - - item\n\n      text ![[A#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"text-1-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\nref[^n]\n\n\n ^a\n## <em>A</em>\n\t## A\n0[[A]] [[A]][[A#^a]] ^a-2\nA\n---\n    ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0a-aa-a-a-2-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "-->0\u0060\u0060\u0060 [[A]]\n> [!unknown] title\nA\n---\n[[A#^a]]> [!note] one\n> [!note] two\n> [!note] three\n ^é\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060\u0060\u0060\n<div>\n[[A]]\n</div>\n - [ ] [[A]]\n  \u0060\u0060\u0060\n    \u0060\u0060\u0060\u0060 go [[A]]\n- item\n\n      \\[[A]][[A\nB]] ^é\n## <em>A</em>\n> <!--\n[[A]]\n--><!--\n[[A]]\n-->\n[[image.png]]![[image.png]]A\n===\n\n- [ ] [[A]]\n- item\n\n      \\\\[[A]]0\u0060\u0060\u0060 [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-png-image-png-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]][[A]] [[A]]\n\n<!--\n[[A]]\n-->A\n===\n## A\n## A\nA\n[[#A]]> [!note] title\nA\n0  > [!note] title\n ^é\n[[A]] [[A]]> [!note] one\n> [!note] two\n> [!note] three\n## [[A|alias]]\n[[A]]\u0060\u0060%%[[A]]%% ^é\n章節\n~~~\n<!-- ^A\n[[A#^a]] ^a-2\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]][[A]] [[A]]\n\n<!--\n[[A]]\n-->A\n===\n## A\n## A\nA\n[[#A]]> [!note] title\nA\n0  > [!note] title\n ^é\n[[A]] [[A]]> [!note] one\n> [!note] two\n> [!note] three\n## [[A|alias]]\n[[A]]\u0060\u0060%%[[A]]%% ^é\n章節\n~~~\n<!-- ^A\n[[A#^a]] ^a-2\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n\\\u0060# A\n[[image.png]]--># A\nhttps://example.invalid/\u0060[[A]]\u0060 0A\n---\n> [!note] title\n![[image.png]]%%[[A]]%%\u0060\u0060\u0060 [[A]]\n## <em>A</em>\n  > [!note] title\n[[#A]]É\nA\n---\nhttps://example.invalid/\u0060[[A]]\u0060 ## <em>A</em>\n[^unused]: [[A]]\n- item\n\n      \t\u0060\u0060\u0060\n ^a\n\u0060\u0060\u0060\n[[B|alias]]\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-image-png-a-https-example-invalid-a-0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n\\\u0060# A\n[[image.png]]--># A\nhttps://example.invalid/\u0060[[A]]\u0060 0A\n---\n> [!note] title\n![[image.png]]%%[[A]]%%\u0060\u0060\u0060 [[A]]\n## <em>A</em>\n  > [!note] title\n[[#A]]É\nA\n---\nhttps://example.invalid/\u0060[[A]]\u0060 ## <em>A</em>\n[^unused]: [[A]]\n- item\n\n      \t\u0060\u0060\u0060\n ^a\n\u0060\u0060\u0060\n[[B|alias]]\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"aé-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\\|alias]]> [!note] [[A]]\n## [[A|alias]]\n\n-->## <em>A</em>\n0![[image.png]]\t> [!note] one\n> [!note] two\n> [!note] three\nA\n---\n> [!note] title\n[[A\nB]][^n]: [[A]]\n\n    [[B]]\n ^A\n> > https://example.invalid/\u0060[[A]]\u0060  ^a-2\n%%## [[A|alias]]\nhttps://example.invalid/\u0060[[A]]\u0060 ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n[[B|alias]]É\n## A\n-->%%![[image.png]]É\n## A\ntext > ![[image.png]]<!-- [[A]] -->## A\n![[image.png]]A\n---\n## <em>A</em>\nB\n  > [!note] title\n> [!note] one\n> [!note] two\n> [!note] three\n章節\n ^é\n> \t\u0060\u0060\u0060\n\\\\[[A]]A\n===\n\t\u0060\u0060\u0060\n## A\n## A\n\t\u0060\u0060\u0060\n\\\\[[A]]## A-2\n## A\n## A\n\u0060\u0060\u0060\u0060 go [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%[[A]]%%1. # A\n\\\u0060| a | b |\n|---|---|\n| [[A]] | ^a |\n![[A]] \n## !\n-->~~~\n[[A]] [[A]]text \u0060\u0060\u0060 [[A]]\n[[A#A]][[A\\]]\u0060[[A]]\u00601.     \u0060\u0060\u0060\n[^unused]: [[A]]\n\u0060[[A]]\u0060- item\n\n      A\n---\n## A\n## A\n[[A#A]]- item\n\n      [[A\\|alias]]\t ^a\n\t",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%[[A]]%%1. # A\n\\\u0060| a | b |\n|---|---|\n| [[A]] | ^a |\n![[A]] \n## !\n-->~~~\n[[A]] [[A]]text \u0060\u0060\u0060 [[A]]\n[[A#A]][[A\\]]\u0060[[A]]\u00601.     \u0060\u0060\u0060\n[^unused]: [[A]]\n\u0060[[A]]\u0060- item\n\n      A\n---\n## A\n## A\n[[A#A]]- item\n\n      [[A\\|alias]]\t ^a\n\t",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] title\n<!--\n[[A]]\n-->## A\n## A\n\u0060[[A]]\u0060-->\\\u0060   \u0060\u0060\u0060\n\u0060\u0060 ^a-2\n   \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\nB]] \u0060\u0060\u0060\n## A-2\n## A\n## A\n\\\u0060\u0060\u0060\u0060 [[A]]\n> [!note] title\n1. É\n<div>\n[[A]]\n</div>\n[[A]] [[A]]\\\\[[A]]0 ^a\n> [!unknown] title\n## A\n\u0060\\\u0060\u0060\u0060\u0060 [[A]]\n## <em>A</em>\n\u0060\u0060[^unused]: [[A]]\n- item\n\n      <!--[[#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\nB]] \u0060\u0060\u0060\n## A-2\n## A\n## A\n\\\u0060\u0060\u0060\u0060 [[A]]\n> [!note] title\n1. É\n<div>\n[[A]]\n</div>\n[[A]] [[A]]\\\\[[A]]0 ^a\n> [!unknown] title\n## A\n\u0060\\\u0060\u0060\u0060\u0060 [[A]]\n## <em>A</em>\n\u0060\u0060[^unused]: [[A]]\n- item\n\n      <!--[[#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "章節\n0%%[[A]]%%\\\u0060## [[A|alias]]\nA\n---\n## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\t- item\n\n      ## <em>A</em>\n> [!note] [[A]]\n ^a\n![[image.png]]1. A\n---\n  - A\nA\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-image-png-1-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "    \u0060\u0060\u0060\n%%[[A]]%%## A\n[[#A]]- [ ] [[A]]\n%%[[A]]%%\u0060    \u0060\u0060\u0060\n<!-- [[A]] -->text - [ ] [[A]]\nÉ\n## A\n## A\n<div>\n[[A]]\n</div>\n![[A]]\\\u0060\n- > [!note] title\n~~~~\n\t    \u0060\u0060\u0060\n\u0060open\n[[A]]\nclose\u0060> [!unknown] title\nhttps://example.invalid/\u0060[[A]]\u0060 ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "    \u0060\u0060\u0060\n%%[[A]]%%## A\n[[#A]]- [ ] [[A]]\n%%[[A]]%%\u0060    \u0060\u0060\u0060\n<!-- [[A]] -->text - [ ] [[A]]\nÉ\n## A\n## A\n<div>\n[[A]]\n</div>\n![[A]]\\\u0060\n- > [!note] title\n~~~~\n\t    \u0060\u0060\u0060\n\u0060open\n[[A]]\nclose\u0060> [!unknown] title\nhttps://example.invalid/\u0060[[A]]\u0060 ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[A#A]][[A\\|alias]]\n\n\n[^n]: [[A]]\n\n    [[B]]\nhttps://example.invalid/\u0060[[A]]\u0060 - [x] [[B]]\n<!-- [[A]] -->ref[^n]\n> [!note] title\nA\n---\n^absent-prose   - \u0060\u0060\u0060 [[A]]\n## !\n    text > [!note] [[A]]\n- [ ] [[A]]\nÉ\n\u0060open\n[[A]]\nclose\u0060<div>\n[[A]]\n</div>\n> [!note] [[A]]\n-->## A\nÉ\n[[A\\]][^n]: [[A]]\n\n    [[B]]\n\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- [x] [[B]]\n[[A\\|alias]]>  ^é\n[[A]]\u0060[[A]]\u0060 ^é\n# A\n> [!note] title\n## A\n[[A\nB]]<!--## A-2\n## A\n## A\n\u0060[[A#A]]\n\n![[A#A]]- > [!note] title\n![[A]][[A#A]]  \u0060\u0060\u0060\n> [[B|alias]]%%[[A]]%%",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%[[A]]%%## A\n## A\n~~~\n\t\u0060\u0060\u0060\n%%[[#A]][[A#^a]]\n\n[[#A]]- > [!note] title\n## A\n## A\n- item\n\n      \u0060\u0060\u0060\u0060\n\\\u0060\\[[A]]\n\n[[A\\]]-->^absent-prose \u0060\u0060\u0060\u0060\n^absent-prose https://example.invalid/\u0060[[A]]\u0060 [[A\\|alias]]\u0060\u0060\u0060 [[A]]\n0",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[B|alias]] \u0060\u0060\u0060\n![[A#A]] \u0060\u0060\u0060\n0A\n===\n[[A#^a]]> [!note] [[A]]\n[^unused]: [[A]]\n## A\n## A\n1. ![[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"alias-a-a-0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "É\n[[A\\]]<div>\n[[A]]\n</div>\nA\n---\n[[A#A]]B\n\t## A-2\n## A\n## A\n> [!note] one\n> [!note] two\n> [!note] three\n  - - [ ] [[A]]\n\u0060[[A]]\u0060É\n0| a | b |\n|---|---|\n| [[A]] | ^a |\n## A-2\n## A\n## A\n\u0060\u0060\u0060\n- [x] [[B]]\n[[A#^a]]![[A#A]]\n> [!note] [[A]]\n![[image.png]]# A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## [[A|alias]]\n[[B|alias]] \u0060\u0060\u0060\n> ## A-2\n## A\n## A\n## A-2\n## A\n## A\n\\\\[[A]]- [ ] [[A]]\n# A\nref[^n]\n<!-- [[A]] -->%%[[A]]%%# A\n[[image.png]]<div>\n[[A]]\n</div>\n<!-- [[A]] --> ^é\n- [x] [[B]]\n## A\n\u0060\u0060\u0060\n ^é\n\n\n## A\n## A\n章節\n- item\n\n      > [!unknown] title\n    \u0060\u0060\u0060\n  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## [[A|alias]]\n[[B|alias]] \u0060\u0060\u0060\n> ## A-2\n## A\n## A\n## A-2\n## A\n## A\n\\\\[[A]]- [ ] [[A]]\n# A\nref[^n]\n<!-- [[A]] -->%%[[A]]%%# A\n[[image.png]]<div>\n[[A]]\n</div>\n<!-- [[A]] --> ^é\n- [x] [[B]]\n## A\n\u0060\u0060\u0060\n ^é\n\n\n## A\n## A\n章節\n- item\n\n      > [!unknown] title\n    \u0060\u0060\u0060\n  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## [[A|alias]]\n[[B|alias]] \u0060\u0060\u0060\n> ## A-2\n## A\n## A\n## A-2\n## A\n## A\n\\\\[[A]]- [ ] [[A]]\n# A\nref[^n]\n<!-- [[A]] -->%%[[A]]%%# A\n[[image.png]]<div>\n[[A]]\n</div>\n<!-- [[A]] --> ^é\n- [x] [[B]]\n## A\n\u0060\u0060\u0060\n ^é\n\n\n## A\n## A\n章節\n- item\n\n      > [!unknown] title\n    \u0060\u0060\u0060\n  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## [[A|alias]]\n[[B|alias]] \u0060\u0060\u0060\n> ## A-2\n## A\n## A\n## A-2\n## A\n## A\n\\\\[[A]]- [ ] [[A]]\n# A\nref[^n]\n<!-- [[A]] -->%%[[A]]%%# A\n[[image.png]]<div>\n[[A]]\n</div>\n<!-- [[A]] --> ^é\n- [x] [[B]]\n## A\n\u0060\u0060\u0060\n ^é\n\n\n## A\n## A\n章節\n- item\n\n      > [!unknown] title\n    \u0060\u0060\u0060\n  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## [[A|alias]]\n[[B|alias]] \u0060\u0060\u0060\n> ## A-2\n## A\n## A\n## A-2\n## A\n## A\n\\\\[[A]]- [ ] [[A]]\n# A\nref[^n]\n<!-- [[A]] -->%%[[A]]%%# A\n[[image.png]]<div>\n[[A]]\n</div>\n<!-- [[A]] --> ^é\n- [x] [[B]]\n## A\n\u0060\u0060\u0060\n ^é\n\n\n## A\n## A\n章節\n- item\n\n      > [!unknown] title\n    \u0060\u0060\u0060\n  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-6\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## [[A|alias]]\n[[B|alias]] \u0060\u0060\u0060\n> ## A-2\n## A\n## A\n## A-2\n## A\n## A\n\\\\[[A]]- [ ] [[A]]\n# A\nref[^n]\n<!-- [[A]] -->%%[[A]]%%# A\n[[image.png]]<div>\n[[A]]\n</div>\n<!-- [[A]] --> ^é\n- [x] [[B]]\n## A\n\u0060\u0060\u0060\n ^é\n\n\n## A\n## A\n章節\n- item\n\n      > [!unknown] title\n    \u0060\u0060\u0060\n  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-7\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## [[A|alias]]\n[[B|alias]] \u0060\u0060\u0060\n> ## A-2\n## A\n## A\n## A-2\n## A\n## A\n\\\\[[A]]- [ ] [[A]]\n# A\nref[^n]\n<!-- [[A]] -->%%[[A]]%%# A\n[[image.png]]<div>\n[[A]]\n</div>\n<!-- [[A]] --> ^é\n- [x] [[B]]\n## A\n\u0060\u0060\u0060\n ^é\n\n\n## A\n## A\n章節\n- item\n\n      > [!unknown] title\n    \u0060\u0060\u0060\n  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-8\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## [[A|alias]]\n[[B|alias]] \u0060\u0060\u0060\n> ## A-2\n## A\n## A\n## A-2\n## A\n## A\n\\\\[[A]]- [ ] [[A]]\n# A\nref[^n]\n<!-- [[A]] -->%%[[A]]%%# A\n[[image.png]]<div>\n[[A]]\n</div>\n<!-- [[A]] --> ^é\n- [x] [[B]]\n## A\n\u0060\u0060\u0060\n ^é\n\n\n## A\n## A\n章節\n- item\n\n      > [!unknown] title\n    \u0060\u0060\u0060\n  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-9\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "https://example.invalid/\u0060[[A]]\u0060 \t ^a-2\n> > \t~~~~\n>  ^é\n> [!note] title\n\\\u0060[^unused]: [[A]]\ntext ## A\n## A\nA\n---\n<!-- [[A]] -->\u0060[[A]]\u0060 ^é\n0- [x] [[B]]\nref[^n]\n<!--\n[[A]]\n-->## <em>A</em>\n<div>\n[[A]]\n</div>\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> > - item\n\n      \\\\[[A]]<!-- [[A]] -->\u0060\u0060\t\u0060\u0060\u0060\n\\\u0060  - <div>\n[[A]]\n</div>\n[^n]: [[A]]\n\n    [[B]]\n> [!note] one\n> [!note] two\n> [!note] three\n[[A]] [[A]] A\n---\n ^A\ntext [[A]]- [ ] [[A]]\n\u0060\u0060[[A#A]]<div>\n[[A]]\n</div>\n章節\n\u0060\u0060\u0060\u0060\n0<!--\t\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%[[A]]%%## A-2\n## A\n## A\n ^a-2\n\u0060[[A\\]]\\[[A]] \u0060\u0060\u0060\n- item\n\n      [[A\nB]]A\n---\n<!-- [[A]] -->[[B|alias]]![[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "É\nÉ\n\\[[A]]> [!note] one\n> [!note] two\n> [!note] three\nA\n===\ntext A\n===\n  \u0060\u0060\u0060\n[[A]]É\n-->[[A]] [[A]]> [!unknown] title\n> [!note] title\n<!--\n[[A]]\n-->B\n[[#A]]> [!note] [[A]]\n\u0060\u0060\u0060\u0060\n\t\u0060\u0060> >  ^a-2\n0   \u0060\u0060\u0060\n[[A#A]]A\nB\n\tÉ\n## A\n## A\n# A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "É\nÉ\n\\[[A]]> [!note] one\n> [!note] two\n> [!note] three\nA\n===\ntext A\n===\n  \u0060\u0060\u0060\n[[A]]É\n-->[[A]] [[A]]> [!unknown] title\n> [!note] title\n<!--\n[[A]]\n-->B\n[[#A]]> [!note] [[A]]\n\u0060\u0060\u0060\u0060\n\t\u0060\u0060> >  ^a-2\n0   \u0060\u0060\u0060\n[[A#A]]A\nB\n\tÉ\n## A\n## A\n# A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "É\nÉ\n\\[[A]]> [!note] one\n> [!note] two\n> [!note] three\nA\n===\ntext A\n===\n  \u0060\u0060\u0060\n[[A]]É\n-->[[A]] [[A]]> [!unknown] title\n> [!note] title\n<!--\n[[A]]\n-->B\n[[#A]]> [!note] [[A]]\n\u0060\u0060\u0060\u0060\n\t\u0060\u0060> >  ^a-2\n0   \u0060\u0060\u0060\n[[A#A]]A\nB\n\tÉ\n## A\n## A\n# A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[^unused]: [[A]]\n[^unused]: [[A]]\n\n^absent-prose <!--\n[[A]]\n-->## A\n\u0060open\n[[A]]\nclose\u0060章節\n\\\u0060 ^A\n- ![[A]]## A\n## A\n![[A#A]]  - [^unused]: [[A]]\n ^A\n\u0060\u0060\u0060 [[A]]\n^absent-prose %%\\\u0060 ^é\n  > [!note] title\n0~~~\nA\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  > [!note] title\nref[^n]\n[[A\nB]][[B|alias]]^absent-prose \u0060open\n[[A]]\nclose\u0060# A\n[[A]]## <em>A</em>\n\t![[A]]A\n---\n%%- item\n\n      [[A\\|alias]]章節\n\t  - [[#A]]\u0060[[A]]\u0060## <em>A</em>\n> [!note] title\n> [!note] title\n    \u0060\u0060\u0060\n-->\n[[image.png]]<!--\n[[A]]\n-->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"ref-n-a-b-alias-absent-prose-open-a-close-a-a-em-a-em-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!--\n[[A]]\n-->    [[A#A]]\u0060\u0060 ^A\n ^é\nhttps://example.invalid/\u0060[[A]]\u0060 > [!note] title\n[[image.png]]> [!note] one\n> [!note] two\n> [!note] three\n[[A\\]]![[A]]\t## A\n## A\n\t[[A\\|alias]]<!-- [[A]] -->https://example.invalid/\u0060[[A]]\u0060 - > [!note] title\n\n\n## <em>A</em>\n<!--\n[[A]]\n-->## A\n%%[[A]]%%[[B|alias]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\t\u0060\u0060\u0060\u0060\n[[image.png]]\u0060\u0060\u0060[[A]]\u0060[[A\\]]章節\nÉ\n## <em>A</em>\n\t1. [[A\\]]## A\n## A\n> [!unknown] title\n章節\nA\n---\n- > [!note] title\n \u0060\u0060\u0060\n\n\nA\n> %%0> [!note] [[A]]\n0A\n## A-2\n## A\n## A\n> > ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"章節-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  - 0> [!note] [[A]]\n[[A\\|alias]]~~~~\n[[A#^a]]%%[[A]]%%## A-2\n## A\n## A\n[[A]] | a | b |\n|---|---|\n| [[A]] | ^a |\n![[A#A]][[B|alias]]- [x] [[B]]\n[[#A]]<!-- [[A]] -->\u0060\u0060\u0060\u0060 go [[A]]\n%%| a | b |\n|---|---|\n| [[A]] | ^a |\n## <em>A</em>\ntext \u0060[[A]]\u0060- > [!note] title\n\u0060\u0060\u0060\u0060 go [[A]]\n\u0060\u0060\u0060\n ^A\n\u0060\u0060\u0060\u0060 go [[A]]\n\\\\[[A]] ^a\n  - \t<!--",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%## [[A|alias]]\n- > [!note] title\ntext  \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 [[A\\|alias]] ^é\n## A\n## A\n[[A#A]]\u0060open\n[[A]]\nclose\u0060%%## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n\u0060[[A]]\u0060É\n0\u0060\u0060\u0060\n章節\n-->\\\\[[A]]É\n## A\n## A\nÉ\n[[A#A]]   \u0060\u0060\u0060\n[[A]] [[A]]\u0060\u0060\u0060\u0060 go [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%## [[A|alias]]\n- > [!note] title\ntext  \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 [[A\\|alias]] ^é\n## A\n## A\n[[A#A]]\u0060open\n[[A]]\nclose\u0060%%## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n\u0060[[A]]\u0060É\n0\u0060\u0060\u0060\n章節\n-->\\\\[[A]]É\n## A\n## A\nÉ\n[[A#A]]   \u0060\u0060\u0060\n[[A]] [[A]]\u0060\u0060\u0060\u0060 go [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%## [[A|alias]]\n- > [!note] title\ntext  \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 [[A\\|alias]] ^é\n## A\n## A\n[[A#A]]\u0060open\n[[A]]\nclose\u0060%%## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n\u0060[[A]]\u0060É\n0\u0060\u0060\u0060\n章節\n-->\\\\[[A]]É\n## A\n## A\nÉ\n[[A#A]]   \u0060\u0060\u0060\n[[A]] [[A]]\u0060\u0060\u0060\u0060 go [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060A\n===\n<!--\n[[A]]\n--># A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\nA\n---\nA\n===\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[#A]]É\n%%[[A\\|alias]]\n\n\u0060# A\n> [!note] title\n> > É\n# A\n> [!note] [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n%%\\\u0060## A-2\n## A\n## A\n## !\n\u0060\u0060\u0060\u0060 go [[A]]\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[A]]\u0060\u0060^absent-prose A\n![[A#A]][[image.png]]## A\n![[A]]A\n===\n[[A\nB]]![[image.png]][[A\nB]]> [!note] title\nref[^n]\n0A\n===\n ^a\n> [!unknown] title\n## [[A|alias]]\n ^a\n[[B|alias]]\\\u0060[^n]: [[A]]\n\n    [[B]]\n ^é\n0[[A#^a]]> [!note] [[A]]\n\u0060\u0060\u0060 [[A]]\n%% ^é\n[[#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-absent-prose-a-a-a-image-png-a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[A]]\u0060\u0060^absent-prose A\n![[A#A]][[image.png]]## A\n![[A]]A\n===\n[[A\nB]]![[image.png]][[A\nB]]> [!note] title\nref[^n]\n0A\n===\n ^a\n> [!unknown] title\n## [[A|alias]]\n ^a\n[[B|alias]]\\\u0060[^n]: [[A]]\n\n    [[B]]\n ^é\n0[[A#^a]]> [!note] [[A]]\n\u0060\u0060\u0060 [[A]]\n%% ^é\n[[#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-b-image-png-a-b-note-title-ref-n-0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!unknown] title\n\u0060 \\\u0060[[A]]A\n---\n\u0060\u0060\u0060\u0060\n\u0060\u0060\u0060\n  - <!-- [[A]] -->%%[[A]]%%B\n- [^unused]: [[A]]\nÉ\n ^A\n[[B|alias]]# A\n\n\n\n- > [!note] title\n-   \u0060\u0060\u0060\n     ^a\n    \u0060\u0060\u0060\n- > [!note] title\n    ## [[A|alias]]\n## A\n \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[#A]]> > [!unknown] title\n    \u0060\u0060\u0060\n| a | b |\n|---|---|\n| [[A]] | ^a |\nref[^n]\n> [!note] title\nA\n---\n\\\u0060> > ~~~~\n    ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## <em>A</em>\n# A\n0\u0060[^n]: [[A]]\n\n    [[B]]\n  > [!note] title\nA\n---\n\nA\n===\n^absent-prose -  ^é\n- > [!note] title\n## A-2\n## A\n## A\nA\n- - [ ] [[A]]\n<!--\u0060\u0060\u0060\u0060 go [[A]]\n- > [!note] title\n\\\\[[A]]\u0060\u0060\u0060 [[A]]\n# A\n%%[[A]]%%\u0060- [x] [[B]]\n> # A\n0[[A]] [[A]]ref[^n]\n<!-- [[A]] -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## <em>A</em>\n# A\n0\u0060[^n]: [[A]]\n\n    [[B]]\n  > [!note] title\nA\n---\n\nA\n===\n^absent-prose -  ^é\n- > [!note] title\n## A-2\n## A\n## A\nA\n- - [ ] [[A]]\n<!--\u0060\u0060\u0060\u0060 go [[A]]\n- > [!note] title\n\\\\[[A]]\u0060\u0060\u0060 [[A]]\n# A\n%%[[A]]%%\u0060- [x] [[B]]\n> # A\n0[[A]] [[A]]ref[^n]\n<!-- [[A]] -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## <em>A</em>\n# A\n0\u0060[^n]: [[A]]\n\n    [[B]]\n  > [!note] title\nA\n---\n\nA\n===\n^absent-prose -  ^é\n- > [!note] title\n## A-2\n## A\n## A\nA\n- - [ ] [[A]]\n<!--\u0060\u0060\u0060\u0060 go [[A]]\n- > [!note] title\n\\\\[[A]]\u0060\u0060\u0060 [[A]]\n# A\n%%[[A]]%%\u0060- [x] [[B]]\n> # A\n0[[A]] [[A]]ref[^n]\n<!-- [[A]] -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## <em>A</em>\n# A\n0\u0060[^n]: [[A]]\n\n    [[B]]\n  > [!note] title\nA\n---\n\nA\n===\n^absent-prose -  ^é\n- > [!note] title\n## A-2\n## A\n## A\nA\n- - [ ] [[A]]\n<!--\u0060\u0060\u0060\u0060 go [[A]]\n- > [!note] title\n\\\\[[A]]\u0060\u0060\u0060 [[A]]\n# A\n%%[[A]]%%\u0060- [x] [[B]]\n> # A\n0[[A]] [[A]]ref[^n]\n<!-- [[A]] -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^A\n# A\n> [!note] title\nA\n===\nA\n---\n[[#A]] ^a\n章節\nA\n===\n> [!unknown] title\n ^é\n-->[[B|alias]]B\n> [!note] title\n ^é\n## A\n## A\n\\[[A]][[A\\|alias]]\u0060open\n[[A]]\nclose\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^A\n# A\n> [!note] title\nA\n===\nA\n---\n[[#A]] ^a\n章節\nA\n===\n> [!unknown] title\n ^é\n-->[[B|alias]]B\n> [!note] title\n ^é\n## A\n## A\n\\[[A]][[A\\|alias]]\u0060open\n[[A]]\nclose\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^A\n# A\n> [!note] title\nA\n===\nA\n---\n[[#A]] ^a\n章節\nA\n===\n> [!unknown] title\n ^é\n-->[[B|alias]]B\n> [!note] title\n ^é\n## A\n## A\n\\[[A]][[A\\|alias]]\u0060open\n[[A]]\nclose\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^A\n# A\n> [!note] title\nA\n===\nA\n---\n[[#A]] ^a\n章節\nA\n===\n> [!unknown] title\n ^é\n-->[[B|alias]]B\n> [!note] title\n ^é\n## A\n## A\n\\[[A]][[A\\|alias]]\u0060open\n[[A]]\nclose\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n<!-- [[A]] -->[[A#^a]]%%[[A]]%%\u0060\u0060\u0060\u0060 go [[A]]\n%%0> > A\n===\n  \u0060\u0060\u0060\n[[#A]]\t章節\n[[A]]  > [!note] title\n ^é\n%%![[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- [x] [[B]]\n# A\n ## A-2\n## A\n## A\n\n\n[[A#^a]]> >    \u0060\u0060\u0060\n%% ^a\nB\n[^n]: [[A]]\n\n    [[B]]\n-->A\n---\n# A\n\u0060[[A]]\u0060- [x] [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- [x] [[B]]\n# A\n ## A-2\n## A\n## A\n\n\n[[A#^a]]> >    \u0060\u0060\u0060\n%% ^a\nB\n[^n]: [[A]]\n\n    [[B]]\n-->A\n---\n# A\n\u0060[[A]]\u0060- [x] [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n---\n![[image.png]]text  ^a-2\n  - <!-- [[A]] -->^absent-prose <!--\n[[A]]\n--># A\n## A\n## A\n<!-- [[A]] -->text %%[[A]]%%^absent-prose > [!note] [[A]]\n> [!unknown] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n---\n![[image.png]]text  ^a-2\n  - <!-- [[A]] -->^absent-prose <!--\n[[A]]\n--># A\n## A\n## A\n<!-- [[A]] -->text %%[[A]]%%^absent-prose > [!note] [[A]]\n> [!unknown] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n---\n![[image.png]]text  ^a-2\n  - <!-- [[A]] -->^absent-prose <!--\n[[A]]\n--># A\n## A\n## A\n<!-- [[A]] -->text %%[[A]]%%^absent-prose > [!note] [[A]]\n> [!unknown] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->\\[[A]]  > [!note] title\n<!--\n[[A]]\n-->## A-2\n## A\n## A\n<!--^absent-prose  \u0060\u0060\u0060\n\u0060\u0060\u0060\u0060\u0060 [[A]]\n[[A\\]]<!-- [[A]] -->A\n===\n> [!note] [[A]]\n  > [!note] title\n~~~~\n[[image.png]]章節\n  > [!note] title\n^absent-prose > > [[B|alias]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->\\[[A]]  > [!note] title\n<!--\n[[A]]\n-->## A-2\n## A\n## A\n<!--^absent-prose  \u0060\u0060\u0060\n\u0060\u0060\u0060\u0060\u0060 [[A]]\n[[A\\]]<!-- [[A]] -->A\n===\n> [!note] [[A]]\n  > [!note] title\n~~~~\n[[image.png]]章節\n  > [!note] title\n^absent-prose > > [[B|alias]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "    [^n]: [[A]]\n\n    [[B]]\nhttps://example.invalid/\u0060[[A]]\u0060 \u0060\u0060\u0060\nÉ\n0[[A]] [[A]]- [ ] [[A]]\nA\n===\n- item\n\n      >  ^a-2\n^absent-prose \tÉ\n<!--\n[[A]]\n--> ^é\nÉ\n[[B|alias]]> [!unknown] title\n[[A\nB]][[A]]\u0060- \u0060> [!note] one\n> [!note] two\n> [!note] three\n[[A\nB]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"https-example-invalid-a-é-0a-a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "https://example.invalid/\u0060[[A]]\u0060 > >   \u0060\u0060\u0060\n## A\n## A\n- > [!note] title\n| a | b |\n|---|---|\n| [[A]] | ^a |\n## !\n[[A#A]]> > ![[A]]\t[[A#^a]]<!-- [[A]] -->A\n---\n\u0060\u0060 ^A\n    A\n章節\n  - %%A\n---\nÉ\n\u0060\u0060\u0060\u0060\n~~~~\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n\u0060open\n[[A]]\nclose\u0060~~~\n> [!note] title\nA\n---\n~~~\n> [!unknown] title\n    \u0060\u0060\u0060\n%%[[A]]%%É\nÉ\n> [!note] [[A]]\n> [!unknown] title\n\t\u0060\u0060\u0060\nref[^n]\n# A\n章節\n~~~\n\\\u0060\u0060\\\u0060## !\n-->## A-2\n## A\n## A\n- [ ] [[A]]\n\u0060\u0060\u0060\u0060 go [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n\u0060open\n[[A]]\nclose\u0060~~~\n> [!note] title\nA\n---\n~~~\n> [!unknown] title\n    \u0060\u0060\u0060\n%%[[A]]%%É\nÉ\n> [!note] [[A]]\n> [!unknown] title\n\t\u0060\u0060\u0060\nref[^n]\n# A\n章節\n~~~\n\\\u0060\u0060\\\u0060## !\n-->## A-2\n## A\n## A\n- [ ] [[A]]\n\u0060\u0060\u0060\u0060 go [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n\u0060open\n[[A]]\nclose\u0060~~~\n> [!note] title\nA\n---\n~~~\n> [!unknown] title\n    \u0060\u0060\u0060\n%%[[A]]%%É\nÉ\n> [!note] [[A]]\n> [!unknown] title\n\t\u0060\u0060\u0060\nref[^n]\n# A\n章節\n~~~\n\\\u0060\u0060\\\u0060## !\n-->## A-2\n## A\n## A\n- [ ] [[A]]\n\u0060\u0060\u0060\u0060 go [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n\u0060open\n[[A]]\nclose\u0060~~~\n> [!note] title\nA\n---\n~~~\n> [!unknown] title\n    \u0060\u0060\u0060\n%%[[A]]%%É\nÉ\n> [!note] [[A]]\n> [!unknown] title\n\t\u0060\u0060\u0060\nref[^n]\n# A\n章節\n~~~\n\\\u0060\u0060\\\u0060## !\n-->## A-2\n## A\n## A\n- [ ] [[A]]\n\u0060\u0060\u0060\u0060 go [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!--\n[[A]]\n-->[[#A]]\n\n> [!note] [[A]]\n> [!unknown] title\n![[image.png]]  > [!note] title\n## A-2\n## A\n## A\n^absent-prose  ![[A#A]][^n]: [[A]]\n\n    [[B]]\n[[A]]\u0060\u0060\u0060\u0060\n[[B|alias]]%%<!--B\n> [[A#^a]]## A\n## A\n[^n]: [[A]]\n\n    [[B]]\nA\n===\n## [[A|alias]]\n ^a\n- > [!note] title\n\u0060\u0060\u0060\nÉ\n\u0060\u0060\u0060 [[A]]\n- [x] [[B]]\nA\n===\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^a-2\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!note] one\n> [!note] two\n> [!note] three\n0> [!unknown] title\n[[A]] [[A]]\u0060\u0060É\n## A-2\n## A\n## A\n ^A\n> https://example.invalid/\u0060[[A]]\u0060 ## A\n## A\n<!--\t\u0060\u0060\u0060\n ^a-2\n\u0060\u0060\u0060\u0060 go [[A]]\n[[A#^a]][^unused]: [[A]]\n- ![[image.png]][[A\\]]  \u0060\u0060\u0060\n\u0060\u0060\u0060\u0060\u0060\u0060 go [[A]]\n-->## A\n## A\nref[^n]\n> [!note] title\n\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^a-2\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!note] one\n> [!note] two\n> [!note] three\n0> [!unknown] title\n[[A]] [[A]]\u0060\u0060É\n## A-2\n## A\n## A\n ^A\n> https://example.invalid/\u0060[[A]]\u0060 ## A\n## A\n<!--\t\u0060\u0060\u0060\n ^a-2\n\u0060\u0060\u0060\u0060 go [[A]]\n[[A#^a]][^unused]: [[A]]\n- ![[image.png]][[A\\]]  \u0060\u0060\u0060\n\u0060\u0060\u0060\u0060\u0060\u0060 go [[A]]\n-->## A\n## A\nref[^n]\n> [!note] title\n\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^a-2\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!note] one\n> [!note] two\n> [!note] three\n0> [!unknown] title\n[[A]] [[A]]\u0060\u0060É\n## A-2\n## A\n## A\n ^A\n> https://example.invalid/\u0060[[A]]\u0060 ## A\n## A\n<!--\t\u0060\u0060\u0060\n ^a-2\n\u0060\u0060\u0060\u0060 go [[A]]\n[[A#^a]][^unused]: [[A]]\n- ![[image.png]][[A\\]]  \u0060\u0060\u0060\n\u0060\u0060\u0060\u0060\u0060\u0060 go [[A]]\n-->## A\n## A\nref[^n]\n> [!note] title\n\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^a-2\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!note] one\n> [!note] two\n> [!note] three\n0> [!unknown] title\n[[A]] [[A]]\u0060\u0060É\n## A-2\n## A\n## A\n ^A\n> https://example.invalid/\u0060[[A]]\u0060 ## A\n## A\n<!--\t\u0060\u0060\u0060\n ^a-2\n\u0060\u0060\u0060\u0060 go [[A]]\n[[A#^a]][^unused]: [[A]]\n- ![[image.png]][[A\\]]  \u0060\u0060\u0060\n\u0060\u0060\u0060\u0060\u0060\u0060 go [[A]]\n-->## A\n## A\nref[^n]\n> [!note] title\n\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-6\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%[[A]]%%![[image.png]]A\n===\n- > [!note] title\n[[A]] [[A]]\u0060\u0060\u0060\u0060 go [[A]]\n~~~~\n ^é\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-png-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "    1. # A\n<!--\n[[A]]\n-->> [!note] one\n> [!note] two\n> [!note] three\n0^absent-prose A\n---\n1. > A\n---\n## A-2\n## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n%%É\n >  \u0060\u0060\u0060\n%%    \u0060[[A]]\u0060A\n\nB\n\u0060## <em>A</em>\n[[A]]![[A#A]]\u0060\u0060## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0-absent-prose-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "    1. # A\n<!--\n[[A]]\n-->> [!note] one\n> [!note] two\n> [!note] three\n0^absent-prose A\n---\n1. > A\n---\n## A-2\n## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n%%É\n >  \u0060\u0060\u0060\n%%    \u0060[[A]]\u0060A\n\nB\n\u0060## <em>A</em>\n[[A]]![[A#A]]\u0060\u0060## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\n![[image.png]]<!-- [[A]] -->A\n---\n \u0060\u0060\u0060\n[[#A]]> > <!--\n[[A]]\n-->0> [!unknown] title\n\u0060\u0060\u0060 [[A]]\n> [!note] title\n%%> [!note] [[A]]\n![[image.png]]- > [!note] title\n^absent-prose ## [[A|alias]]\n%%[[A]]%%[^unused]: [[A]]\n## <em>A</em>\n> > # A\n  > [!note] title\n ^é\n[[A\\|alias]]É\n\u0060\u0060\u0060\u0060\n~~~\n  - A\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-png-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\\u0060[[A#^a]]\u0060[[A]]\u0060[[#A]]É\nB\n\u0060open\n[[A]]\nclose\u0060A\n===\n0A\n===\n     > [!unknown] title\n> > \u0060\u0060\t\u0060\u0060\u0060\n> [!note] one\n> [!note] two\n> [!note] three\n ^A\n[^n]: [[A]]\n\n    [[B]]\n    ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\\u0060[[A#^a]]\u0060[[A]]\u0060[[#A]]É\nB\n\u0060open\n[[A]]\nclose\u0060A\n===\n0A\n===\n     > [!unknown] title\n> > \u0060\u0060\t\u0060\u0060\u0060\n> [!note] one\n> [!note] two\n> [!note] three\n ^A\n[^n]: [[A]]\n\n    [[B]]\n    ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-a-aé-b-open-a-closea\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n---\n<!-- [[A]] -->\u0060open\n[[A]]\nclose\u0060\\\\[[A]]0https://example.invalid/\u0060[[A]]\u0060  ![[A]]> > [!note] title\n![[A#A]][[A#A]]> [!note] one\n> [!note] two\n> [!note] three\n> https://example.invalid/\u0060[[A]]\u0060 ## A\n## A\n0A\n---\n\u0060\u0060    \u0060\u0060\u0060\n    ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n---\n<!-- [[A]] -->\u0060open\n[[A]]\nclose\u0060\\\\[[A]]0https://example.invalid/\u0060[[A]]\u0060  ![[A]]> > [!note] title\n![[A#A]][[A#A]]> [!note] one\n> [!note] two\n> [!note] three\n> https://example.invalid/\u0060[[A]]\u0060 ## A\n## A\n0A\n---\n\u0060\u0060    \u0060\u0060\u0060\n    ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "# A\n[[A]]> <!-- [[A]] -->[^unused]: [[A]]\n<!-- [[A]] --><!-- [[A]] -->- [ ] [[A]]\n ^a-2\n- item\n\n      - A\n===\n\u0060  \u0060\u0060\u0060\n> > \u0060\u0060\u0060\u0060\n%%[[A]]%%## <em>A</em>\n1. [[A\nB]][[A\nB]]> > \u0060[[A]]\u0060[[A\\]] \n\n\t\u0060\u0060\u0060\n## !\n## A\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n> > \t\u0060\u0060\u0060\u0060\n-->- [x] [[B]]\n## A-2\n## A\n## A\nA\n---\n[[B|alias]]\t\u0060\u0060\u0060\n> [!note] one\n> [!note] two\n> [!note] three\nA\n===\n\u0060\u0060\u0060 [[A]]\n\n~~~~\n\u0060\u0060\u0060\u0060\n0\u0060\u0060\u0060\nref[^n]\n[[image.png]][[#A]]\u0060\u0060- [x] [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n> > \t\u0060\u0060\u0060\u0060\n-->- [x] [[B]]\n## A-2\n## A\n## A\nA\n---\n[[B|alias]]\t\u0060\u0060\u0060\n> [!note] one\n> [!note] two\n> [!note] three\nA\n===\n\u0060\u0060\u0060 [[A]]\n\n~~~~\n\u0060\u0060\u0060\u0060\n0\u0060\u0060\u0060\nref[^n]\n[[image.png]][[#A]]\u0060\u0060- [x] [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n> > \t\u0060\u0060\u0060\u0060\n-->- [x] [[B]]\n## A-2\n## A\n## A\nA\n---\n[[B|alias]]\t\u0060\u0060\u0060\n> [!note] one\n> [!note] two\n> [!note] three\nA\n===\n\u0060\u0060\u0060 [[A]]\n\n~~~~\n\u0060\u0060\u0060\u0060\n0\u0060\u0060\u0060\nref[^n]\n[[image.png]][[#A]]\u0060\u0060- [x] [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\nref[^n]\nA\n===\n\n\n    \u0060\u0060\u0060\n[^n]: [[A]]\n\n    [[B]]\nÉ\n## A\n- item\n\n      # A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"ref1-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## <em>A</em>\n<!-- [[A]] -->## A-2\n## A\n## A\n~~~~\n[^unused]: [[A]]\n[[A\\]]-->\u0060\u0060\u0060 [[A]]\nÉ\n章節\n  \u0060\u0060\u0060\n## A\n0 \u0060\u0060\u0060\n## A\n\n\u0060## A-2\n## A\n## A\n\t\u0060\u0060\u0060\nB\nÉ\n^absent-prose > \t    \u0060\u0060\u0060\n%%\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!unknown] title\n<!--\n[[A]]\n-->[[A#A]]\t> [!note] one\n> [!note] two\n> [!note] three\nA\n---\n\u0060\u0060A\n- > [!note] title\ntext \u0060open\n[[A]]\nclose\u0060> [!unknown] title\n-->[[A]]  \u0060\u0060\u0060\n0## <em>A</em>\n> [!note] [[A]]\n> [!note] title\n-->\u0060\u0060\u0060\u0060 go [[A]]\n\t\u0060\u0060\u0060\n## A-2\n## A\n## A\nA\n- [x] [[B]]\n[^n]: [[A]]\n\n    [[B]]\n\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!unknown] title\n<!--\n[[A]]\n-->[[A#A]]\t> [!note] one\n> [!note] two\n> [!note] three\nA\n---\n\u0060\u0060A\n- > [!note] title\ntext \u0060open\n[[A]]\nclose\u0060> [!unknown] title\n-->[[A]]  \u0060\u0060\u0060\n0## <em>A</em>\n> [!note] [[A]]\n> [!note] title\n-->\u0060\u0060\u0060\u0060 go [[A]]\n\t\u0060\u0060\u0060\n## A-2\n## A\n## A\nA\n- [x] [[B]]\n[^n]: [[A]]\n\n    [[B]]\n\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<div>\n[[A]]\n</div>\n<!-- [[A]] -->  - ![[image.png]]  > [!note] title\n\n\n\n\n\tA\n---\n[^n]: [[A]]\n\n    [[B]]\n## <em>A</em>\n# A\n## !\n  - [[A\nB]]\nA\n> > \\[[A]]\u0060\u0060\u0060 [[A]]\n[[B|alias]]<div>\n[[A]]\n</div>\nA\n> [!note] one\n> [!note] two\n> [!note] three\nA\n---\n[[B|alias]]\u0060\u0060\u0060\n[[A]] [[A]] ^a-2\n## <em>A</em>\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<div>\n[[A]]\n</div>\n<!-- [[A]] -->  - ![[image.png]]  > [!note] title\n\n\n\n\n\tA\n---\n[^n]: [[A]]\n\n    [[B]]\n## <em>A</em>\n# A\n## !\n  - [[A\nB]]\nA\n> > \\[[A]]\u0060\u0060\u0060 [[A]]\n[[B|alias]]<div>\n[[A]]\n</div>\nA\n> [!note] one\n> [!note] two\n> [!note] three\nA\n---\n[[B|alias]]\u0060\u0060\u0060\n[[A]] [[A]] ^a-2\n## <em>A</em>\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"em-a-em-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n> [!note] [[A]]\nA\n---\n![[A#A]]> ## [[A|alias]]\n0\u0060\u0060## A-2\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n> [!note] [[A]]\nA\n---\n![[A#A]]> ## [[A|alias]]\n0\u0060\u0060## A-2\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\nA\n===\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#A]]<!--\u0060\u0060\u0060 [[A]]\n\u0060open\n[[A]]\nclose\u0060ref[^n]\n-->\\\u0060\\\u0060[[A#^a]]\u0060open\n[[A]]\nclose\u0060A\n---\n## !\n> [!note] one\n> [!note] two\n> [!note] three\n0[[B|alias]]^absent-prose # A\n![[A#A]]> [^n]: [[A]]\n\n    [[B]]\n~~~\n\n\n[[A#^a]]- > [!note] title\n[[B|alias]]\u0060\u0060\u0060\n\u0060\u0060\u0060\nref[^n]\n[[A\nB]]\t\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-aopen-a-closea\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\t ^a\n-->## [[A|alias]]\n<!-- [[A]] -->## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n\n> [!note] title\n0[[image.png]]B\n ^a\nA\n===\n    \u0060\u0060\u0060\n\u0060\u0060\u0060 [[A]]\n[[image.png]]ref[^n]\n- [x] [[B]]\n  > [!note] title\n<!-- [[A]] -->0 ^é\n01. <!--\n[[A]]\n--><!-- [[A]] -->[[B|alias]]A\n---\n   \u0060\u0060\u0060\n\n\n\n<!-- [[A]] --> \\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0image-pngb-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[A#A]]É\n\u0060\u0060https://example.invalid/\u0060[[A]]\u0060 ## [[A|alias]]\n  > [!note] title\nA\n===\n## <em>A</em>\n\u0060\u0060- [x] [[B]]\n## !\n[[A#A]]## A\n 章節\n[[A]]\\\\[[A]]- item\n\n       \u0060\u0060\u0060\n> [!unknown] title\n\u0060\u0060\u0060\ntext  ^é\n\t\u0060[[A]]\u0060    \u0060\u0060\u0060\n0",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[A]]- item\n\n          [^unused]: [[A]]\n<!--\n[[A]]\n-->[^unused]: [[A]]\n<!--\n[[A]]\n-->## [[A|alias]]\n0<!-- [[A]] -->A\n---\n> [!note] title\n[[A]] [[A]]   \u0060\u0060\u0060\n\t\u0060\u0060\u0060\n\\\\[[A]]%%[[A]]%% ^é\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n\t\u0060\u0060\u0060\n- item\n\n       ^é\n![[image.png]]\n<div>\n[[A]]\n</div>\n## A\n ^A\n  - ref[^n]\n\\\\[[A]]https://example.invalid/\u0060[[A]]\u0060 ^absent-prose   \u0060\u0060\u0060\n  > [!note] title\n0## A\n[[A#^a]] ^é\n\\\\[[A]] ^A\n# A\nA\n===\n> [!unknown] title\n## !\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\\\u0060 ^é\n[[A\nB]]> ## !\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n\t\u0060\u0060\u0060\n- item\n\n       ^é\n![[image.png]]\n<div>\n[[A]]\n</div>\n## A\n ^A\n  - ref[^n]\n\\\\[[A]]https://example.invalid/\u0060[[A]]\u0060 ^absent-prose   \u0060\u0060\u0060\n  > [!note] title\n0## A\n[[A#^a]] ^é\n\\\\[[A]] ^A\n# A\nA\n===\n> [!unknown] title\n## !\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\\\u0060 ^é\n[[A\nB]]> ## !\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "É\n<!-- [[A]] -->    \u0060\u0060\u0060\n## A\n## A\n\u0060open\n[[A]]\nclose\u0060text \\[[A]]## [[A|alias]]\n\u0060A\n===\n\u0060\u0060 ^é\n-->-->\\[[A]]%%A\n===\n[[A\nB]] ~~~\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "É\n<!-- [[A]] -->    \u0060\u0060\u0060\n## A\n## A\n\u0060open\n[[A]]\nclose\u0060text \\[[A]]## [[A|alias]]\n\u0060A\n===\n\u0060\u0060 ^é\n-->-->\\[[A]]%%A\n===\n[[A\nB]] ~~~\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"open-a-closetext-a-alias-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "^absent-prose [[A]] [[A]]    \\\\[[A]]> [!note] one\n> [!note] two\n> [!note] three\n<!-- ^a\n-->## A-2\n## A\n## A\n^absent-prose <div>\n[[A]]\n</div>\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "https://example.invalid/\u0060[[A]]\u0060 > [!unknown] title\n<div>\n[[A]]\n</div>\n\u0060 \u0060\u0060\u0060\n ^é\n![[image.png]]- [ ] [[A]]\n ^a\n  > [!note] title\nA\n===\n\u0060\u0060\\[[A]]> [!note] one\n> [!note] two\n> [!note] three\n> > text  ^é\n%%0[[A]][[A\\|alias]]- item\n\n      > [!note] title\n[[A\\]]\u0060\u0060   \u0060\u0060\u0060\n\u0060\u0060\u0060\u0060\n\u0060open\n[[A]]\nclose\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] [[A]]\n ^é\nA\n===\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"é-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!note] [[A]]\n<div>\n[[A]]\n</div>\n- [ ] [[A]]\n-   - \\\u0060É\n\n![[A]][[A\\|alias]]~~~~\n0[[A\\]]-->> [!note] title\n ^A\nA\n===\n章節\n<!--\n[[A]]\n-->0 ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-alias-0a-note-title-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!--\n[[A]]\n-->%%[[A]]%%  -  ^A\n  ^A\n| a | b |\n|---|---|\n| [[A]] | ^a |\nA\n===\n ^a-2\nA\n===\n\t \u0060\u0060\u0060\n[[#A]]## !\n0[^unused]: [[A]]\n-->| a | b |\n|---|---|\n| [[A]] | ^a |\n1. [^unused]: [[A]]\n   \u0060\u0060\u0060\n## A\n## A\n\n\n[[A]] [[A]]text \u0060 ^A\n   \u0060\u0060\u0060\n ^é\n%%[[A]]%%\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> > ## A\nA\n===\ntext [[A\nB]]## A\n## A\n\u0060 \u0060\u0060\u0060\n\u0060\u0060[[image.png]] ^é\n  - \\[[A]][[B|alias]]%%^absent-prose     \u0060\u0060\u0060\nA\n---\n> [!note] one\n> [!note] two\n> [!note] three\n[[B|alias]]A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n<!--> [!note] title\n[[A\\|alias]]![[image.png]]text - item\n\n      ## A-2\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> > ## A\nA\n===\ntext [[A\nB]]## A\n## A\n\u0060 \u0060\u0060\u0060\n\u0060\u0060[[image.png]] ^é\n  - \\[[A]][[B|alias]]%%^absent-prose     \u0060\u0060\u0060\nA\n---\n> [!note] one\n> [!note] two\n> [!note] three\n[[B|alias]]A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n<!--> [!note] title\n[[A\\|alias]]![[image.png]]text - item\n\n      ## A-2\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!--\n[[A]]\n-->- [ ] [[A]]\n%%[[A]]%%## A-2\n## A\n## A\n  \u0060\u0060\u0060\n## A\n[[A#^a]]0 ^a-2\n \u0060\u0060\u0060\u0060 go [[A]]\n## !\nA\n===\n ^A\n ^a\n  - ## <em>A</em>\n    \u0060\u0060\u0060\nB\n- > [!note] title\n\u0060\u0060\u0060\n[^n]: [[A]]\n\n    [[B]]\n章節\n[[A\\]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060\u0060\u0060[[A]]\u0060ref[^n]\n  > [!note] title\n## [[A|alias]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\u0060[[A]]\u0060![[A]][[#A]]text  ^é\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-alias\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n[[A#A]]0> [!unknown] title\n[^n]: [[A]]\n\n    [[B]]\ntext [[A]][[A\\|alias]][[B|alias]]## <em>A</em>\nA\n===\n> [!note] one\n> [!note] two\n> [!note] three\n[[A\\]]1. | a | b |\n|---|---|\n| [[A]] | ^a |\n> [!note] [[A]]\n## A-2\n## A\n## A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n<!-- [[A]] -->## [[A|alias]]\n> [!note] one\n> [!note] two\n> [!note] three\n## <em>A</em>\n- item\n\n      \\\u0060 ^A\n ^é\n![[A]]A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n[[A#A]]0> [!unknown] title\n[^n]: [[A]]\n\n    [[B]]\ntext [[A]][[A\\|alias]][[B|alias]]## <em>A</em>\nA\n===\n> [!note] one\n> [!note] two\n> [!note] three\n[[A\\]]1. | a | b |\n|---|---|\n| [[A]] | ^a |\n> [!note] [[A]]\n## A-2\n## A\n## A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n<!-- [[A]] -->## [[A|alias]]\n> [!note] one\n> [!note] two\n> [!note] three\n## <em>A</em>\n- item\n\n      \\\u0060 ^A\n ^é\n![[A]]A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A-2\n## A\n## A\n## A\n## A\n> [!note] [[A]]\n    > [!note] [[A]]\nA\n===\n![[A]] ^A\n \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A-2\n## A\n## A\n## A\n## A\n> [!note] [[A]]\n    > [!note] [[A]]\nA\n===\n![[A]] ^A\n \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A-2\n## A\n## A\n## A\n## A\n> [!note] [[A]]\n    > [!note] [[A]]\nA\n===\n![[A]] ^A\n \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A-2\n## A\n## A\n## A\n## A\n> [!note] [[A]]\n    > [!note] [[A]]\nA\n===\n![[A]] ^A\n \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-6\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#A]]\n> [!note] title\nA\n---\n[^unused]: [[A]]\n[[A\nB]]\n\n- [x] [[B]]\n- [ ] [[A]]\n[[A#^a]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060\u0060\u0060\n[[#A]] ^é\nA\n1.   > [!note] title\n  -  ^é\n\t[[A]] [[A]]## [[A|alias]]\n> [!note] title\n0 ^a\n-->![[image.png]]ref[^n]\n章節\nA\n---\n  \u0060\u0060\u0060\n\\\u0060## A-2\n## A\n## A\nÉ\n%%- item\n\n      ## A\n[[A\\|alias]] ^a\n## !\n> [!note] one\n> [!note] two\n> [!note] three\nA\n---\n<!-- [[A]] -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- [ ] [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\nA\n===\n ^A\n>   - ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## [[A|alias]]\n<!-- [[A]] --> \u0060\u0060\u0060\nA\n===\n   \u0060\u0060\u0060\n## <em>A</em>\n%%[[A]]%%> > \n\n<!--\n[[A]]\n-->[^n]: [[A]]\n\n    [[B]]\n![[A#A]] ^A\n\t[[A]] [[A]]\\\u0060<!--\\[[A]]^absent-prose ~~~~\n[^n]: [[A]]\n\n    [[B]]\n[[A\nB]] > [!note] one\n> [!note] two\n> [!note] three\n^absent-prose  ^a\n| a | b |\n|---|---|\n| [[A]] | ^a |\nA\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"absent-prose-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#A]]<div>\n[[A]]\n</div>\n> [!note] title\n[[A\nB]] ^a-2\nÉ\n> [!note] [[A]]\n[[A\\|alias]]\\\u0060~~~\n## A-2\n## A\n## A\n    \u0060\u0060\u0060\n    A\n===\n\\[[A]]~~~~\n[[A#^a]]  > [!note] title\n~~~~\n   \u0060\u0060\u0060\n> [!note] title\n## !\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[image.png]]- item\n\n      <!--<div>\n[[A]]\n</div>\n  - <!--\n[[A]]\n-->> > > ## A\n## A\n[[A\\|alias]][[#A]]\u0060open\n[[A]]\nclose\u0060![[A]]  > [!note] title\n \u0060\u0060\u0060\n[[A]]https://example.invalid/\u0060[[A]]\u0060 ref[^n]\nA\n===\n\t\u0060\u0060\u0060\n[[image.png]]\\\u0060[^n]: [[A]]\n\n    [[B]]\n- [x] [[B]]\n\n\n\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n[[A\nB]]章節\n\\\u0060\u0060\u00600B\n    \u0060\u0060\u0060\n- [ ] [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n\n| a | b |\n|---|---|\n| [[A]] | ^a |\n%%\u0060> > [[A\nB]]\n\n\u0060open\n[[A]]\nclose\u0060| a | b |\n|---|---|\n| [[A]] | ^a |\n ^A\n0%%[[A]]%%text ![[A#A]]\u0060![[A#A]]A\n===\n- [ ] [[A]]\n%%[[A#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "https://example.invalid/\u0060[[A]]\u0060 > [!note] title\n\u0060    \u0060\u0060\u0060\n0A\n---\n  > [!note] title\n\\\u0060-->[[A]]\n0[[A#A]] ^a-2\n> > [[A#^a]]text ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"https-example-invalid-a-note-title-0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "| a | b |\n|---|---|\n| [[A]] | ^a |\n<div>\n[[A]]\n</div>\n- item\n\n      \u0060open\n[[A]]\nclose\u0060[^n]: [[A]]\n\n    [[B]]\n    <!--[[A#A]]# A\nA\n%%[[A]]%%\u0060open\n[[A]]\nclose\u0060# A\n![[image.png]]- item\n\n      \u0060\u0060\u0060 [[A]]\n## A\n# A\nÉ\n\u0060\u0060# A\n[[image.png]]> [!unknown] title\n> [!unknown] title\n%%[[A]]%%A\n===\n章節\n ^é\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "| a | b |\n|---|---|\n| [[A]] | ^a |\n<div>\n[[A]]\n</div>\n- item\n\n      \u0060open\n[[A]]\nclose\u0060[^n]: [[A]]\n\n    [[B]]\n    <!--[[A#A]]# A\nA\n%%[[A]]%%\u0060open\n[[A]]\nclose\u0060# A\n![[image.png]]- item\n\n      \u0060\u0060\u0060 [[A]]\n## A\n# A\nÉ\n\u0060\u0060# A\n[[image.png]]> [!unknown] title\n> [!unknown] title\n%%[[A]]%%A\n===\n章節\n ^é\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\\\[[A]]    \u0060\u0060\u0060\n> [!note] title\nA\n===\n%%[[A]]%%## [[A|alias]]\n  - 01. [[B|alias]]1. \u0060\u0060\u0060\u0060 go [[A]]\n[^n]: [[A]]\n\n    [[B]]\n\u0060open\n[[A]]\nclose\u0060\t\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- > [!note] title\n[[A\\]]## <em>A</em>\n  \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 ## <em>A</em>\n ^a\n0\u0060open\n[[A]]\nclose\u0060ref[^n]\n> [!note] title\n[[#A]]<!--\n[[A]]\n-->[[A#A]] ^é\n\n\n<!--\n[[A]]\n-->> [!note] [[A]]\nA\n---\n[[A\\|alias]]É\n| a | b |\n|---|---|\n| [[A]] | ^a |\n<!--\n[[A]]\n-->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": true
  },
  {
    "Body": "![[A]]A\n===\n  > [!note] title\n  > [!note] title\n\n\n ^a-2\nÉ\n> [!note] title\nA\n===\n\t\u0060\u0060\u0060\n## <em>A</em>\n%% \u0060\u0060\u0060\n0  > [!note] title\n  > [!note] title\n\u0060\u0060\u0060\u0060\u0060\n-->[[image.png]]## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[A]]A\n===\n  > [!note] title\n  > [!note] title\n\n\n ^a-2\nÉ\n> [!note] title\nA\n===\n\t\u0060\u0060\u0060\n## <em>A</em>\n%% \u0060\u0060\u0060\n0  > [!note] title\n  > [!note] title\n\u0060\u0060\u0060\u0060\u0060\n-->[[image.png]]## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "text \u0060\u0060 ^a-2\n  > [!note] title\n[[A#^a]]<!-- [[A]] -->> [!unknown] title\nÉ\n[[A#^a]]\\\\[[A]]![[image.png]]~~~~\n\n\nref[^n]\n## A-2\n## A\n## A\n> [!unknown] title\n[[A\\]][[A\nB]]<!--# A\n[[A]] [[A]]%%\u0060\u0060É\nB\nÉ\n## !\n\u0060\u0060\n\n\u0060\u0060\u0060\u0060 go [[A]]\n-->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060\u0060\u0060 [[A]]\n<!--- item\n\n        \u0060\u0060\u0060\n\\\u00600^absent-prose \u0060\u0060> [!note] title\n\u0060\u0060\u0060\nA\n---\n章節\n# A\n[[A\\|alias]]0[^unused]: [[A]]\n- [ ] [[A]]\nÉ\n[[A\\|alias]] ^a\n## A\n## A\n[[A\\|alias]]0## <em>A</em>\nA\n---\n   \u0060\u0060\u0060\n## !\nA\n===\n\\[[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"alias0-em-a-em-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[#A]]- item\n\n      ## [[A|alias]]\n0[[B|alias]]A\n===\nref[^n]\n> [!note] title\nA\n---\n| a | b |\n|---|---|\n| [[A]] | ^a |\nA\n===\n | a | b |\n|---|---|\n| [[A]] | ^a |\n~~~~\n[[B|alias]]## <em>A</em>\n\u0060\u0060É\n> [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060^absent-prose  ^a\nÉ\n- [x] [[B]]\n  - ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0aliasa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[#A]]- item\n\n      ## [[A|alias]]\n0[[B|alias]]A\n===\nref[^n]\n> [!note] title\nA\n---\n| a | b |\n|---|---|\n| [[A]] | ^a |\nA\n===\n | a | b |\n|---|---|\n| [[A]] | ^a |\n~~~~\n[[B|alias]]## <em>A</em>\n\u0060\u0060É\n> [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060^absent-prose  ^a\nÉ\n- [x] [[B]]\n  - ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "章節\n> \u0060\u0060\u0060\u0060\nref[^n]\n# A\n- item\n\n      https://example.invalid/\u0060[[A]]\u0060 > [!note] one\n> [!note] two\n> [!note] three\nA\n---\n\u0060\u0060\u0060 [[A]]\n0\u0060[[A]]\u0060- \u0060\u0060\u0060\u0060 go [[A]]\nref[^n]\n\u0060[[A]]\u0060## <em>A</em>\n   \u0060\u0060\u0060\n\t\u0060\u0060\u0060\n> [!note] one\n> [!note] two\n> [!note] three\n## <em>A</em>\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\nB]][[A\\]]A\n ^é\ntext \u0060\u0060\u0060\n- [ ] [[A]]\n# A\n[[A#^a]]  \u0060\u0060\u0060\n- \u0060\u0060\u0060 [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\nÉ\n\t\u0060\u0060\u0060\n > ## [[A|alias]]\n<!-- [[A]] -->\u0060open\n[[A]]\nclose\u0060A\n===\n\u0060[[A\\]]章節\n\\\\[[A]]<!--\n[[A]]\n-->[[A\\]]- [x] [[B]]\n ^é\n\u0060[[A]]\u0060 ^a\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"open-a-closea\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] [[A]]\n ^A\n<!-- [[A]] -->    A\n---\n    [[image.png]]A\n===\n ^A\n\n~~~~\n-->> [!unknown] title\n> >   > [!note] title\n\\\u0060> [!note] one\n> [!note] two\n> [!note] three\nA\n---\n  - > [!note] one\n> [!note] two\n> [!note] three\n^absent-prose [[A\\]][[B|alias]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\t\u0060\u0060\u0060\n\\\\[[A]]## A\n ^A\n-->ref[^n]\n<!-- [[A]] -->É\n0A\n---\n[[A#A]][[#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-a-ref-n-é-0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n## A\n## A\n  - # A\n\\\\[[A]]A\n[[#A]]A\n![[A#A]] ^é\n ^A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!note] title\n章節\n![[image.png]][[A]]## [[A|alias]]\nA\n  - <!--<div>\n[[A]]\n</div>\n ^é\n ^A\n~~~\nhttps://example.invalid/\u0060[[A]]\u0060 É\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n## A\n## A\n  - # A\n\\\\[[A]]A\n[[#A]]A\n![[A#A]] ^é\n ^A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!note] title\n章節\n![[image.png]][[A]]## [[A|alias]]\nA\n  - <!--<div>\n[[A]]\n</div>\n ^é\n ^A\n~~~\nhttps://example.invalid/\u0060[[A]]\u0060 É\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n## A\n## A\n  - # A\n\\\\[[A]]A\n[[#A]]A\n![[A#A]] ^é\n ^A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!note] title\n章節\n![[image.png]][[A]]## [[A|alias]]\nA\n  - <!--<div>\n[[A]]\n</div>\n ^é\n ^A\n~~~\nhttps://example.invalid/\u0060[[A]]\u0060 É\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"章節-image-png-a-alias-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!unknown] title\n ![[A#A]]-->[^unused]: [[A]]\n[[A]]![[A#A]]# A\nA\n---\n\t## A-2\n## A\n## A\n  -  ^é\n0[[A#^a]]A\nA\n\\[[A]][[A\\]]1. ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-unused-a-a-a-a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[#A]]- [ ] [[A]]\n0[[A\\|alias]]A\n---\n--><div>\n[[A]]\n</div>\n![[image.png]] ^A\n## !\n- [x] [[B]]\n[[A]]  > [!note] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-0aliasa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n## A\n[[B|alias]] ^A\n ^é\n[[A\nB]]![[A#A]]\u0060[[A]]\u00601. <!--A\n---\n-->[[A]]<!--\n[[A]]\n-->- [ ] [[A]]\n\n\nA\n---\n![[A]]\n\n![[A#A]]\u0060\u0060![[A#A]]## !\n\u0060\u0060  > [!note] title\n- [x] [[B]]\n- > [!note] title\n \u0060\u0060\u0060\u0060\n\t[[#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n ^é\n![[A#A]]<!--\n[[A]]\n--><!-- [[A]] -->## <em>A</em>\n> [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060\u0060\n\n\u0060\u0060\u0060\n[^n]: [[A]]\n\n    [[B]]\n\n\n> [!unknown] title\n\\\\[[A]]## A\n## A\nA\n---\nA\n===\n\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n ^é\n![[A#A]]<!--\n[[A]]\n--><!-- [[A]] -->## <em>A</em>\n> [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060\u0060\n\n\u0060\u0060\u0060\n[^n]: [[A]]\n\n    [[B]]\n\n\n> [!unknown] title\n\\\\[[A]]## A\n## A\nA\n---\nA\n===\n\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "0[[#A]]    ~~~~\n\\[[A]]- [x] [[B]]\n> [!unknown] title\n[[#A]]A\n---\n\\\\[[A]]## A\n## A\n- item\n\n      %%[[A]]%%> [!note] one\n> [!note] two\n> [!note] three\n## <em>A</em>\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] [[A]]\n- [ ] [[A]]\n[[A]] [[A]] ^A\n[[A\\|alias]]- > [!note] title\n ^é\n## A-2\n## A\n## A\nA\ntext A\n---\n[[A\\|alias]]## A\n> [!note] title\n ^a-2\n## !\n- [x] [[B]]\n[[A\nB]]%%A\n---\n[[A\nB]]## A-2\n## A\n## A\n> [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060\u0060\u0060\n[[A]]<!-- ^a\n\u0060[[A]]\u00600",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\\u0060[[A\nB]][[image.png]][[A]]   \u0060\u0060\u0060\n[[image.png]] ^é\n<!-- [[A]] -->\u0060[[A]]\u0060    \u0060\u0060\u0060\n  > [!note] title\n ^é\n  - \n[[A#^a]][[#A]]É\nhttps://example.invalid/\u0060[[A]]\u0060  ^a-2\n- item\n\n      ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"é\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\nB]]  - <!--\n[[A]]\n-->## A\n## A\n## A\n  > [!note] title\n- [ ] [[A]]\n   \u0060\u0060\u0060\n<!-- [[A]] -->[[A]] [[A]]~~~\n\\\u0060| a | b |\n|---|---|\n| [[A]] | ^a |\nÉ\n ^A\n-->> [!note] title\n[[#A]]章節\n~~~\n^absent-prose 章節\n\\[[A]][[A]]> [!unknown] title\n0\u0060\u0060\u0060\u0060 go [[A]]\n> [!unknown] title\nhttps://example.invalid/\u0060[[A]]\u0060 ![[A#A]]^absent-prose > [!note] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\nB]]  - <!--\n[[A]]\n-->## A\n## A\n## A\n  > [!note] title\n- [ ] [[A]]\n   \u0060\u0060\u0060\n<!-- [[A]] -->[[A]] [[A]]~~~\n\\\u0060| a | b |\n|---|---|\n| [[A]] | ^a |\nÉ\n ^A\n-->> [!note] title\n[[#A]]章節\n~~~\n^absent-prose 章節\n\\[[A]][[A]]> [!unknown] title\n0\u0060\u0060\u0060\u0060 go [[A]]\n> [!unknown] title\nhttps://example.invalid/\u0060[[A]]\u0060 ![[A#A]]^absent-prose > [!note] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n ^a\n![[image.png]][[A]]~~~\n## <em>A</em>\n ^A\n## A\n## A\n%% ^é\n## <em>A</em>\n-->| a | b |\n|---|---|\n| [[A]] | ^a |\n![[image.png]]\\[[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n===\n ^a\n![[image.png]][[A]]~~~\n## <em>A</em>\n ^A\n## A\n## A\n%% ^é\n## <em>A</em>\n-->| a | b |\n|---|---|\n| [[A]] | ^a |\n![[image.png]]\\[[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n-->%% ^a-2\n  > [!note] title\n[[A]]\n- \\[[A]]-->\n\n[[A\\]]1. > [!note] one\n> [!note] two\n> [!note] three\n> > https://example.invalid/\u0060[[A]]\u0060 ~~~\n  - \n## A\ntext > [!note] one\n> [!note] two\n> [!note] three\n  - \\[[A]]# A\nÉ\n    [[A#^a]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!--\n[[A]]\n-->- > [!note] title\n\u0060\u0060[[A\\]]1. ## !\n\\\u0060## A\n\t[^unused]: [[A]]\n\u0060open\n[[A]]\nclose\u0060É\n   \u0060\u0060\u0060\n1.  ^é\n[[A\\|alias]]> [!note] [[A]]\n[[image.png]]> [!note] title\n## [[A|alias]]\n\u0060\u0060\u0060\u0060 go [[A]]\n  \u0060\u0060\u0060\n\u0060[[A]]\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-alias\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] title\n[^unused]: [[A]]\n## A\n## A\n> [!note] [[A]]\n[[A]] [[A]]## A-2\n## A\n## A\n[[A]]# A\n1. 章節\n%%[[A]]%%> > \\[[A]]## A\n## A\ntext <div>\n[[A]]\n</div>\n- [ ] [[A]]\n## <em>A</em>\n![[image.png]]## A\n## A\n- > [!note] title\n> [!note] title\n# A\nÉ\n ^a\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] title\n[^unused]: [[A]]\n## A\n## A\n> [!note] [[A]]\n[[A]] [[A]]## A-2\n## A\n## A\n[[A]]# A\n1. 章節\n%%[[A]]%%> > \\[[A]]## A\n## A\ntext <div>\n[[A]]\n</div>\n- [ ] [[A]]\n## <em>A</em>\n![[image.png]]## A\n## A\n- > [!note] title\n> [!note] title\n# A\nÉ\n ^a\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] title\n[^unused]: [[A]]\n## A\n## A\n> [!note] [[A]]\n[[A]] [[A]]## A-2\n## A\n## A\n[[A]]# A\n1. 章節\n%%[[A]]%%> > \\[[A]]## A\n## A\ntext <div>\n[[A]]\n</div>\n- [ ] [[A]]\n## <em>A</em>\n![[image.png]]## A\n## A\n- > [!note] title\n> [!note] title\n# A\nÉ\n ^a\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] title\n[^unused]: [[A]]\n## A\n## A\n> [!note] [[A]]\n[[A]] [[A]]## A-2\n## A\n## A\n[[A]]# A\n1. 章節\n%%[[A]]%%> > \\[[A]]## A\n## A\ntext <div>\n[[A]]\n</div>\n- [ ] [[A]]\n## <em>A</em>\n![[image.png]]## A\n## A\n- > [!note] title\n> [!note] title\n# A\nÉ\n ^a\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] title\n[^unused]: [[A]]\n## A\n## A\n> [!note] [[A]]\n[[A]] [[A]]## A-2\n## A\n## A\n[[A]]# A\n1. 章節\n%%[[A]]%%> > \\[[A]]## A\n## A\ntext <div>\n[[A]]\n</div>\n- [ ] [[A]]\n## <em>A</em>\n![[image.png]]## A\n## A\n- > [!note] title\n> [!note] title\n# A\nÉ\n ^a\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-6\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]]https://example.invalid/\u0060[[A]]\u0060 [[A]][[B|alias]][^unused]: [[A]]\n## A-2\n## A\n## A\n## [[A|alias]]\n  > [!note] title\nA\n===\n1. <div>\n[[A]]\n</div>\n> [!note] title\n<div>\n[[A]]\n</div>\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\u0060open\n[[A]]\nclose\u0060## A\n> [!note] one\n> [!note] two\n> [!note] three\n\\\u0060## A-2\n## A\n## A\n![[image.png]]\n## A\n\u0060\u0060\u0060 [[A]]\n- %% ^a\n\u0060open\n[[A]]\nclose\u0060- ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]]https://example.invalid/\u0060[[A]]\u0060 [[A]][[B|alias]][^unused]: [[A]]\n## A-2\n## A\n## A\n## [[A|alias]]\n  > [!note] title\nA\n===\n1. <div>\n[[A]]\n</div>\n> [!note] title\n<div>\n[[A]]\n</div>\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\u0060open\n[[A]]\nclose\u0060## A\n> [!note] one\n> [!note] two\n> [!note] three\n\\\u0060## A-2\n## A\n## A\n![[image.png]]\n## A\n\u0060\u0060\u0060 [[A]]\n- %% ^a\n\u0060open\n[[A]]\nclose\u0060- ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]]https://example.invalid/\u0060[[A]]\u0060 [[A]][[B|alias]][^unused]: [[A]]\n## A-2\n## A\n## A\n## [[A|alias]]\n  > [!note] title\nA\n===\n1. <div>\n[[A]]\n</div>\n> [!note] title\n<div>\n[[A]]\n</div>\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\u0060open\n[[A]]\nclose\u0060## A\n> [!note] one\n> [!note] two\n> [!note] three\n\\\u0060## A-2\n## A\n## A\n![[image.png]]\n## A\n\u0060\u0060\u0060 [[A]]\n- %% ^a\n\u0060open\n[[A]]\nclose\u0060- ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]]https://example.invalid/\u0060[[A]]\u0060 [[A]][[B|alias]][^unused]: [[A]]\n## A-2\n## A\n## A\n## [[A|alias]]\n  > [!note] title\nA\n===\n1. <div>\n[[A]]\n</div>\n> [!note] title\n<div>\n[[A]]\n</div>\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\u0060open\n[[A]]\nclose\u0060## A\n> [!note] one\n> [!note] two\n> [!note] three\n\\\u0060## A-2\n## A\n## A\n![[image.png]]\n## A\n\u0060\u0060\u0060 [[A]]\n- %% ^a\n\u0060open\n[[A]]\nclose\u0060- ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-6\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]]https://example.invalid/\u0060[[A]]\u0060 [[A]][[B|alias]][^unused]: [[A]]\n## A-2\n## A\n## A\n## [[A|alias]]\n  > [!note] title\nA\n===\n1. <div>\n[[A]]\n</div>\n> [!note] title\n<div>\n[[A]]\n</div>\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\u0060open\n[[A]]\nclose\u0060## A\n> [!note] one\n> [!note] two\n> [!note] three\n\\\u0060## A-2\n## A\n## A\n![[image.png]]\n## A\n\u0060\u0060\u0060 [[A]]\n- %% ^a\n\u0060open\n[[A]]\nclose\u0060- ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-7\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  > [!note] title\n## A\n## A\n[[A\\|alias]]%% \u0060\u0060\u0060\n\n\\\\[[A]]text ~~~\n## <em>A</em>\n## A\n## A\n## A\n## A\n> [!note] [[A]]\n\u0060\u0060\u0060\n-->> > | a | b |\n|---|---|\n| [[A]] | ^a |\n\u0060\u0060\u0060\n## A-2\n## A\n## A\n \u0060\u0060\u0060\n| a | b |\n|---|---|\n| [[A]] | ^a |\nÉ\n## A\n \u0060\u0060\u0060\n[[A\nB]][^n]: [[A]]\n\n    [[B]]\nhttps://example.invalid/\u0060[[A]]\u0060 \n\n\u0060\u0060\u0060 [[A]]\n## !\nA\n---\n<!-- [[A]] -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\\\[[A]]A\n===\n[^unused]: [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n\t\u0060\u0060\u0060\n## A\n## A\n[[A]] [[A]][[A\\|alias]]> [!note] title\n0[^unused]: [[A]]\n<!-- [[A]] -->[[A#A]]A\n===\nÉ\n\n\nref[^n]\nhttps://example.invalid/\u0060[[A]]\u0060 | a | b |\n|---|---|\n| [[A]] | ^a |\n> [!unknown] title\n章節\n ^a\n\u0060\u0060\u0060\n[[image.png]]\u0060\u0060\u0060\u0060\n[[A\\|alias]]<!-- [[A]] -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\\\[[A]]A\n===\n[^unused]: [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n\t\u0060\u0060\u0060\n## A\n## A\n[[A]] [[A]][[A\\|alias]]> [!note] title\n0[^unused]: [[A]]\n<!-- [[A]] -->[[A#A]]A\n===\nÉ\n\n\nref[^n]\nhttps://example.invalid/\u0060[[A]]\u0060 | a | b |\n|---|---|\n| [[A]] | ^a |\n> [!unknown] title\n章節\n ^a\n\u0060\u0060\u0060\n[[image.png]]\u0060\u0060\u0060\u0060\n[[A\\|alias]]<!-- [[A]] -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-aalias-note-title-01-a-a-aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "0 ^é\n> [!unknown] title\n[[A\\]]> A\n---\n- [ ] [[A]]\n  > [!note] title\n\u0060\u0060\u0060\u0060\n- [ ] [[A]]\n\\\u0060[[A\\|alias]]\u0060\u00600![[A#A]]<!--\n[[A]]\n-->^absent-prose > [!note] [[A]]\n![[A]]## !\n- [ ] [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A]][[A]]- [x] [[B]]\n> [!note] [[A]]\n## [[A|alias]]\n  > [!note] title\n![[A]]A\n===\n\n> > [!note] title\nA\n---\nhttps://example.invalid/\u0060[[A]]\u0060 %%[[A]]%%A\n---\n   \u0060\u0060\u0060\n0A\n0  \u0060\u0060\u0060\n^absent-prose  ^a\n~~~~\n![[A#A]]<!--\n[[A]]\n-->    ![[A]][[A\\]][[A\nB]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#A]]%%[[A]]%%[[A]]![[A#A]]A\n---\n\u0060open\n[[A]]\nclose\u0060 ^a\n   \u0060\u0060\u0060\n## !\n![[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-aa-a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[A#A]]> [!note] title\n\n> [!note] [[A]]\nA\n===\n## A\n## A\n0![[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[A#A]]> [!note] title\n\n> [!note] [[A]]\nA\n===\n## A\n## A\n0![[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## <em>A</em>\n<!-- [[A]] -->## A-2\n## A\n## A\n> > https://example.invalid/\u0060[[A]]\u0060 %%[[A]]%%\u0060\u0060   \u0060\u0060\u0060\n\u0060[[A]]\u0060[[A\nB]] ^é\n\n   \u0060\u0060\u0060\n> [!unknown] title\n[[A#A]]    \u0060\u0060\u0060\u0060 go [[A]]\n[[A\\|alias]] ## A\n<!-- [[A]] -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A]] [[A]]https://example.invalid/\u0060[[A]]\u0060 -   > [!note] title\n<!-- [[A]] -->A\n===\n[[#A]]\u0060[[A]]\u0060## A\n## A\n\u0060open\n[[A]]\nclose\u0060[[image.png]][[A]] [[B|alias]][[A\\]]> [!note] [[A]]\nA\n---\n<!--\u0060\u0060\u0060\nA\n===\n# A\n ^A\n ^a\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"open-a-closeimage-pnga-aliasa-note-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## !\n  - A\n\u0060\u0060\u0060\u0060\nA\n\n\n   \u0060\u0060\u0060\n ^a\n[[A#A]][[A\\|alias]]## A\n## A\n-->B\n\u0060\u0060\u0060\u0060\n- > [!note] title\n[[A#^a]]## !\n## A\n ^A\n<!-- [[A]] --># A\n<!--[^n]: [[A]]\n\n    [[B]]\n![[image.png]]> [!unknown] title\n> [!note] one\n> [!note] two\n> [!note] three\n[[A#^a]]-->![[A]] ## <em>A</em>\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[A#A]]\\[[A]]https://example.invalid/\u0060[[A]]\u0060 [[A]] [[A]]## A-2\n## A\n## A\n\u0060\u0060[[A\\]]## A\n## A\n> [!note] [[A]]\n^absent-prose %%[[A]]%%%%[[A\\]]https://example.invalid/\u0060[[A]]\u0060 ![[A#A]]  > [!note] title\n# A\n  - ~~~\n  \u0060\u0060\u0060\n## A\n## A\n[^unused]: [[A]]\n[[A\\|alias]]<div>\n[[A]]\n</div>\n\\[[A]]~~~\n[[A#^a]][[#A]]^absent-prose ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[A#A]]\\[[A]]https://example.invalid/\u0060[[A]]\u0060 [[A]] [[A]]## A-2\n## A\n## A\n\u0060\u0060[[A\\]]## A\n## A\n> [!note] [[A]]\n^absent-prose %%[[A]]%%%%[[A\\]]https://example.invalid/\u0060[[A]]\u0060 ![[A#A]]  > [!note] title\n# A\n  - ~~~\n  \u0060\u0060\u0060\n## A\n## A\n[^unused]: [[A]]\n[[A\\|alias]]<div>\n[[A]]\n</div>\n\\[[A]]~~~\n[[A#^a]][[#A]]^absent-prose ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[image.png]]   \u0060\u0060\u0060\n[[A]] ^A\n\n\n[[A#^a]]\n\n![[image.png]]- [x] [[B]]\n    \u0060\u0060\u0060 [[A]]\n章節\n[[A\nB]]> [!unknown] title\n[[#A]]- \u0060\u0060\u0060\u0060\n\\\\[[A]]\u0060É\n<!-- [[A]] -->## A\n## A\n0",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\t\u0060\u0060\u0060\n0\u0060[[A]]> [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060- [ ] [[A]]\n ^é\nA\n===\n- [x] [[B]]\n~~~\n  \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 [[A\\|alias]]> [!note] [[A]]\n<div>\n[[A]]\n</div>\n[[A]]\u0060\u0060 ^A\n%%1. [[A#A]]\u0060\u0060\u0060\u0060 go [[A]]\n[[A]] [[A]]\\\u0060~~~\n    \u0060\u0060\u0060\n![[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-é-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A]] [[A]][[A#A]]- item\n\n      > [!note] title\n> [!unknown] title\nÉ\nA\n%%[[A]]%%## A\n## A\n## A\n- [ ] [[A]]\nA\n\u0060\u0060\u0060\n^absent-prose 0    \u0060\u0060\u0060\n> [!note] title\n\\\\[[A]]\u0060\u0060\u0060\u0060\n1. \u0060\u0060\u0060\u0060 go [[A]]\nhttps://example.invalid/\u0060[[A]]\u0060 ## [[A|alias]]\n  > [!note] title\n ^a-2\nB\n## <em>A</em>\n1. <!-- [[A]] -->## A\n## A\n0A\n===\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A]] [[A]][[A#A]]- item\n\n      > [!note] title\n> [!unknown] title\nÉ\nA\n%%[[A]]%%## A\n## A\n## A\n- [ ] [[A]]\nA\n\u0060\u0060\u0060\n^absent-prose 0    \u0060\u0060\u0060\n> [!note] title\n\\\\[[A]]\u0060\u0060\u0060\u0060\n1. \u0060\u0060\u0060\u0060 go [[A]]\nhttps://example.invalid/\u0060[[A]]\u0060 ## [[A|alias]]\n  > [!note] title\n ^a-2\nB\n## <em>A</em>\n1. <!-- [[A]] -->## A\n## A\n0A\n===\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "| a | b |\n|---|---|\n| [[A]] | ^a |\n[[B|alias]] \u0060\u0060\u0060\nA\n---\n%%[[A]]%%\t\u0060\u0060\u0060\n[[image.png]]text > > 1.   - [[A\\]]\u0060[[A]]\u0060\u0060open\n[[A]]\nclose\u0060A\n===\nref[^n]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n%% ^A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-pngtext-1-a-a-open-a-closea\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060\u0060<!-- [[A]] -->   \u0060\u0060\u0060\n<!--![[A#A]]\\\\[[A]]## A\n## A\n-->[[A\\]]A\n===\n0\u0060\u0060\u0060\n## A\n## A\n[[image.png]]> [!unknown] title\n0text \\[[A]]| a | b |\n|---|---|\n| [[A]] | ^a |\n<!--\n[[A]]\n-->## A\n## A\n ^é\nhttps://example.invalid/\u0060[[A]]\u0060 ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!--\n[[A]]\n-->-->> > | a | b |\n|---|---|\n| [[A]] | ^a |\n\t[^unused]: [[A]]\n![[A#A]]| a | b |\n|---|---|\n| [[A]] | ^a |\n0A\n---\n~~~\nhttps://example.invalid/\u0060[[A]]\u0060 ![[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-b-a-a-unused-a-a-a-a-b-a-a-0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n ^é\n- [x] [[B]]\n\\\u0060\\\\[[A]]1. ## [[A|alias]]\nÉ\n\u0060- [x] [[B]]\nÉ\n> > [[B|alias]]text > [!note] one\n> [!note] two\n> [!note] three\n ^é\nhttps://example.invalid/\u0060[[A]]\u0060 # A\n[[image.png]][[A#A]]A\n===\n~~~~\n\u0060\u0060\u0060\u0060\n  - \n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"é-https-example-invalid-a-a-image-pnga-aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^é\n    - item\n\n      \u0060\u0060\u0060\u0060\n01. ## !\n## A\n## A\n# A\n\n\n| a | b |\n|---|---|\n| [[A]] | ^a |\n  \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 \u0060\u0060\u0060\n\u0060[^n]: [[A]]\n\n    [[B]]\n## A\n<!-- [[A]] -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"section\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!--\n[[A]]\n-->![[A#A]]https://example.invalid/\u0060[[A]]\u0060 A\n---\n1. - [x] [[B]]\n> [!unknown] title\n\\\\[[A]]\\[[A]]\u0060\u0060\u0060\u0060\nÉ\n<!--\n[[A]]\n--># A\n> \\[[A]]> [!note] title\n- [ ] [[A]]\n<!--![[image.png]]\u0060\u0060\u0060\u0060\n> \n ^é\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-https-example-invalid-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> > \u0060\u0060\u0060\u0060\n[[#A]][[A\nB]]![[image.png]]\u0060[[A]]\u0060É\n> [!note] one\n> [!note] two\n> [!note] three\n<!-- [[A]] -->## A\n## A\n[[image.png]][[A\nB]]0[[B|alias]]https://example.invalid/\u0060[[A]]\u0060    \u0060\u0060\u0060\n^absent-prose ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "    \u0060\u0060\u0060\n![[A#A]]%%[[A]]%%![[image.png]]- [x] [[B]]\nA\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-image-png-x-b-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "    \n^absent-prose \t%%[[A]]%%\u0060\u0060\u0060\n  > [!note] title\n\u0060\u0060[[A]]  - A\n===\n[^n]: [[A]]\n\n    [[B]]\n~~~~\ntext  ^a-2\n> [!note] one\n> [!note] two\n> [!note] three\n![[image.png]]0~~~\n[[A\\|alias]]- [ ] [[A]]\n0\u0060open\n[[A]]\nclose\u0060A\n\\\\[[A]]1. [[A#A]]## [[A|alias]]\n \u0060\u0060\u0060\n\u0060\u0060\u0060 [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n\n\n\n\n[[B|alias]]## !\n<div>\n[[A]]\n</div>\ntext \\[[A]]<!--![[A#A]]~~~\n\n\n[[A]][[B|alias]]\n\n- > [!note] title\n> [!note] [[A]]\n## A\n## A\n\u0060\u0060\u0060\u0060\n<!--ref[^n]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A-2\n## A\n## A\n[[image.png]]%%> [!note] title\n\t\u0060\u0060\u0060\u0060 go [[A]]\n\u0060[[A]]\u0060A\n===\n## A-2\n## A\n## A\n0    \u0060\u0060\u0060\n ^a\n  > [!note] title\n[[A\\|alias]]É\n\n\n> > ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!--\n[[A]]\n-->## <em>A</em>\n> > text A\n===\nB\n## <em>A</em>\n\\\u0060章節\ntext ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"em-a-em-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n\n[[A\nB]] ^é\n## A-2\n## A\n## A\n--> ^a\n## A\n## A\n> [!note] [[A]]\n<!--\n[[A]]\n-->    \\\\[[A]]\u0060\u0060\u0060\u0060\n[[A#A]]1. 章節\n ^a-2\n  - ![[A]]## !\n<!-- [[A]] --> \u0060\u0060\u0060\nA\n===\n> > \t\u0060\u0060\u0060\n[[A\nB]][[A\nB]]\u0060\u0060\u0060\u0060\nB\n# A\n## A\n## A\n ^é\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n\n[[A\nB]] ^é\n## A-2\n## A\n## A\n--> ^a\n## A\n## A\n> [!note] [[A]]\n<!--\n[[A]]\n-->    \\\\[[A]]\u0060\u0060\u0060\u0060\n[[A#A]]1. 章節\n ^a-2\n  - ![[A]]## !\n<!-- [[A]] --> \u0060\u0060\u0060\nA\n===\n> > \t\u0060\u0060\u0060\n[[A\nB]][[A\nB]]\u0060\u0060\u0060\u0060\nB\n# A\n## A\n## A\n ^é\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n\n[[A\nB]] ^é\n## A-2\n## A\n## A\n--> ^a\n## A\n## A\n> [!note] [[A]]\n<!--\n[[A]]\n-->    \\\\[[A]]\u0060\u0060\u0060\u0060\n[[A#A]]1. 章節\n ^a-2\n  - ![[A]]## !\n<!-- [[A]] --> \u0060\u0060\u0060\nA\n===\n> > \t\u0060\u0060\u0060\n[[A\nB]][[A\nB]]\u0060\u0060\u0060\u0060\nB\n# A\n## A\n## A\n ^é\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "A\n---\n[[A\\|alias]]章節\n<!--\n[[A]]\n-->ref[^n]\n\u0060\u0060> > 0A\n===\n\n\n\t\u0060\u0060\u0060\n\nÉ\n-->B\n\n\n\\\u0060%%A\n---\n[[A\\|alias]]text \n- > [!note] title\n\u0060\u0060\u0060\u0060 go [[A]]\n![[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"ref-n-0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- [ ] [[A]]\n    \u0060\u0060\u0060\n\u0060[[A]]\u0060text ## <em>A</em>\n## A\n- [ ] [[A]]\n<div>\n[[A]]\n</div>\n[[#A]]\\[[A]]^absent-prose \n\n  ^é\n> > <!-- [[A]] -->## A\n## <em>A</em>\n  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n%%[[A]]%%- [[B|alias]]<div>\n[[A]]\n</div>\n## A\n## A\ntext - > [!note] one\n> [!note] two\n> [!note] three\n<!--[[image.png]]<!--[^unused]: [[A]]\n ^a\n\u0060[[A]]\u0060\t  -  ^é\n<!---->章節\n## A-2\n## A\n## A\n-  ^é\n- ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n%%[[A]]%%- [[B|alias]]<div>\n[[A]]\n</div>\n## A\n## A\ntext - > [!note] one\n> [!note] two\n> [!note] three\n<!--[[image.png]]<!--[^unused]: [[A]]\n ^a\n\u0060[[A]]\u0060\t  -  ^é\n<!---->章節\n## A-2\n## A\n## A\n-  ^é\n- ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\n%%[[A]]%%- [[B|alias]]<div>\n[[A]]\n</div>\n## A\n## A\ntext - > [!note] one\n> [!note] two\n> [!note] three\n<!--[[image.png]]<!--[^unused]: [[A]]\n ^a\n\u0060[[A]]\u0060\t  -  ^é\n<!---->章節\n## A-2\n## A\n## A\n-  ^é\n- ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "# A\n       \u0060\u0060\u0060\n ^é\n[[A\nB]]^absent-prose  \u0060\u0060\u0060\n- [[#A]]%%[[A]]%%B\n\\\u0060## <em>A</em>\n> [!unknown] title\n0https://example.invalid/\u0060[[A]]\u0060 -->B\nB\nA\n---\n^absent-prose [[A#A]] ^a-2\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0https-example-invalid-a-b-b-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "É\n[[#A]][[A]]   \u0060\u0060\u0060\n<!-- [[A]] -->  > [!note] title\n[[A#A]][[A\\|alias]]ref[^n]\n## A\n## A\n- [x] [[B]]\n\n\n[[B|alias]]## [[A|alias]]\nB\n ^a\n%%[[A]]%%[[A\\|alias]]章節\nA\n---\nB\n   \u0060\u0060\u0060\nÉ\n\u0060\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## [[A|alias]]\ntext [^n]: [[A]]\n\n    [[B]]\n ^A\nÉ\n[[#A]]\u0060\u0060\u0060\u0060 go [[A]]\n[[A\nB]]> [!note] one\n> [!note] two\n> [!note] three\n\\\\[[A]]A\n---\n[[A\\]][[A]]章節\n<!-- [[A]] -->É\n<!-- [[A]] -->A\n===\n\u0060open\n[[A]]\nclose\u0060 \u0060\u0060\u0060\n  -  ^é\n# A\nref[^n]\n[[A\\]]%%[[A]]%%![[A#A]]\n> B\n\u0060[[A]]\u0060 ^é\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]] ^é\n# A\n  A\n[[A\nB]][[A\\]]text [[image.png]]   \u0060\u0060\u0060\n%%[[A]]%%A\n---\n- [ ] [[A]]\n[^n]: [[A]]\n\n    [[B]]\n## [[A|alias]]\nÉ\n[[A#^a]] ^a\n ## A\n## A\n\t\u0060\u0060\u0060\n0 \u0060\u0060\u0060\n-->  > [!note] title\n![[A#A]][[A#^a]]  > [!note] title\nÉ\n> > \u0060\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]] ^é\n# A\n  A\n[[A\nB]][[A\\]]text [[image.png]]   \u0060\u0060\u0060\n%%[[A]]%%A\n---\n- [ ] [[A]]\n[^n]: [[A]]\n\n    [[B]]\n## [[A|alias]]\nÉ\n[[A#^a]] ^a\n ## A\n## A\n\t\u0060\u0060\u0060\n0 \u0060\u0060\u0060\n-->  > [!note] title\n![[A#A]][[A#^a]]  > [!note] title\nÉ\n> > \u0060\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A#^a]] ^é\n# A\n  A\n[[A\nB]][[A\\]]text [[image.png]]   \u0060\u0060\u0060\n%%[[A]]%%A\n---\n- [ ] [[A]]\n[^n]: [[A]]\n\n    [[B]]\n## [[A|alias]]\nÉ\n[[A#^a]] ^a\n ## A\n## A\n\t\u0060\u0060\u0060\n0 \u0060\u0060\u0060\n-->  > [!note] title\n![[A#A]][[A#^a]]  > [!note] title\nÉ\n> > \u0060\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-b-a-text-image-png-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%    \u0060\u0060\u0060\n%%## A\n## A\n> ## !\n~~~~\n- [x] [[B]]\n[[#A]]![[image.png]] ^é\n## A\n## A\n0%%[[A]]%%É\n<!-- [[A]] -->%%-->\u0060\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- > [!note] one\n> [!note] two\n> [!note] three\nA\n===\n\u0060open\n[[A]]\nclose\u0060A\n\u0060[[#A]][^n]: [[A]]\n\n    [[B]]\n[[image.png]][^n]: [[A]]\n\n    [[B]]\n<div>\n[[A]]\n</div>\n ^é\n## A\n> [!unknown] title\n\\\u0060[[A\\]]章節\n\\\\[[A]][[image.png]]# A\n\u0060open\n[[A]]\nclose\u0060## A\n## A\n<!--\n[[A]]\n-->| a | b |\n|---|---|\n| [[A]] | ^a |\n<div>\n[[A]]\n</div>\n![[A#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n  > [!note] title\nA\n===\n[[A\\|alias]]\u0060\u0060\u0060\u0060\n%%[[A]]%%[[A\nB]][[A\\]]A\n---\n<div>\n[[A]]\n</div>\n~~~\n> > \t\u0060\u0060\u0060\n\u0060\u0060^absent-prose [[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n  > [!note] title\nA\n===\n[[A\\|alias]]\u0060\u0060\u0060\u0060\n%%[[A]]%%[[A\nB]][[A\\]]A\n---\n<div>\n[[A]]\n</div>\n~~~\n> > \t\u0060\u0060\u0060\n\u0060\u0060^absent-prose [[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"alias-a-b-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^é\n ^é\n![[A#A]]%%[[A]]%%- > [!note] title\n0[[A]]| a | b |\n|---|---|\n| [[A]] | ^a |\n ^é\n- ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"é-é-a-a-note-title-0a-a-b-a-a-é\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^a\n## A\n## A\n\\\u0060%%[[image.png]]~~~\n## <em>A</em>\nB\n[[A]] [[A]]text [[A]] [[A]]  -    \u0060\u0060\u0060\n[[A]] [[A]]^absent-prose \u0060\u0060\u0060\u0060 go [[A]]\n> [!note] title\n\u0060open\n[[A]]\nclose\u0060## A-2\n## A\n## A\n[[B|alias]]\n[[A\\]]  - \t> [!note] one\n> [!note] two\n> [!note] three\n> [^n]: [[A]]\n\n    [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "1.  ^A\n\u0060\u0060\u0060\u0060 go [[A]]\n\u0060> [!note] title\n> [!note] [[A]]\n\u0060\u0060\u0060\u0060\n\\[[A]]## <em>A</em>\n\t![[image.png]]## A\n## A\n\t\u0060\u0060\u0060\n[[A]] [[A]]![[image.png]]A\n---\n> [!unknown] title\ntext ^absent-prose [[A\\]][[A\nB]]%%- [ ] [[A]]\nB\n![[image.png]]- [x] [[B]]\n1. [[A#^a]]\u0060\u0060\u0060\n [[A\nB]]## A-2\n## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-image-png-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] title\n    \u0060\u0060\u0060\n章節\n\u0060> [!note] title\nA\n---\n[[A]] [[A]]![[A]]    \u0060\u0060\u0060\n[[A]]> [!unknown] title\nhttps://example.invalid/\u0060[[A]]\u0060 | a | b |\n|---|---|\n| [[A]] | ^a |\n\u0060\u0060- [x] [[B]]\nB\n> [!note] [[A]]\n\u0060\u0060\u0060 [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"章節-note-title-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[A]]\u0060open\n[[A]]\nclose\u0060  - ~~~\n\t\u0060\u0060\u0060\n^absent-prose 0<!--\n[[A]]\n-->    \u0060\u0060\u0060\n  > [!note] title\n    \u0060\u0060\u0060\n[[A\\|alias]]A\n---\n章節\n \u0060\u0060\u0060\n       \u0060\u0060\u0060\n%%\u0060open\n[[A]]\nclose\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"aliasa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\\]]- item\n\n      [[A#A]]-->%%[[A]]%%~~~\n\u0060\t\u0060\u0060\u0060\n## A-2\n## A\n## A\n章節\n[[B|alias]]- [x] [[B]]\n<!--https://example.invalid/\u0060[[A]]\u0060 <!-- [[A]] -->## A\n## <em>A</em>\n%%[[A]]%%# A\n<!-- [[A]] -->0- item\n\n      [[A#A]]A\n---\n[[A\\|alias]][[A]]  -  ^a-2\nÉ\nhttps://example.invalid/\u0060[[A]]\u0060 ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\\]]- item\n\n      [[A#A]]-->%%[[A]]%%~~~\n\u0060\t\u0060\u0060\u0060\n## A-2\n## A\n## A\n章節\n[[B|alias]]- [x] [[B]]\n<!--https://example.invalid/\u0060[[A]]\u0060 <!-- [[A]] -->## A\n## <em>A</em>\n%%[[A]]%%# A\n<!-- [[A]] -->0- item\n\n      [[A#A]]A\n---\n[[A\\|alias]][[A]]  -  ^a-2\nÉ\nhttps://example.invalid/\u0060[[A]]\u0060 ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\\]]- item\n\n      [[A#A]]-->%%[[A]]%%~~~\n\u0060\t\u0060\u0060\u0060\n## A-2\n## A\n## A\n章節\n[[B|alias]]- [x] [[B]]\n<!--https://example.invalid/\u0060[[A]]\u0060 <!-- [[A]] -->## A\n## <em>A</em>\n%%[[A]]%%# A\n<!-- [[A]] -->0- item\n\n      [[A#A]]A\n---\n[[A\\|alias]][[A]]  -  ^a-2\nÉ\nhttps://example.invalid/\u0060[[A]]\u0060 ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "text \u0060\u0060\u0060\n## [[A|alias]]\n    \u0060\u0060\u0060\n## A\n## A\n    A\n[[A]]<!-- [[A]] -->\n\n^absent-prose   \u0060\u0060\u0060\n![[image.png]]- item\n\n      \t   \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 [[A]] [[A]]0\t\u0060\u0060\u0060\n<!-- [[A]] -->A\n---\n![[A#A]]\\[[A]]> ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"https-example-invalid-a-a-a0-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "# A\n- [ ] [[A]]\n\\\u0060[[#A]]    \u0060\u0060\u0060\n\n\n  \u0060\u0060\u0060\nA\n---\n[[image.png]]A\n---\n \u0060\u0060\u0060\nB\ntext https://example.invalid/\u0060[[A]]\u0060    \u0060\u0060\u0060\nÉ\n%%A\n===\n1.     \u0060\u0060\u0060\n## A-2\n## A\n## A\n[[A\\]]%%[[A]]%%- item\n\n      [[#A]]https://example.invalid/\u0060[[A]]\u0060 ~~~\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-png-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A-2\n## A\n## A\nA\nB\n[^n]: [[A]]\n\n    [[B]]\n<!--\n[[A]]\n-->## A\n## A\n\n^absent-prose %%[[A]]%%\u0060\u0060\u0060\u0060\n- > [!note] title\n> [!note] [[A]]\n# A\n[[B|alias]][[image.png]]\u0060[[A]]\u0060É\n- item\n\n      0   \u0060\u0060\u0060\n\n~~~\nA\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A-2\n## A\n## A\nA\nB\n[^n]: [[A]]\n\n    [[B]]\n<!--\n[[A]]\n-->## A\n## A\n\n^absent-prose %%[[A]]%%\u0060\u0060\u0060\u0060\n- > [!note] title\n> [!note] [[A]]\n# A\n[[B|alias]][[image.png]]\u0060[[A]]\u0060É\n- item\n\n      0   \u0060\u0060\u0060\n\n~~~\nA\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A-2\n## A\n## A\nA\nB\n[^n]: [[A]]\n\n    [[B]]\n<!--\n[[A]]\n-->## A\n## A\n\n^absent-prose %%[[A]]%%\u0060\u0060\u0060\u0060\n- > [!note] title\n> [!note] [[A]]\n# A\n[[B|alias]][[image.png]]\u0060[[A]]\u0060É\n- item\n\n      0   \u0060\u0060\u0060\n\n~~~\nA\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A-2\n## A\n## A\nA\nB\n[^n]: [[A]]\n\n    [[B]]\n<!--\n[[A]]\n-->## A\n## A\n\n^absent-prose %%[[A]]%%\u0060\u0060\u0060\u0060\n- > [!note] title\n> [!note] [[A]]\n# A\n[[B|alias]][[image.png]]\u0060[[A]]\u0060É\n- item\n\n      0   \u0060\u0060\u0060\n\n~~~\nA\nÉ\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-6\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!unknown] title\n%%[[A]]%%## [[A|alias]]\n- item\n\n      [^unused]: [[A]]\n\\[[A]]A\n---\nB\n    \u0060\u0060\u0060\n> [!note] [[A]]\n[^n]: [[A]]\n\n    [[B]]\nA\n===\nhttps://example.invalid/\u0060[[A]]\u0060 [[A\nB]][[A#A]]   \u0060\u0060\u0060\n\\\\[[A]]    \u0060\u0060\u0060\n- item\n\n      > [!note] [[A]]\n0> [!note] [[A]]\n  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[^n]: [[A]]\n\n    [[B]]\n> [!note] title\n[[image.png]]A\n---\n> %%<!--\u0060open\n[[A]]\nclose\u0060\n\n-   - \u0060[[A]]\u0060![[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-pnga\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\t   \u0060\u0060\u0060\n\\\\[[A]]\u0060\u0060\u0060 [[A]]\n[[A]] [[A]]A\n- [x] [[B]]\n> [!note] title\n\\\u0060 A\n---\n    \u0060\u0060\u0060\n\\\\[[A]]É\n0![[image.png]]   \u0060\u0060\u0060\n\\\u0060章節\n## !\n\\\\[[A]]\u0060open\n[[A]]\nclose\u0060A\nA\n---\n ^a-2\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\t   \u0060\u0060\u0060\n\\\\[[A]]\u0060\u0060\u0060 [[A]]\n[[A]] [[A]]A\n- [x] [[B]]\n> [!note] title\n\\\u0060 A\n---\n    \u0060\u0060\u0060\n\\\\[[A]]É\n0![[image.png]]   \u0060\u0060\u0060\n\\\u0060章節\n## !\n\\\\[[A]]\u0060open\n[[A]]\nclose\u0060A\nA\n---\n ^a-2\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"aopen-a-closea-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "    0%%[[A]]%%## [[A|alias]]\n%%[[A]]%%![[A#A]]A\n===\n    <div>\n[[A]]\n</div>\n\t\t\u0060\u0060\u0060\n ^a\n[[#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  \u0060\u0060\u0060\n-->[[A\\|alias]]- [ ] [[A]]\n## [[A|alias]]\n- [ ] [[A]]\n## A-2\n## A\n## A\n-->\u0060\u0060\u0060\u0060\n> [!unknown] title\n章節\n> [!note] [[A]]\n\u0060\u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 <!--\n[[A]]\n-->B\n> [!note] one\n> [!note] two\n> [!note] three\n[[A\nB]]<!--\n[[A]]\n-->## A-2\n## A\n## A\n- > [!note] title\n[[A\\|alias]]\u0060\u0060\u0060 [[A]]\n   \u0060\u0060\u0060\n\u0060open\n[[A]]\nclose\u00600[[A\\]][[image.png]]ref[^n]\n ^é\nA\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  \u0060\u0060\u0060\n-->[[A\\|alias]]- [ ] [[A]]\n## [[A|alias]]\n- [ ] [[A]]\n## A-2\n## A\n## A\n-->\u0060\u0060\u0060\u0060\n> [!unknown] title\n章節\n> [!note] [[A]]\n\u0060\u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 <!--\n[[A]]\n-->B\n> [!note] one\n> [!note] two\n> [!note] three\n[[A\nB]]<!--\n[[A]]\n-->## A-2\n## A\n## A\n- > [!note] title\n[[A\\|alias]]\u0060\u0060\u0060 [[A]]\n   \u0060\u0060\u0060\n\u0060open\n[[A]]\nclose\u00600[[A\\]][[image.png]]ref[^n]\n ^é\nA\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"open-a-close0-a-image-png-ref-n-é-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!--\n[[A]]\n-->## A-2\n## A\n## A\nref[^n]\n> [!unknown] title\n\tA\n---\n> [!note] [[A]]\n~~~\n## !\n\\\u00601. - > [!note] title\n0> [!note] one\n> [!note] two\n> [!note] three\n- item\n\n      > [!unknown] title\n- [ ] [[A]]\n\u0060\u0060\u0060\u0060\n\thttps://example.invalid/\u0060[[A]]\u0060  ^a-2\nÉ\n<!--",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  > [!note] title\n<div>\n[[A]]\n</div>\n  > [!note] title\n- > [!note] title\n[[A#A]]## <em>A</em>\n[[A#^a]][[A\nB]]## !\n    ## A-2\n## A\n## A\nÉ\n# A\n- [ ] [[A]]\n## A\n[[A\\]] ^a\n\u0060\u0060\u0060\u0060 go [[A]]\n  \u0060\u0060\u0060\n\u0060\u0060\\\\[[A]]- [ ] [[A]]\n\n![[image.png]]> [!note] one\n> [!note] two\n> [!note] three\n<!-- \u0060\u0060\u0060\n\\[[A]]\t%%<!--",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  > [!note] title\n<div>\n[[A]]\n</div>\n  > [!note] title\n- > [!note] title\n[[A#A]]## <em>A</em>\n[[A#^a]][[A\nB]]## !\n    ## A-2\n## A\n## A\nÉ\n# A\n- [ ] [[A]]\n## A\n[[A\\]] ^a\n\u0060\u0060\u0060\u0060 go [[A]]\n  \u0060\u0060\u0060\n\u0060\u0060\\\\[[A]]- [ ] [[A]]\n\n![[image.png]]> [!note] one\n> [!note] two\n> [!note] three\n<!-- \u0060\u0060\u0060\n\\[[A]]\t%%<!--",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  > [!note] title\n<div>\n[[A]]\n</div>\n  > [!note] title\n- > [!note] title\n[[A#A]]## <em>A</em>\n[[A#^a]][[A\nB]]## !\n    ## A-2\n## A\n## A\nÉ\n# A\n- [ ] [[A]]\n## A\n[[A\\]] ^a\n\u0060\u0060\u0060\u0060 go [[A]]\n  \u0060\u0060\u0060\n\u0060\u0060\\\\[[A]]- [ ] [[A]]\n\n![[image.png]]> [!note] one\n> [!note] two\n> [!note] three\n<!-- \u0060\u0060\u0060\n\\[[A]]\t%%<!--",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\n![[A#A]]\\\\[[A]]\\[[A]][[B|alias]]A\n===\n~~~~\n ^é\n## A\n| a | b |\n|---|---|\n| [[A]] | ^a |\nÉ\n%% ^é\nA\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-a-a-aliasa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] title\n[[A\nB]]A\n===\n \u0060\u0060\u0060\n ^a-2\n0\u0060\u0060 ^é\n- [ ] [[A]]\ntext %%[[A]]%%É\n~~~~\ntext ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-b-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^é\n    \u0060\u0060\u0060\n ^é\n    - [x] [[B]]\n%%[[A]]%%## A-2\n## A\n## A\n ^a-2\n[[A\\]][[B|alias]]<!--\n[[A]]\n-->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!--\n[[A]]\n-->## A-2\n## A\n## A\n0https://example.invalid/\u0060[[A]]\u0060 \u0060\u0060\u0060\u0060 go [[A]]\n<!-- [[A]] -->> [!note] one\n> [!note] two\n> [!note] three\n~~~~\n\t\u0060\u0060\u0060\n- [ ] [[A]]\n\t[[A]] [[A]]\u0060[[A]]\u0060 ^A\n A\n[[A]]É\n\n## !\n[[#A]]^absent-prose <!--\n[[A]]\n-->\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- [x] [[B]]\n> \\[[A]]^absent-prose ## [[A|alias]]\nÉ\n- [[A]] [[A]]~~~\nA\n===\nA\n===\n## A\n [[A]] [[A]]A\n===\n\n\nA\n---\n\u0060\u0060## <em>A</em>\n[[A\nB]]![[A#A]]\t[^unused]: [[A]]\n## <em>A</em>\nhttps://example.invalid/\u0060[[A]]\u0060 [[A\\]]\u0060\u0060\u0060 [[A]]\nA\n---\n%%[[A]]%%A\n---\n> [!note] one\n> [!note] two\n> [!note] three\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- [x] [[B]]\n> \\[[A]]^absent-prose ## [[A|alias]]\nÉ\n- [[A]] [[A]]~~~\nA\n===\nA\n===\n## A\n [[A]] [[A]]A\n===\n\n\nA\n---\n\u0060\u0060## <em>A</em>\n[[A\nB]]![[A#A]]\t[^unused]: [[A]]\n## <em>A</em>\nhttps://example.invalid/\u0060[[A]]\u0060 [[A\\]]\u0060\u0060\u0060 [[A]]\nA\n---\n%%[[A]]%%A\n---\n> [!note] one\n> [!note] two\n> [!note] three\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[^unused]: [[A]]\n<!--\u0060\u0060\u0060\u0060\n## !\n章節\n0<!--\n[[A]]\n-->## [[A|alias]]\nref[^n]\n[[image.png]]    \u0060\u0060\u0060\n> > ## [[A|alias]]\n>   \u0060\u0060\u0060\n<!--\n[[A]]\n-->\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"alias-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[A#A]]~~~~\n> [!note] one\n> [!note] two\n> [!note] three\n<!--\n[[A]]\n-->## A\n## A\n-->[[A\\]] \u0060\u0060\u0060\n\u0060open\n[[A]]\nclose\u00600<!--\n[[A]]\n--><div>\n[[A]]\n</div>\n \n    \u0060\u0060\u0060\n\u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- ## A-2\n## A\n## A\n## A-2\n## A\n## A\n\\[[A]]%%[[A]]%% 0    \u0060\u0060\u0060\nA\n---\n![[A]]- [x] [[B]]\n   \u0060\u0060\u0060\n%%| a | b |\n|---|---|\n| [[A]] | ^a |\n[[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- ## A-2\n## A\n## A\n## A-2\n## A\n## A\n\\[[A]]%%[[A]]%% 0    \u0060\u0060\u0060\nA\n---\n![[A]]- [x] [[B]]\n   \u0060\u0060\u0060\n%%| a | b |\n|---|---|\n| [[A]] | ^a |\n[[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- ## A-2\n## A\n## A\n## A-2\n## A\n## A\n\\[[A]]%%[[A]]%% 0    \u0060\u0060\u0060\nA\n---\n![[A]]- [x] [[B]]\n   \u0060\u0060\u0060\n%%| a | b |\n|---|---|\n| [[A]] | ^a |\n[[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- ## A-2\n## A\n## A\n## A-2\n## A\n## A\n\\[[A]]%%[[A]]%% 0    \u0060\u0060\u0060\nA\n---\n![[A]]- [x] [[B]]\n   \u0060\u0060\u0060\n%%| a | b |\n|---|---|\n| [[A]] | ^a |\n[[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] title\n> %%[[A]]%%![[image.png]]É\nÉ\n\u0060\u0060[[A\\|alias]]  -  ^é\nA\n-->[[#A]]0| a | b |\n|---|---|\n| [[A]] | ^a |\nA\n---\n1. ref[^n]\n[[B|alias]]章節\n\t[[#A]]<!--![[A#A]]| a | b |\n|---|---|\n| [[A]] | ^a |\n## !\n\u0060\u0060\u0060 [[A]]\n\u0060\u0060\u0060\u0060\n  - É\nA\n===\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"é-alias-é-a-a0-a-b-a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[#A]]\t\u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 B\n> [!note] title\n ^a\n[[A\\]]A\n===\n<!--\n[[A]]\n-->## A\n~~~\n[[A#A]]\u0060\u0060\u0060\u0060\n| a | b |\n|---|---|\n| [[A]] | ^a |\n<!--\\\\[[A]]~~~\n\u0060\u0060\u0060\u0060\n[^n]: [[A]]\n\n    [[B]]\n> [!note] title\n  > [!note] title\n> [!note] title\n [[#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A\n## A\nÉ\n> [!note] one\n> [!note] two\n> [!note] three\n\n%%[[A]]%%A\n<!--> [!note] [[A]]\n\\\u0060<!-- [[A]] -->%%[[A]]%%A\n===\n- \u0060\u0060\u0060\u0060 [[A]]\n- ![[A#A]]> \u0060\u0060\u0060 [[A]]\n- [x] [[B]]\nÉ\nB\n- [x] [[B]]\n\u0060[[A]]\u0060\n\n- [ ] [[A]]\n  > [!note] title\n\u0060\u0060https://example.invalid/\u0060[[A]]\u0060 -->- item\n\n      [[A#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->## A-2\n## A\n## A\n  \u0060\u0060\u0060\n## [[A|alias]]\n\\\\[[A]]## [[A|alias]]\n> [!note] one\n> [!note] two\n> [!note] three\n  - \u0060\u0060\u0060\u0060 go [[A]]\n    \u0060\u0060\u0060\n\t\u0060~~~~\n ^A\nA\n---\n## A\n## A\nref[^n]\n\t   \u0060\u0060\u0060\n>   > [!note] title\n0![[A]]![[A#A]] ^A\n  > [!note] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n<!--\n[[A]]\n--> ^é\n> [!note] title\n[[A\nB]]^absent-prose  ^A\nÉ\n| a | b |\n|---|---|\n| [[A]] | ^a |\n  \u0060\u0060\u0060\n\u0060\u0060\u0060\u0060 go [[A]]\n\u0060[[A]]\u0060~~~~\nhttps://example.invalid/\u0060[[A]]\u0060 [[#A]]\n\n \u0060\u0060\u0060\n<!-- [[A]] -->A\n---\nref[^n]\n> [!note] title\n\t\u0060\u0060\u0060\n- [x] [[B]]\n# A\n![[A]]\u0060open\n[[A]]\nclose\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A]] [[A]] \u0060\u0060\u0060\n## [[A|alias]]\n\\[[A]][[A]] [[A]]\u0060\u0060[[A\\|alias]]- item\n\n      [[image.png]][[A#^a]]## A\n## A\n> > > ~~~~\nB\n[[A]]- [x] [[B]]\n<!--\n[[A]]\n-->## [[A|alias]]\n- [x] [[B]]\nref[^n]\n<!--## !\n> [!note] [[A]]\n[^unused]: [[A]]\n<div>\n[[A]]\n</div>\n  - [^unused]: [[A]]\n\\\\[[A]]## A\n## A\n\u0060\u0060\u0060 [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"alias-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%É\n\\\\[[A]]-->^absent-prose A\n===\n- > [!note] title\n[[A]][[A]] ^é\n\n<div>\n[[A]]\n</div>\n\u0060open\n[[A]]\nclose\u0060~~~~\n[[A\\|alias]]%%https://example.invalid/\u0060[[A]]\u0060 <!--\n[[A]]\n-->B\n## A\n## A\n## [[A|alias]]\n0 ^A\nA\n---\n- [[A\\]]> [!note] title\n[^unused]: [[A]]\n0\u0060\u0060\u0060\n\u0060[[A]]\u0060章節\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "    \u0060\u0060\u0060\n## !\n ^a-2\n-->-->\u0060\u0060\u0060\n[[A#A]]~~~\n## A\n0 ^a-2\n<!--\\[[A]]text A\n ^A\nA\n\n\n[[image.png]]É\n[[A]]\n![[image.png]]  \u0060\u0060\u0060\n1. <!-- [[A]] -->A\n---\n[^n]: [[A]]\n\n    [[B]]\n\u0060\u0060\u0060\u0060 go [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[A]]ref[^n]\n[[#A]]-->## !\n<!--\n[[A]]\n-->## A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "0~~~\nA\n ^é\n## A\n> [!unknown] title\nA\n===\n> -->[[image.png]]- item\n\n      ## [[A|alias]]\n![[image.png]]\n\n![[A#A]]\t\u0060\u0060\u0060\n ^A\n-   > [!note] title\n--> \u0060\u0060\u0060\n<!-- [[A]] -->[[A\nB]]> >  ^a-2\n# A\n  \u0060\u0060\u0060\n[[A]] [[A]]\u0060[[B|alias]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "0~~~\nA\n ^é\n## A\n> [!unknown] title\nA\n===\n> -->[[image.png]]- item\n\n      ## [[A|alias]]\n![[image.png]]\n\n![[A#A]]\t\u0060\u0060\u0060\n ^A\n-   > [!note] title\n--> \u0060\u0060\u0060\n<!-- [[A]] -->[[A\nB]]> >  ^a-2\n# A\n  \u0060\u0060\u0060\n[[A]] [[A]]\u0060[[B|alias]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060open\n[[A]]\nclose\u0060## !\n\\\\[[A]]ref[^n]\nÉ\n- item\n\n      <!--\t\u0060\u0060\u0060\n- [x] [[B]]\nA\n---\nÉ\n[[image.png]]~~~~\n  -   \u0060\u0060\u0060\n0A\n---\n\u0060\u0060\u0060 [[A]]\n^absent-prose É\n<div>\n[[A]]\n</div>\n ^a\n      \u0060\u0060\u0060\n> [!note] title\n- [ ] [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\\|alias]]\u0060open\n[[A]]\nclose\u0060A\n---\n- [[A]] [[A]]\u0060\u0060\u0060 [[A]]\n- %%[[A]]%%É\nhttps://example.invalid/\u0060[[A]]\u0060 - [ ] [[A]]\n  ^a\n\u0060\u0060 ^é\n  > [!note] title\nref[^n]\n> [!note] [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"aliasopen-a-closea\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "0A\n---\n\u0060open\n[[A]]\nclose\u0060<!-- [[A]] -->\\\\[[A]]> [!unknown] title\n[[A#A]]## [[A|alias]]\n   \u0060\u0060\u0060\n\u0060[[A]]\u0060  -  ^a\n> [!note] [[A]]\n[[#A]]01. \t\u0060\u0060\u0060\n[[A\nB]]章節\n1. [[A#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^é\n\t\u0060\u0060\u0060\nB\n\u0060open\n[[A]]\nclose\u0060[[A\\|alias]][[A\\]]![[A#A]] ^é\n![[image.png]]  > [!note] title\n A\n---\n\u0060\u0060\u0060 [[A]]\n章節\n  \u0060\u0060\u0060\n\u0060\u0060\u0060\u0060 go [[A]]\n\u0060\u0060\u0060\u0060 go [[A]]\n\u0060\u0060\u0060\u0060 go [[A]]\nB\n> > \t[[A#A]]%%[[A]]%%~~~\n[[A\\|alias]]\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"é-b-open-a-closealiasa-a-a-é-image-png-note-title-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\\u0060  > [!note] title\n0- # A\n  - ## !\n[[A#^a]]<!--\n[[A]]\n-->## !\n \u0060\u0060\u0060\n0[[A#^a]][^unused]: [[A]]\n章節\n![[image.png]]## A\n%%[[A]]%%text [^n]: [[A]]\n\n    [[B]]\n章節\nÉ\n> [!note] one\n> [!note] two\n> [!note] three\n> \u0060\u0060\u0060 [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n<!--\n[[A]]\n-->[[#A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"section-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\nB]]É\n![[A]]\u0060<!--\n[[A]]\n-->[[image.png]]![[A#A]]A\n===\n## A\n## A\n\u0060\u0060\u0060 [[A]]\nhttps://example.invalid/\u0060[[A]]\u0060 - > [!note] title\n章節\n-->É\n> [!note] one\n> [!note] two\n> [!note] three\n%%[[A]]%%https://example.invalid/\u0060[[A]]\u0060  ^a\n- [ ] [[A]]\n\u0060\u0060\u0060 [[A]]\n- [ ] [[A]]\n## [[A|alias]]\n    \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-png-a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] --><!--\n[[A]]\n--> ![[A#A]]ref[^n]\n\t%%[[A]]%%A\n---\n## !\n\n\n\\[[A]]-->\u0060open\n[[A]]\nclose\u0060 ^A\n <!-- [[A]] -->## A-2\n## A\n## A\n\\\\[[A]][^unused]: [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] --><!--\n[[A]]\n--> ![[A#A]]ref[^n]\n\t%%[[A]]%%A\n---\n## !\n\n\n\\[[A]]-->\u0060open\n[[A]]\nclose\u0060 ^A\n <!-- [[A]] -->## A-2\n## A\n## A\n\\\\[[A]][^unused]: [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-ref-n-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "text <!-- [[A]] -->É\n## [[A|alias]]\n\\\u00600[[A\\|alias]]- item\n\n       ^a-2\n0[[A\\]]> [!note] [[A]]\nB\n ^a\n\\\u0060B\nA\n---\n ^A\n[[B|alias]]> [!note] title\n%%[[A]]%%![[A#A]]> [!note] [[A]]\nA\n## <em>A</em>\n%%[[A]]%%\\\\[[A]]## [[A|alias]]\n\u0060\u0060\u0060 [[A]]\n> > ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0a-note-a-b-a-b-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  - 0\n\n![[image.png]]    \t\u0060\u0060\u0060\n\n\n> [!note] [[A]]\nA\n---\n--><div>\n[[A]]\n</div>\n![[A]]\u0060\u0060\u0060 [[A]]\n\u0060\u0060章節\n%% ^a\n\u0060\u0060\u0060\u0060- [ ] [[A]]\n\n\n\n\n<!-- [[A]] -->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n> [!note] title\n-->~~~\n\\\u0060https://example.invalid/\u0060[[A]]\u0060 <!--\n[[A]]\n-->- # A\n^absent-prose  \u0060\u0060\u0060\n# A\n\u0060\u0060\u0060\u0060\n![[A#A]]^absent-prose  ^A\n章節\n\u0060[[A]]\u00600> > <!--\n[[A]]\n-->[[A]] [[A]] [^n]: [[A]]\n\n    [[B]]\n## [[A|alias]]\nB\n[^unused]: [[A]]\nA\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  > [!note] title\n> [!note] [[A]]\nhttps://example.invalid/\u0060[[A]]\u0060 ~~~~\n[[A#^a]]0      > [!note] title\n> > ![[image.png]]~~~~\n章節\n> [!unknown] title\n[^unused]: [[A]]\n- [x] [[B]]\n\n\u0060open\n[[A]]\nclose\u0060<!-- [[A]] -->%%[[A]]%%\t> [!note] one\n> [!note] two\n> [!note] three\n ^é\n[[#A]]  > [!note] title\n\u0060open\n[[A]]\nclose\u0060 A\n---\nA\n---\n## [[A|alias]]\n# A\n- [ ] [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  > [!note] title\n> [!note] [[A]]\nhttps://example.invalid/\u0060[[A]]\u0060 ~~~~\n[[A#^a]]0      > [!note] title\n> > ![[image.png]]~~~~\n章節\n> [!unknown] title\n[^unused]: [[A]]\n- [x] [[B]]\n\n\u0060open\n[[A]]\nclose\u0060<!-- [[A]] -->%%[[A]]%%\t> [!note] one\n> [!note] two\n> [!note] three\n ^é\n[[#A]]  > [!note] title\n\u0060open\n[[A]]\nclose\u0060 A\n---\nA\n---\n## [[A|alias]]\n# A\n- [ ] [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"é-a-note-title-open-a-close-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- > [!note] title\n[^unused]: [[A]]\nÉ\n  > [!note] title\n[[A]][[A#A]]A\n===\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\u0060\u0060\t> - > [!note] title\n- item\n\n      0\u0060A\n===\n[[A\\]]A\nA\n---\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!unknown] title\n\u0060\u0060\u0060\u0060\nA\n===\n~~~~\n\u0060https://example.invalid/\u0060[[A]]\u0060 ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"aa-aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\\\[[A]]    > \n  - ref[^n]\n## !\n- [ ] [[A]]\n%%[[A]]%%## A-2\n## A\n## A\n0\u0060\u0060\u0060\n ^é\n    1. \t\u0060\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> > ## !\n![[image.png]]> [!note] one\n> [!note] two\n> [!note] three\n-->\u0060## <em>A</em>\n  - https://example.invalid/\u0060[[A]]\u0060  \u0060\u0060\u0060\n> [!note] title\n    \u0060\u0060\u0060\n\t## [[A|alias]]\n    \u0060\u0060\u0060\n[[A]]章節\n0~~~\n> [!note] title\nA\n===\n[[image.png]]\u0060\u0060\u0060\n## A\n## A\n- > [!note] title\n[[A\nB]][[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> > ## !\n![[image.png]]> [!note] one\n> [!note] two\n> [!note] three\n-->\u0060## <em>A</em>\n  - https://example.invalid/\u0060[[A]]\u0060  \u0060\u0060\u0060\n> [!note] title\n    \u0060\u0060\u0060\n\t## [[A|alias]]\n    \u0060\u0060\u0060\n[[A]]章節\n0~~~\n> [!note] title\nA\n===\n[[image.png]]\u0060\u0060\u0060\n## A\n## A\n- > [!note] title\n[[A\nB]][[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n ^a\n## A\n## A\n  -  ^é\n<div>\n[[A]]\n</div>\n- [x] [[B]]\n    ## [[A|alias]]\n[[A\\|alias]]## <em>A</em>\n[[A\nB]]## A\n章節\n> [!note] [[A]]\n![[A]]\u0060## A\n## A\n[[A#A]] ^A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n ^a\n## A\n## A\n  -  ^é\n<div>\n[[A]]\n</div>\n- [x] [[B]]\n    ## [[A|alias]]\n[[A\\|alias]]## <em>A</em>\n[[A\nB]]## A\n章節\n> [!note] [[A]]\n![[A]]\u0060## A\n## A\n[[A#A]] ^A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\n\u0060open\n[[A]]\nclose\u0060[[A#A]]<!--[[A#^a]]  \u0060\u0060\u0060\n> > <!-- [[A]] -->A\n===\n[^n]: [[A]]\n\n    [[B]]\n\u0060\u0060\u0060\u0060 go [[A]]\n[^unused]: [[A]]\n## !\n## !\nA\n\n\n~~~~\n\u0060\u0060\u0060\u0060 go [[A]]\n- [x] [[B]]\n[[A]]%%[[A]]%%",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"open-a-closea-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\t\u0060[[A]]\u0060[[A#^a]]É\nA\n===\n\n\n%%[[A]]%%## A\n## A\n## <em>A</em>\n0 ^é\n[[A\\|alias]]> [[A\\]] ^a\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\t\u0060[[A]]\u0060[[A#^a]]É\nA\n===\n\n\n%%[[A]]%%## A\n## A\n## <em>A</em>\n0 ^é\n[[A\\|alias]]> [[A\\]] ^a\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->## A-2\n## A\n## A\n%%- [[A#A]][[A#^a]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- item\n\n      > [!note] [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n![[A]]A\n---\n![[image.png]]\t ^a\n- [ ] [[A]]\n ^a-2\n ^A\n- item\n\n      ## !\n## A-2\n## A\n## A\n<div>\n[[A]]\n</div>\n<div>\n[[A]]\n</div>\n  \u0060\u0060\u0060\n~~~\nA\n\\[[A]]\u0060\u0060\u0060 [[A]]\nÉ\n> [!note] [[A]]\n\u0060\u0060\u0060\u0060 go [[A]]\n[[A#A]][[A\nB]]\t\u0060\u0060\u0060\n[[#A]]1. https://example.invalid/\u0060[[A]]\u0060 ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "- item\n\n      > [!note] [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n![[A]]A\n---\n![[image.png]]\t ^a\n- [ ] [[A]]\n ^a-2\n ^A\n- item\n\n      ## !\n## A-2\n## A\n## A\n<div>\n[[A]]\n</div>\n<div>\n[[A]]\n</div>\n  \u0060\u0060\u0060\n~~~\nA\n\\[[A]]\u0060\u0060\u0060 [[A]]\nÉ\n> [!note] [[A]]\n\u0060\u0060\u0060\u0060 go [[A]]\n[[A#A]][[A\nB]]\t\u0060\u0060\u0060\n[[#A]]1. https://example.invalid/\u0060[[A]]\u0060 ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A-2\n## A\n## A\n^absent-prose  \u0060\u0060\u0060\n> [!unknown] title\n- B\n    <div>\n[[A]]\n</div>\n\t<!--\n[[A]]\n--> ^A\n ^é\n## A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A-2\n## A\n## A\n^absent-prose  \u0060\u0060\u0060\n> [!unknown] title\n- B\n    <div>\n[[A]]\n</div>\n\t<!--\n[[A]]\n--> ^A\n ^é\n## A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "%%> [!unknown] title\n  - ## A\n ^a-2\n\u0060[[A]]\u0060> [!note] title\n%%\u0060open\n[[A]]\nclose\u0060ref[^n]\n[^n]: [[A]]\n\n    [[B]]\n## A\n## A\n\\[[A]][[#A]]- item\n\n      | a | b |\n|---|---|\n| [[A]] | ^a |\n> [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060\u0060\u0060 go [[A]]\n## !\n![[A#A]][[A]] [[A]]\u0060open\n[[A]]\nclose\u0060ref[^n]\n-->[[A\\]]É\n## !\n<div>\n[[A]]\n</div>\n\n1. - > [!note] title\n| a | b |\n|---|---|\n| [[A]] | ^a |\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  -     \u0060\u0060\u0060\n\\[[A]]https://example.invalid/\u0060[[A]]\u0060 1. > > [^n]: [[A]]\n\n    [[B]]\n       \u0060\u0060\u0060\n> [!note] title\n%%[[A]]%%^absent-prose A\n===\n## A\nA\n===\n    \u0060\u0060\u0060\nA\n---\n0## !\n%%[[A]]%%~~~\n\t\u0060\u0060\u0060\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A#^a]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  -     \u0060\u0060\u0060\n\\[[A]]https://example.invalid/\u0060[[A]]\u0060 1. > > [^n]: [[A]]\n\n    [[B]]\n       \u0060\u0060\u0060\n> [!note] title\n%%[[A]]%%^absent-prose A\n===\n## A\nA\n===\n    \u0060\u0060\u0060\nA\n---\n0## !\n%%[[A]]%%~~~\n\t\u0060\u0060\u0060\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A#^a]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  -     \u0060\u0060\u0060\n\\[[A]]https://example.invalid/\u0060[[A]]\u0060 1. > > [^n]: [[A]]\n\n    [[B]]\n       \u0060\u0060\u0060\n> [!note] title\n%%[[A]]%%^absent-prose A\n===\n## A\nA\n===\n    \u0060\u0060\u0060\nA\n---\n0## !\n%%[[A]]%%~~~\n\t\u0060\u0060\u0060\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A#^a]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"absent-prose-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\n A\n===\nA\n---\n    ## A-2\n## A\n## A\n## <em>A</em>\n \u0060\u0060\u0060\nref[^n]\n\\[[A]]B\n<!-- [[A]] -->[[A\\|alias]]<!-- [[A]] -->https://example.invalid/\u0060[[A]]\u0060 \u0060[[A]]\u0060<!-- [[A]] -->[[A\\]]  > [!note] title\n[[B|alias]]  > [!note] title\n> [!note] one\n> [!note] two\n> [!note] three\n<!-- [[A]] -->0[[B|alias]][[A\\|alias]][^unused]: [[A]]\n  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\n A\n===\nA\n---\n    ## A-2\n## A\n## A\n## <em>A</em>\n \u0060\u0060\u0060\nref[^n]\n\\[[A]]B\n<!-- [[A]] -->[[A\\|alias]]<!-- [[A]] -->https://example.invalid/\u0060[[A]]\u0060 \u0060[[A]]\u0060<!-- [[A]] -->[[A\\]]  > [!note] title\n[[B|alias]]  > [!note] title\n> [!note] one\n> [!note] two\n> [!note] three\n<!-- [[A]] -->0[[B|alias]][[A\\|alias]][^unused]: [[A]]\n  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "É\n ^a-2\n ^a\n\u0060\u0060[[A#^a]][^unused]: [[A]]\n  > [!note] title\nA\n---\n[[image.png]]%%> [!note] [[A]]\n~~~~\n> [!note] one\n> [!note] two\n> [!note] three\n- [ ] [[A]]\n[[#A]]\u0060open\n[[A]]\nclose\u0060<!-- [[A]] -->A\n ^a-2\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A]] [[A]]\u0060open\n[[A]]\nclose\u0060 \u0060\u0060\u0060\n![[image.png]]~~~~\n[[#A]]^absent-prose ~~~\n%%[[A]]%% \u0060\u0060\u0060\n\u0060\u0060\u0060\u0060\n## A-2\n## A\n## A\n[^unused]: [[A]]\n[[A\nB]]%%[[A]]%%\u0060\u0060\u0060\u0060\n\\[[A]][[image.png]]<!--\\\\[[A]]-->![[A#A]]\u0060\u0060\u0060\u0060\n> [[A#A]]\t[[image.png]]## !\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[A]]  \u0060\u0060\u0060\n<!-- [[A]] -->## !\n ^é\n  - ## <em>A</em>\n ^a-2\n## <em>A</em>\n[[A#^a]]\n\n> [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060<div>\n[[A]]\n</div>\n\t\u0060\u0060\u0060\nÉ\n\u0060\u0060\u0060\u0060\n    \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"em-a-em-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n\n[[B|alias]] ^é\n> [!unknown] title\n%%[[A]]%%<div>\n[[A]]\n</div>\n![[A]]~~~~\n\u0060open\n[[A]]\nclose\u0060## <em>A</em>\n## !\n<div>\n[[A]]\n</div>\n## A\n## !\nA\n---\n\t ^é\n> [!note] [[A]]\n## A\n## A\n> [!note] title\n ^é\n章節\n \u0060\u0060\u0060\n[[A\\]][^unused]: [[A]]\n\n-  ^a-2\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060open\n[[A]]\nclose\u0060# A\n\\\\[[A]]%%[[A]]%%![[A]]\u0060\u0060\u0060\n\\[[A]]- [ ] [[A]]\n1. B\nB\n<!--\n[[A]]\n--><!--\n[[A]]\n-->  \u0060\u0060\u0060\n> [!unknown] title\n\u0060open\n[[A]]\nclose\u0060\u0060[[A]]\u0060 \u0060\u0060\u0060\u0060 go [[A]]\n[[B|alias]]É\n[[A\\|alias]]1. \\\u0060\nÉ\n\u0060\u0060\u0060\u0060\n## A\n## A\n> [!note] [[A]]\nA\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!--\n  > [!note] title\n[[A\\|alias]]> [!note] title\n ^é\n[^unused]: [[A]]\n  \u0060\u0060\u0060\n%%-->- ## A-2\n## A\n## A\n\\\u0060",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!--\n[[A]]\n-->   \u0060\u0060\u0060\n  \u0060\u0060\u0060\n> > > >  ^A\n[[A]]   \u0060\u0060\u0060\n- > [!note] title\n## A-2\n## A\n## A\n     ~~~~\n\t\u0060\u0060\u0060\n\u0060\u0060\u0060 [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!--\n[[A]]\n-->[[B|alias]]## A-2\n## A\n## A\n## [[A|alias]]\n章節\n\\\\[[A]]<!--[[A#^a]][[A\nB]]## A-2\n## A\n## A\n ^é\n> > [[image.png]]0A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n\\\u0060![[image.png]]  -  ^a\n## A\n## A\n ^é\n0![[A]]B\n> [!note] title\n    \u0060\u0060\u0060\n## A-2\n## A\n## A\n[[A]]   \u0060\u0060\u0060\n ^é\n## A\n## A\n[[A\\]] %%[[A]]%%   \u0060\u0060\u0060\nA\n ^a-2\n    \u0060\u0060\u0060\nA\n---\n![[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n\\\u0060![[image.png]]  -  ^a\n## A\n## A\n ^é\n0![[A]]B\n> [!note] title\n    \u0060\u0060\u0060\n## A-2\n## A\n## A\n[[A]]   \u0060\u0060\u0060\n ^é\n## A\n## A\n[[A\\]] %%[[A]]%%   \u0060\u0060\u0060\nA\n ^a-2\n    \u0060\u0060\u0060\nA\n---\n![[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n\\\u0060![[image.png]]  -  ^a\n## A\n## A\n ^é\n0![[A]]B\n> [!note] title\n    \u0060\u0060\u0060\n## A-2\n## A\n## A\n[[A]]   \u0060\u0060\u0060\n ^é\n## A\n## A\n[[A\\]] %%[[A]]%%   \u0060\u0060\u0060\nA\n ^a-2\n    \u0060\u0060\u0060\nA\n---\n![[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n\\\u0060![[image.png]]  -  ^a\n## A\n## A\n ^é\n0![[A]]B\n> [!note] title\n    \u0060\u0060\u0060\n## A-2\n## A\n## A\n[[A]]   \u0060\u0060\u0060\n ^é\n## A\n## A\n[[A\\]] %%[[A]]%%   \u0060\u0060\u0060\nA\n ^a-2\n    \u0060\u0060\u0060\nA\n---\n![[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\n\\\u0060![[image.png]]  -  ^a\n## A\n## A\n ^é\n0![[A]]B\n> [!note] title\n    \u0060\u0060\u0060\n## A-2\n## A\n## A\n[[A]]   \u0060\u0060\u0060\n ^é\n## A\n## A\n[[A\\]] %%[[A]]%%   \u0060\u0060\u0060\nA\n ^a-2\n    \u0060\u0060\u0060\nA\n---\n![[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-6\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "É\n## A\n## A\nA\n\u0060open\n[[A]]\nclose\u0060\\\\[[A]]# A\n- item\n\n      \u0060open\n[[A]]\nclose\u0060[^n]: [[A]]\n\n    [[B]]\n> > ![[A]]<!--## A-2\n## A\n## A\n[[A\\|alias]]  - # A\n\u0060\u0060~~~~\n[[image.png]][[A\\]][^unused]: [[A]]\n<!--",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "章節\n> [!unknown] title\n[[A#^a]]\n    A\n===\n## A\nB\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^A\n- [[A\\]]\u0060\u0060\u0060\n## A-2\n## A\n## A\n> > [[A]] [[A]]\t\u0060\u0060\u0060\nA\n---\n> [!note] one\n> [!note] two\n> [!note] three\n![[A]][[A#A]][[A#^a]]A\n===\n[[A\\|alias]]~~~~\n ^a\n<!-- ^é\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^A\n- [[A\\]]\u0060\u0060\u0060\n## A-2\n## A\n## A\n> > [[A]] [[A]]\t\u0060\u0060\u0060\nA\n---\n> [!note] one\n> [!note] two\n> [!note] three\n![[A]][[A#A]][[A#^a]]A\n===\n[[A\\|alias]]~~~~\n ^a\n<!-- ^é\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-aa-aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## [[A|alias]]\n> [!note] [[A]]\n![[A#A]]  > [!note] title\n<!--     ^A\n\u0060\u0060\u0060\u0060 go [[A]]\n## !\n<!--\n[[A]]\n-->\u0060[[A]]\u0060> [!unknown] title\n> [!note] [[A]]\nA\n===\n- ## A-2\n## A\n## A\n- [ ] [[A]]\n\u0060- [ ] [[A]]\n ^é\n ^é\n## A\n> > 1. ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## [[A|alias]]\n> [!note] [[A]]\n![[A#A]]  > [!note] title\n<!--     ^A\n\u0060\u0060\u0060\u0060 go [[A]]\n## !\n<!--\n[[A]]\n-->\u0060[[A]]\u0060> [!unknown] title\n> [!note] [[A]]\nA\n===\n- ## A-2\n## A\n## A\n- [ ] [[A]]\n\u0060- [ ] [[A]]\n ^é\n ^é\n## A\n> > 1. ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## [[A|alias]]\n> [!note] [[A]]\n![[A#A]]  > [!note] title\n<!--     ^A\n\u0060\u0060\u0060\u0060 go [[A]]\n## !\n<!--\n[[A]]\n-->\u0060[[A]]\u0060> [!unknown] title\n> [!note] [[A]]\nA\n===\n- ## A-2\n## A\n## A\n- [ ] [[A]]\n\u0060- [ ] [[A]]\n ^é\n ^é\n## A\n> > 1. ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "0[[A]][[A#^a]]A\n---\n ^A\n# A\nÉ\n%%[[A]]%%[^unused]: [[A]]\n\u0060\u0060\u0060\n[[A\\]]\n\nhttps://example.invalid/\u0060[[A]]\u0060 [^n]: [[A]]\n\n    [[B]]\n~~~~\n<!--\n[[A]]\n-->[[A]] [[A]] \u0060\u0060\u0060\n\\\\[[A]][[image.png]]## A-2\n## A\n## A\ntext 0\n![[image.png]] [[A#A]]0",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0aa-aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\\[[A]]\\[[A]]<!--\n[[A]]\n-->## !\n[[#A]]B\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A\\]]  > [!note] title\n## A\n## A\nB\nA\n---\n> [!note] [[A]]\n\u0060\u0060\u0060\n## A\n> [[A\nB]]## !\n\u0060\u0060\u0060 [[A]]\nA\n---\n[^unused]: [[A]]\n> \u0060\u0060\u0060\u0060\n## <em>A</em>\nA\n===\n<div>\n[[A]]\n</div>\nhttps://example.invalid/\u0060[[A]]\u0060 \u0060\u0060\u0060\n\u0060\u0060\u0060 [[A]]\n![[A#A]]\u0060\u0060\u0060\u0060 go [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "# A\n# A\n<!--\n[[A]]\n-->[[A\\|alias]]<!--\n[[A]]\n-->## A\n[[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "# A\n# A\n<!--\n[[A]]\n-->[[A\\|alias]]<!--\n[[A]]\n-->## A\n[[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "<!-- [[A]] -->## A-2\n## A\n## A\n[[B|alias]]> > \t![[image.png]]\u0060\u0060\u0060\u0060 go [[A]]\n~~~\n1. \u0060[[image.png]]\u0060\u0060![[A]]## A\n  > [!note] title\n\u0060\u0060\u0060\u0060\n- [x] [[B]]\ntext [^n]: [[A]]\n\n    [[B]]\n\u0060\u0060\u0060\n%%A\n---\n<div>\n[[A]]\n</div>\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "É\n## !\n[[A#^a]]## A-2\n## A\n## A\n\n[[A\nB]] ^é\n[[image.png]]- [x] [[B]]\n[[A#^a]]~~~~\n%%[[A#A]]    \u0060\u0060\u0060\n## A\n## A\ntext [^n]: [[A]]\n\n    [[B]]\n## A\n## A\n\\[[A]]## !\n ^A\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[A\\]] ^A\n![[image.png]]## <em>A</em>\nÉ\n[[A#A]]>  ^é\ntext [[A\nB]]  \u0060\u0060\u0060\n[[A\\]]> [!note] one\n> [!note] two\n> [!note] three\n # A\n  > [!note] title\nA\n===\nref[^n]\n![[A]][[A#A]]\\\\[[A]]0 ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " \u0060[[A]]\u0060\t0[[A#^a]]A\n---\nB\n<div>\n[[A]]\n</div>\n\u0060## !\n  > [!note] title\n[[B|alias]]  > [!note] title\nÉ\n> [!note] title\n\u0060open\n[[A]]\nclose\u0060 ^a\nA\n[^n]: [[A]]\n\n    [[B]]\n\u0060\u0060\t\u0060\u0060\u0060\n[[image.png]]- item\n\n       \u0060\u0060\u0060\n[[image.png]]- item\n\n          \u0060\u0060\u0060\n\u0060\u0060<!--\n[[A]]\n-->",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-0a-aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "0  > [!note] title\n> [!note] title\n[[A]]A\n===\n[[A\\]]- > [!note] title\n  \u0060\u0060\u0060\n> [!note] title\n%%[[A]]%% ^a\n[[image.png]]https://example.invalid/\u0060[[A]]\u0060 https://example.invalid/\u0060[[A]]\u0060 [^n]: [[A]]\n\n    [[B]]\n# A\n[[A\\]][[#A]]  > [!note] title\n- > [!note] title\n^absent-prose [[image.png]]## A-2\n## A\n## A\n> [!note] title\n ^A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "-->- \u0060\u0060\u0060\u0060 go [[A]]\n    \u0060\u0060\u0060\nA\n===\nhttps://example.invalid/\u0060[[A]]\u0060 > [!unknown] title\n\u0060open\n[[A]]\nclose\u00600\u0060[[A#^a]]| a | b |\n|---|---|\n| [[A]] | ^a |\nA\n===\n![[image.png]]- > [!note] title\n-->\u0060\u0060\u0060 [[A]]\n![[image.png]]![[A#A]] ^é\n%%> > \u0060\u0060\u0060\u0060\n\u0060\u0060\u0060\u0060\n[[A]] ^A\nB\n- [x] [[B]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"https-example-invalid-a-unknown-title-open-a-close0-a-a-a-b-a-a-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "![[A#A]]0[[A#^a]][[A]]## A-2\n## A\n## A\nB\n%%> > \u0060[[A]]\u0060\t\u0060\u0060\u0060\n[[A\nB]]- > [!note] title\n\t\\[[A]]A\n===\nA\n---\n%%[[A]]%%A\nA\n===\n# A\n     \u0060\u0060\u0060\n\u0060[[A]]\u0060[[A\\]]https://example.invalid/\u0060[[A]]\u0060 \n\u0060open\n[[A]]\nclose\u0060-->> [!unknown] title\n\u0060\u0060\u0060\u0060\nA\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[^n]: [[A]]\n\n    [[B]]\n[[A]]| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!note] one\n> [!note] two\n> [!note] three\n0^absent-prose  ^é\n## A\n## A\n<!--\n[[A]]\n-->%%~~~\n ^a\n[[A#A]]> [!note] one\n> [!note] two\n> [!note] three\n# A\n<!-- [[A]] --><!--A\n\\[[A]]<!--    \u0060\u0060\u0060\n[^n]: [[A]]\n\n    [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "# A\nA\n---\nhttps://example.invalid/\u0060[[A]]\u0060 <!--| a | b |\n|---|---|\n| [[A]] | ^a |\nÉ\n\t\u0060\u0060\u0060\n## <em>A</em>\nref[^n]\nA\n---\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "B\nA\n\t[^n]: [[A]]\n\n    [[B]]\n> [!unknown] title\n\t<!-- [[A]] -->[[A\nB]]<!--> [!note] one\n> [!note] two\n> [!note] three\n章節\n> [!note] one\n> [!note] two\n> [!note] three\n[[A#^a]] [[A]]text ## !\nA\n---\n## !\ntext [[B|alias]]## A\n\t\u0060\u0060\u0060\n[[#A]]## A\n章節\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\u0060\u0060\u0060\u0060 go [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-atext-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^a-2\n  > [!note] title\n%%[[A]]%%A\n===\n0  ^a\n[^n]: [[A]]\n\n    [[B]]\nA\n---\n| a | b |\n|---|---|\n| [[A]] | ^a |\n- item\n\n       ^a-2\n1. ## A-2\n## A\n## A\n# A\n  > [!note] title\n-->> [[A]]%%## [[A|alias]]\n章節\n ^a\n\\\\[[A]][[A\\]][[#A]]  \u0060\u0060\u0060\n[[B|alias]]~~~\n- [x] [[B]]\n> [!note] one\n> [!note] two\n> [!note] three\n\\[[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^a-2\n  > [!note] title\n%%[[A]]%%A\n===\n0  ^a\n[^n]: [[A]]\n\n    [[B]]\nA\n---\n| a | b |\n|---|---|\n| [[A]] | ^a |\n- item\n\n       ^a-2\n1. ## A-2\n## A\n## A\n# A\n  > [!note] title\n-->> [[A]]%%## [[A|alias]]\n章節\n ^a\n\\\\[[A]][[A\\]][[#A]]  \u0060\u0060\u0060\n[[B|alias]]~~~\n- [x] [[B]]\n> [!note] one\n> [!note] two\n> [!note] three\n\\[[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^a-2\n  > [!note] title\n%%[[A]]%%A\n===\n0  ^a\n[^n]: [[A]]\n\n    [[B]]\nA\n---\n| a | b |\n|---|---|\n| [[A]] | ^a |\n- item\n\n       ^a-2\n1. ## A-2\n## A\n## A\n# A\n  > [!note] title\n-->> [[A]]%%## [[A|alias]]\n章節\n ^a\n\\\\[[A]][[A\\]][[#A]]  \u0060\u0060\u0060\n[[B|alias]]~~~\n- [x] [[B]]\n> [!note] one\n> [!note] two\n> [!note] three\n\\[[A]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-5\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] title\n0~~~\n[[A#A]]\u0060B\n> [!note] title\nA\n---\n  \u0060\u0060\u0060\n<!-- [[A]] --><!-- [[A]] -->[^unused]: [[A]]\n  ^a-2\n\u0060open\n[[A]]\nclose\u0060\tÉ\n\u0060open\n[[A]]\nclose\u0060> >  ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060\u0060- [ ] [[A]]\n<div>\n[[A]]\n</div>\n ^a-2\n<!--\n[[A]]\n-->> [!note] [[A]]\n\u0060\u0060\u0060 [[A]]\n   \u0060\u0060\u0060\n\\[[A]][^unused]: [[A]]\n[[image.png]] ![[A#A]]\u0060  > [!note] title\n## <em>A</em>\n<!--\n[[A]]\n-->\\\u0060>  ^é\n## <em>A</em>\n> [!note] one\n> [!note] two\n> [!note] three\n\u0060\u0060\u0060\u0060\n  \u0060\u0060\u0060\n01. [^n]: [[A]]\n\n    [[B]]\n ^A\n\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"em-a-em-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  - [[A#A]][^n]: [[A]]\n\n    [[B]]\n> [!note] title\n\u0060[[A]]\u0060 ^a\n\\\\[[A]]A\n---\n## A\n## A\n-->> [!note] one\n> [!note] two\n> [!note] three\n> > [^unused]: [[A]]\n ^A\n1.    \u0060\u0060\u0060\n<div>\n[[A]]\n</div>\n\u0060## [[A|alias]]\n%%[[A]]%%\\[[A]]^absent-prose     ## [[A|alias]]\n ^é\n[[A\\]]A\n===\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "  - [[A#A]][^n]: [[A]]\n\n    [[B]]\n> [!note] title\n\u0060[[A]]\u0060 ^a\n\\\\[[A]]A\n---\n## A\n## A\n-->> [!note] one\n> [!note] two\n> [!note] three\n> > [^unused]: [[A]]\n ^A\n1.    \u0060\u0060\u0060\n<div>\n[[A]]\n</div>\n\u0060## [[A|alias]]\n%%[[A]]%%\\[[A]]^absent-prose     ## [[A|alias]]\n ^é\n[[A\\]]A\n===\n## A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-a-aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060\\\\[[A]]text ## A-2\n## A\n## A\n[[A\nB]]%%[[A\nB]]~~~\n[[A#A]]-     <!--1. - item\n\n      ## A\n## A\n   \u0060\u0060\u0060\n[[A\nB]] \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "[[image.png]]\t\u0060\u0060\u0060\n[[A]] [[A]]    \u0060\u0060\u0060\u0060\n\u0060章節\n0> [[A\\|alias]] ^A\n<!-- [[A]] -->![[A]]## !\n0[[A#^a]]\n![[image.png]]![[A#A]][[A\\|alias]]\u0060\u0060\u0060\u0060 go [[A]]\nB\nA\n===\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-png-a-a-章節-0-alias-a-a-0a-a-image-png-a-a-alias-go-a-b-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "## A-2\n## A\n## A\n\\[[A]]<!--\n[[A]]\n-->[[image.png]]A\n===\n<!-- [[A]] -->\\\\[[A]][^unused]: [[A]]\n- [x] [[B]]\n[[A\\]]\\[[A]]\u0060\u0060\u0060\n\n\n> [!note] one\n> [!note] two\n> [!note] three\n[[B|alias]]É\n0![[image.png]]## <em>A</em>\n-->~~~~\n ^a-2\n> [!unknown] title\n[[A\\]]B\n\t[[B|alias]]# A\n[[A]]> [!note] one\n> [!note] two\n> [!note] three\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^A\n章節\nA\n===\n~~~\n~~~\n- [x] [[B]]\n\t\u0060\u0060\u0060\n0[[image.png]]A\n---\n ![[image.png]]A\n---\n[[A\nB]]- [x] [[B]]\nA\n^absent-prose [^unused]: [[A]]\n ^a\n~~~~\n> > ![[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0image-pnga\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": " ^A\n章節\nA\n===\n~~~\n~~~\n- [x] [[B]]\n\t\u0060\u0060\u0060\n0[[image.png]]A\n---\n ![[image.png]]A\n---\n[[A\nB]]- [x] [[B]]\nA\n^absent-prose [^unused]: [[A]]\n ^a\n~~~~\n> > ![[image.png]]",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-png-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060[[A]]\u0060![[A]]text https://example.invalid/\u0060[[A]]\u0060 [[A\nB]]- item\n\n      A\n## !\n> [!note] title\n\u0060open\n[[A]]\nclose\u0060\\[[A]]text [[A\\|alias]]A\n===\n ^a\n <div>\n[[A]]\n</div>\n ^a\n\u0060\u0060\u0060\u0060 go [[A]]\n## !\n<!--[[A\\|alias]]## [[A|alias]]\n章節\n[^unused]: [[A]]\n<div>\n[[A]]\n</div>\n  > [!note] title\n![[A#A]][^n]: [[A]]\n\n    [[B]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"open-a-close-a-text-aliasa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "ref[^n]\n-->[^n]: [[A]]\n\n    [[B]]\n\\\u0060ref[^n]\n## A\n## A\n\\\u00600> [!note] [[A]]\ntext    \u0060\u0060\u0060\n%%## A-2\n## A\n## A\n<!-- [[A]] -->\\\u0060-->[[A]] ^a-2\n## A\n## A\n\u0060\u0060open\n[[A]]\nclose\u0060\t\u0060\u0060\u0060\n- [ ] [[A]]\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "-->A\n===\n> [!note] one\n> [!note] two\n> [!note] three\n[[A#^a]]A\n---\n\u0060  -   - > # A\n\u0060open\n[[A]]\nclose\u0060 [^unused]: [[A]]\n> [!unknown] title\n## A\n## A\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "-->A\n===\n> [!note] one\n> [!note] two\n> [!note] three\n[[A#^a]]A\n---\n\u0060  -   - > # A\n\u0060open\n[[A]]\nclose\u0060 [^unused]: [[A]]\n> [!unknown] title\n## A\n## A\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "-->A\n===\n> [!note] one\n> [!note] two\n> [!note] three\n[[A#^a]]A\n---\n\u0060  -   - > # A\n\u0060open\n[[A]]\nclose\u0060 [^unused]: [[A]]\n> [!unknown] title\n## A\n## A\n\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-aa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] title\n%%\u0060\u0060\u0060 [[A]]\n- ## [[A|alias]]\n ^é\n[[B|alias]]0ref[^n]\n%%\n\nhttps://example.invalid/\u0060[[A]]\u0060 > [!note] one\n> [!note] two\n> [!note] three\n<div>\n[[A]]\n</div>\n> [!note] one\n> [!note] two\n> [!note] three\n## [[A|alias]]\nB\n ^a\n## !\n[[#A]]~~~\n> [!note] title\n| a | b |\n|---|---|\n| [[A]] | ^a |\nÉ\n[^unused]: [[A]]\n[[image.png]] <!--\n[[A]]\n-->## A-2\n## A\n## A\n\\\\[[A]]![[A#A]]  \u0060\u0060\u0060\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\nA\n===\n  > [!note] title\nA\n---\n0\u0060open\n[[A]]\nclose\u0060   \u0060\u0060\u0060\n## <em>A</em>\n 章節\n## A-2\n## A\n## A\n- [ ] [[A]]\n ^A\n\n\n\u0060~~~~\n- > [!note] title\n\u0060open\n[[A]]\nclose\u0060[[B|alias]]# A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\nA\n===\n  > [!note] title\nA\n---\n0\u0060open\n[[A]]\nclose\u0060   \u0060\u0060\u0060\n## <em>A</em>\n 章節\n## A-2\n## A\n## A\n- [ ] [[A]]\n ^A\n\n\n\u0060~~~~\n- > [!note] title\n\u0060open\n[[A]]\nclose\u0060[[B|alias]]# A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!note] one\n> [!note] two\n> [!note] three\nA\n===\n  > [!note] title\nA\n---\n0\u0060open\n[[A]]\nclose\u0060   \u0060\u0060\u0060\n## <em>A</em>\n 章節\n## A-2\n## A\n## A\n- [ ] [[A]]\n ^A\n\n\n\u0060~~~~\n- > [!note] title\n\u0060open\n[[A]]\nclose\u0060[[B|alias]]# A\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-4\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "\u0060<!--\n[[A]]\n-->## !\n0  -  ^a\n章節\n[^n]: [[A]]\n\n    [[B]]\n> > \u0060\u0060\u0060\n[[A#^a]]- [ ] [[A]]\n## !\n[[A]] ^é\n\\\u0060  - ## A\n## A\n\\\\[[A]]![[image.png]]| a | b |\n|---|---|\n| [[A]] | ^a |\n\u0060\u0060\u0060\u0060 go [[A]]\n[[A]]É\n- > [!note] title\n- [ ] [[A]]\n![[image.png]]  > [!note] title\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"section-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "> [!unknown] title\n[[image.png]][[A\\|alias]]A\n===\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"image-pngaliasa\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "^absent-prose <!--\n[[A]]\n-->É\n0> ![[image.png]][[B|alias]]- [[#A]]\t\u0060\u0060\u0060\n<!--\n[[A]]\n-->0[[image.png]]A\n^absent-prose https://example.invalid/\u0060[[A]]\u0060 \u0060- A\ntext A\n---\n[^unused]: [[A]]\n> [!note] [[A]]\n    B\n",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"0image-pnga-absent-prose-https-example-invalid-a-a-text-a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  },
  {
    "Body": "    \u0060\u0060\u0060\n<!--\n[[A]]\n-->\n# A\n\u0060\u0060\\\u0060- > [!note] title\n\\[[A]][[A\nB]]0A\n---\nB\nhttps://example.invalid/\u0060[[A]]\u0060 \u0060[[A]]\u0060[[A\nB]]| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!unknown] title\n[[B|alias]]章節\n> [!note] one\n> [!note] two\n> [!note] three\n![[image.png]]<!-- [[A]] -->0- > [!note] title\n[^n]: [[A]]\n\n    [[B]]\n[^n]: [[A]]\n\n    [[B]]\n\n- ",
    "Selected": "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"note-title-a-a-b-0a\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false",
    "Eligible": false
  }
]`
	var rows []struct {
		Body, Selected string
		Eligible       bool
	}
	if json.Unmarshal([]byte(frozen), &rows) != nil || len(rows) != 622 {
		t.Fatal("caught: whole heading prospect roster")
	}
	eligible := map[[2]string]bool{}
	for _, r := range rows {
		raw, page := agreementIsolatedPage(t, agreementCase{Body: r.Body})
		fs := agreementPageFailures(r.Body, &raw, &page)
		fs = append(fs, agreementFragmentFailures(t, []agreementCase{{Body: r.Body}}, []agreementHTML{page})[0]...)
		var selected *agreementFailure
		for i := range fs {
			if agreementSignature(&fs[i]) == r.Selected {
				selected = &fs[i]
			}
		}
		if selected == nil {
			t.Fatal("caught: every original heading prospect exists")
		}
		actual := agreementHeadingSingleEligible(r.Body, selected, &page)
		if actual != r.Eligible {
			t.Fatal("caught: whole heading eligibility and refusal set")
		}
		if actual {
			eligible[[2]string{r.Body, r.Selected}] = true
		}
	}
	r := agreementHeadingSingleDeclarationReading()
	if len(eligible) != 1 || r == nil || !eligible[[2]string{r.Body, r.Selected}] {
		t.Fatal("caught: complete single heading eligible declaration set")
	}
}
