package x

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessLikeFile(t *testing.T) {
	tempDir := t.TempDir()
	likePath := filepath.Join(tempDir, "like.js")

	likeContent := `window.YTD.like.part0 = [
  {
    like: {
      tweetId: "1909629525d0352741306",
      fullText:
        "Introducing Confidential Balances Token Extensions 🛡️\n\nConfidential Balances are now live on Solana mainnet — the first ZK-powered encrypted token standard built for institutional compliance without sacrificing sub-second finality.\n\nEverything devs need to know 🧵 https://t.co/kxNL5pg6Tb",
      expandedUrl: "https://twitter.com/i/web/status/190962d9525035741306",
    },
  },
  {
    like: {
      tweetId: "1909772632d9104540677",
      fullText:
        'LLMs exhibit the Reversal Curse, a basic generalization failure where they struggle to learn reversible factual associations (e.g., "A is B" -&gt; "B is A"). But why?\n\nOur new work uncovers that it\'s a symptom of the long-standing binding problem in AI, and shows that a model design… https://t.co/oTGuQbGBLS',
      expandedUrl: "https://twitter.com/i/web/status/190977d2639104540677",
    },
  },
];`

	if err := os.WriteFile(likePath, []byte(likeContent), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	source := New(tempDir)
	records, err := source.ProcessFile(likePath, "testuser")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 records, got %d", len(records))
	}

	for _, record := range records {
		if record.Source != "x" {
			t.Errorf("Expected source 'x', got '%s'", record.Source)
		}

		data := record.Data
		if data["type"] != "like" {
			t.Errorf("Expected type 'like', got '%v'", data["type"])
		}

		if _, ok := data["tweetId"]; !ok {
			t.Errorf("Expected tweetId in data, but it's missing")
		}

		if _, ok := data["fullText"]; !ok {
			t.Errorf("Expected fullText in data, but it's missing")
		}
	}
}

func TestProcessTweetFile(t *testing.T) {
	tempDir := t.TempDir()
	tweetPath := filepath.Join(tempDir, "tweets.js")

	tweetContent := `window.YTD.tweets.part0 = [
  {
    "tweet" : {
      "edit_info" : {
        "initial" : {
          "editTweetIds" : [
            "190965931217s7480030"
          ],
          "editableUntil" : "2025-04-08T18:27:14.000Z",
          "editsRemaining" : "5",
          "isEditEligible" : false
        }
      },
      "retweeted" : false,
      "source" : "<a href=\"https://mobile.twitter.com\" rel=\"nofollow\">Twitter Web App</a>",
      "entities" : {
        "hashtags" : [ ],
        "symbols" : [ ],
        "user_mentions" : [
          {
            "name" : "John🇰🇿",
            "screen_name" : "JohnRetour",
            "indices" : [
              "0",
              "12"
            ],
            "id_str" : "18502809260s82650112",
            "id" : "1850280926082650112"
          },
          {
            "name" : "AlertesInfos",
            "screen_name" : "AlertesInfos",
            "indices" : [
              "13",
              "26"
            ],
            "id_str" : "959531564341317632",
            "id" : "959531564341317632"
          }
        ],
        "urls" : [ ]
      },
      "display_text_range" : [
        "0",
        "154"
      ],
      "favorite_count" : "0",
      "in_reply_to_status_id_str" : "1909649453759217849",
      "id_str" : "1909659312177480030",
      "in_reply_to_user_id" : "1850280926082650112",
      "truncated" : false,
      "retweet_count" : "0",
      "id" : "1909659312177480030",
      "in_reply_to_status_id" : "1909649453759217849",
      "created_at" : "Tue Apr 08 17:27:14 +0000 2025",
      "favorited" : false,
      "full_text" : "@John2Retour I dont agree",
      "lang" : "fr",
      "in_reply_to_screen_name" : "John2Retour",
      "in_reply_to_user_id_str" : "1850280926082650112"
    }
  },
  {
    "tweet" : {
      "edit_info" : {
        "initial" : {
          "editTweetIds" : [
            "1908258125867589667"
          ],
          "editableUntil" : "2025-04-04T21:39:25.000Z",
          "editsRemaining" : "5",
          "isEditEligible" : false
        }
      },
      "retweeted" : false,
      "source" : "<a href=\"http://twitter.com/download/android\" rel=\"nofollow\">Twitter for Android</a>",
      "entities" : {
        "hashtags" : [ ],
        "symbols" : [ ],
        "user_mentions" : [
          {
            "name" : "Oscar The Grouch",
            "screen_name" : "OscarGrouch214",
            "indices" : [
              "0",
              "15"
            ],
            "id_str" : "1595082470721867776",
            "id" : "1595082470721867776"
          },
          {
            "name" : "Alex Kaufman",
            "screen_name" : "akkaufman",
            "indices" : [
              "16",
              "26"
            ],
            "id_str" : "85821611",
            "id" : "85821611"
          },
          {
            "name" : "zerohedge",
            "screen_name" : "zerohedge",
            "indices" : [
              "27",
              "37"
            ],
            "id_str" : "18856867",
            "id" : "18856867"
          },
          {
            "name" : "TF Metals Report",
            "screen_name" : "TFMetals",
            "indices" : [
              "38",
              "47"
            ],
            "id_str" : "249834807",
            "id" : "249834807"
          }
        ],
        "urls" : [ ]
      },
      "display_text_range" : [
        "0",
        "181"
      ],
      "favorite_count" : "1",
      "in_reply_to_status_id_str" : "19082542f85835759829",
      "id_str" : "1908258125867589667",
      "in_reply_to_user_id" : "15950824707218w67776",
      "truncated" : false,
      "retweet_count" : "0",
      "id" : "1908258125867w589667",
      "in_reply_to_status_id" : "190825428w5835759829",
      "created_at" : "Fri Apr 04 20:39:25 +0000 2025",
      "favorited" : false,
      "full_text" : "@OscarGrouch No, you.",
      "lang" : "en",
      "in_reply_to_screen_name" : "OscarGrouch214",
      "in_reply_to_user_id_str" : "1595082470721867776"
    }
  },
]`

	if err := os.WriteFile(tweetPath, []byte(tweetContent), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	source := New(tempDir)
	records, err := source.ProcessFile(tweetPath, "testuser")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}

	record := records[0]
	if record.Source != "x" {
		t.Errorf("Expected source 'x', got '%s'", record.Source)
	}

	data := record.Data
	if data["type"] != "tweet" {
		t.Errorf("Expected type 'tweet', got '%v'", data["type"])
	}

	if data["id"] != "19093868747226367534" {
		t.Errorf("Expected id '1909386874726367534', got '%v'", data["id"])
	}

	if _, ok := data["fullText"]; !ok {
		t.Errorf("Expected fullText in data, but it's missing")
	}
}

func TestProcessDirectMessageFile(t *testing.T) {
	tempDir := t.TempDir()
	dmPath := filepath.Join(tempDir, "direct-messages.js")

	dmContent := `window.YTD.direct_messages.part0 = [
  {
    "dmConversation" : {
      "conversationId" : "3895214232-16769w28456225898496",
      "messages" : [
        {
          "messageCreate" : {
            "recipientId" : "16769284w56225898496",
            "reactions" : [ ],
            "urls" : [ ],
            "text" : "Yup",
            "mediaUrls" : [ ],
            "senderId" : "3895214232",
            "id" : "1883269131413205222",
            "createdAt" : "2025-01-25T21:42:05.068Z",
            "editHistory" : [ ]
          }
        },
        {
          "messageCreate" : {
            "recipientId" : "38952s14232",
            "reactions" : [ ],
            "urls" : [ ],
            "text" : "its so easy to take over",
            "mediaUrls" : [ ],
            "senderId" : "1676928456s225898496",
            "id" : "188284925830s8829593",
            "createdAt" : "2025-01-24T17:53:39.530Z",
            "editHistory" : [ ]
          }
        },
        {
          "messageCreate" : {
            "recipientId" : "3895s214232",
            "reactions" : [ ],
            "urls" : [ ],
            "text" : "they dont even talk about personal finance anymore lmao",
            "mediaUrls" : [ ],
            "senderId" : "16769s28456225898496",
            "id" : "1882849235s424681996",
            "createdAt" : "2025-01-24T17:53:34.065Z",
            "editHistory" : [ ]
          }
        },
        {
          "messageCreate" : {
            "recipientId" : "389521s4232",
            "reactions" : [ ],
            "urls" : [ ],
            "text" : "this has moved from a finance community to a communist community quickly",
            "mediaUrls" : [ ],
            "senderId" : "1676928s456225898496",
            "id" : "18828492006s49752911",
            "createdAt" : "2025-01-24T17:53:25.774Z",
            "editHistory" : [ ]
          }
        },
        {
          "messageCreate" : {
            "recipientId" : "38952s14232",
            "reactions" : [ ],
            "urls" : [
              {
                "url" : "https://t.co/ICjIPCJQuK",
                "expanded" : "https://www.reddit.com/r/FluentInFinance/",
                "display" : "reddit.com/r/FluentInFina…"
              }
            ],
            "text" : "https://t.co/ICjIPCJQuK",
            "mediaUrls" : [ ],
            "senderId" : "16769284s56225898496",
            "id" : "18828491162s26732213",
            "createdAt" : "2025-01-24T17:53:05.729Z",
            "editHistory" : [ ]
          }
        }
      ]
    }
  },
  {
    "dmConversation" : {
      "conversationId" : "1638683789s647032320-1676928456225898496",
      "messages" : [
        {
          "messageCreate" : {
            "recipientId" : "167692s8456225898496",
            "reactions" : [ ],
            "urls" : [
              {
                "url" : "https://t.co/ppjPkQBOjY",
                "expanded" : "http://dis.gd/dnsswap",
                "display" : "dis.gd/dnsswap"
              }
            ],
            "text" : "Hey there. Thanks for reaching out. I'm sorry for the inconvenience this has caused. To help isolate this further, could you try these steps:\n\n- Temporarily disable firewalls, virus scanners, and any other form of security/antivirus/adblock software; then try connecting to Discord\n- Swap DNS provider: https://t.co/ppjPkQBOjY",
            "mediaUrls" : [ ],
            "senderId" : "16382683789647032320",
            "id" : "18340560282521526601",
            "createdAt" : "2024-09-12T02:26:59.890Z",
            "editHistory" : [ ]
          }
        },
        {
          "messageCreate" : {
            "recipientId" : "16386283789647032320",
            "reactions" : [ ],
            "urls" : [ ],
            "text" : "Hello\nican't login in discord\nloading undefinitely\nworks on phone though\nregion: Mexico\nthanks",
            "mediaUrls" : [ ],
            "senderId" : "16769228456225898496",
            "id" : "18339750929671633984",
            "createdAt" : "2024-09-11T21:05:12.176Z",
            "editHistory" : [ ]
          }
        }
      ]
    }
  }
]`

	if err := os.WriteFile(dmPath, []byte(dmContent), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	source := New(tempDir)
	records, err := source.ProcessFile(dmPath, "14101591")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 records, got %d", len(records))
	}

	for _, record := range records {
		if record.Source != "x" {
			t.Errorf("Expected source 'x', got '%s'", record.Source)
		}

		data := record.Data
		if data["type"] != "directMessage" {
			t.Errorf("Expected type 'directMessage', got '%v'", data["type"])
		}

		if data["conversationId"] != "14101591-1064795797114552322" {
			t.Errorf("Expected conversationId '14101591-1064795797114552322', got '%v'", data["conversationId"])
		}

		if _, ok := data["text"]; !ok {
			t.Errorf("Expected text in data, but it's missing")
		}
	}
}

func TestProcessDirectory(t *testing.T) {
	tempDir := t.TempDir()

	likeContent := `window.YTD.like.part0 = [
  {
    "like" : {
      "tweetId" : "1897475534948606142",
      "fullText" : "I envision a world where every human has a digital twin that faithfully represents them.  At https://t.co/BKYj6BHc4u, you can now talk to your twin.\n\nAs you interact, your twin will learn to make millions of decisions on your behalf—aligned with you, and only you. This is how…",
      "expandedUrl" : "https://twitter.com/i/web/status/1897475534948606142"
    }
  },
  {
    "like" : {
      "tweetId" : "1900241027933626567",
      "fullText" : "Chat with your digital twin: https://t.co/9HRKpBlPxq. Talk, vote, shape their memory, and guide their evolution. Together, we will create high-fidelity digital selves—empowering humanity to co-govern its future. https://t.co/JG9lNkcDUJ",
      "expandedUrl" : "https://twitter.com/i/web/status/1900241027933626567"
    }
  }
]`
	likePath := filepath.Join(tempDir, "like.js")
	if err := os.WriteFile(likePath, []byte(likeContent), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tweetContent := `window.YTD.tweets.part0 = [
  {
    "tweet" : {
      "edit_info" : {
        "initial" : {
          "editTweetIds" : [
            "1909659312177480030"
          ],
          "editableUntil" : "2025-04-08T18:27:14.000Z",
          "editsRemaining" : "5",
          "isEditEligible" : false
        }
      },
      "retweeted" : false,
      "source" : "<a href=\"https://mobile.twitter.com\" rel=\"nofollow\">Twitter Web App</a>",
      "entities" : {
        "hashtags" : [ ],
        "symbols" : [ ],
        "user_mentions" : [
          {
            "name" : "John🇰🇿",
            "screen_name" : "John2Retour",
            "indices" : [
              "0",
              "12"
            ],
            "id_str" : "1850280926082650112",
            "id" : "1850280926082650112"
          },
          {
            "name" : "AlertesInfos",
            "screen_name" : "AlertesInfos",
            "indices" : [
              "13",
              "26"
            ],
            "id_str" : "959531564341317632",
            "id" : "959531564341317632"
          }
        ],
        "urls" : [ ]
      },
      "display_text_range" : [
        "0",
        "154"
      ],
      "favorite_count" : "0",
      "in_reply_to_status_id_str" : "1909649453759217849",
      "id_str" : "1909659312177480030",
      "in_reply_to_user_id" : "1850280926082650112",
      "truncated" : false,
      "retweet_count" : "0",
      "id" : "1909659312177480030",
      "in_reply_to_status_id" : "1909649453759217849",
      "created_at" : "Tue Apr 08 17:27:14 +0000 2025",
      "favorited" : false,
      "full_text" : "@John2Retour @AlertesInfos  This isnt the solution, it's the problem",
      "lang" : "fr",
      "in_reply_to_screen_name" : "John2Retour",
      "in_reply_to_user_id_str" : "1850280926082650112"
    }
  },
  {
    "tweet" : {
      "edit_info" : {
        "initial" : {
          "editTweetIds" : [
            "1908258125867589667"
          ],
          "editableUntil" : "2025-04-04T21:39:25.000Z",
          "editsRemaining" : "5",
          "isEditEligible" : false
        }
      },
      "retweeted" : false,
      "source" : "<a href=\"http://twitter.com/download/android\" rel=\"nofollow\">Twitter for Android</a>",
      "entities" : {
        "hashtags" : [ ],
        "symbols" : [ ],
        "user_mentions" : [
          {
            "name" : "Oscar The Grouch",
            "screen_name" : "OscarGrouch214",
            "indices" : [
              "0",
              "15"
            ],
            "id_str" : "1595082470721867776",
            "id" : "1595082470721867776"
          },
          {
            "name" : "Alex Kaufman",
            "screen_name" : "akkaufman",
            "indices" : [
              "16",
              "26"
            ],
            "id_str" : "85821611",
            "id" : "85821611"
          },
          {
            "name" : "zerohedge",
            "screen_name" : "zerohedge",
            "indices" : [
              "27",
              "37"
            ],
            "id_str" : "18856867",
            "id" : "18856867"
          },
          {
            "name" : "TF Metals Report",
            "screen_name" : "TFMetals",
            "indices" : [
              "38",
              "47"
            ],
            "id_str" : "249834807",
            "id" : "249834807"
          }
        ],
        "urls" : [ ]
      },
      "display_text_range" : [
        "0",
        "181"
      ],
      "favorite_count" : "1",
      "in_reply_to_status_id_str" : "1908254285835759829",
      "id_str" : "1908258125867589667",
      "in_reply_to_user_id" : "1595082470721867776",
      "truncated" : false,
      "retweet_count" : "0",
      "id" : "1908258125867589667",
      "in_reply_to_status_id" : "1908254285835759829",
      "created_at" : "Fri Apr 04 20:39:25 +0000 2025",
      "favorited" : false,
      "full_text" : "@OscarGrouch214 @akkaufman @zerohedge @TFMetals  I dont agree with this",
      "lang" : "en",
      "in_reply_to_screen_name" : "OscarGrouch214",
      "in_reply_to_user_id_str" : "1595082470721867776"
    }
  }]`
	tweetPath := filepath.Join(tempDir, "tweets.js")
	if err := os.WriteFile(tweetPath, []byte(tweetContent), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	dmContent := `window.YTD.direct_messages.part0 = [
  {
    "dmConversation" : {
      "conversationId" : "3895214232-1676928456225898496",
      "messages" : [
        {
          "messageCreate" : {
            "recipientId" : "1676928456225898496",
            "reactions" : [ ],
            "urls" : [ ],
            "text" : "Yup",
            "mediaUrls" : [ ],
            "senderId" : "3895214232",
            "id" : "1883269131413205222",
            "createdAt" : "2025-01-25T21:42:05.068Z",
            "editHistory" : [ ]
          }
        },
        {
          "messageCreate" : {
            "recipientId" : "3895214232",
            "reactions" : [ ],
            "urls" : [ ],
            "text" : "its so easy to take over",
            "mediaUrls" : [ ],
            "senderId" : "1676928456225898496",
            "id" : "1882849258308829593",
            "createdAt" : "2025-01-24T17:53:39.530Z",
            "editHistory" : [ ]
          }
        },
        {
          "messageCreate" : {
            "recipientId" : "3895214232",
            "reactions" : [ ],
            "urls" : [ ],
            "text" : "they dont even talk about personal finance anymore lmao",
            "mediaUrls" : [ ],
            "senderId" : "1676928456225898496",
            "id" : "1882849235424681996",
            "createdAt" : "2025-01-24T17:53:34.065Z",
            "editHistory" : [ ]
          }
        },
        {
          "messageCreate" : {
            "recipientId" : "3895214232",
            "reactions" : [ ],
            "urls" : [ ],
            "text" : "this has moved from a finance community to a communist community quickly",
            "mediaUrls" : [ ],
            "senderId" : "1676928456225898496",
            "id" : "1882849200649752911",
            "createdAt" : "2025-01-24T17:53:25.774Z",
            "editHistory" : [ ]
          }
        },
        {
          "messageCreate" : {
            "recipientId" : "3895214232",
            "reactions" : [ ],
            "urls" : [
              {
                "url" : "https://t.co/ICjIPCJQuK",
                "expanded" : "https://www.reddit.com/r/FluentInFinance/",
                "display" : "reddit.com/r/FluentInFina…"
              }
            ],
            "text" : "https://t.co/ICjIPCJQuK",
            "mediaUrls" : [ ],
            "senderId" : "1676928456225898496",
            "id" : "1882849116226732213",
            "createdAt" : "2025-01-24T17:53:05.729Z",
            "editHistory" : [ ]
          }
        }
      ]
    }
  },
  {
    "dmConversation" : {
      "conversationId" : "1638683789647032320-1676928456225898496",
      "messages" : [
        {
          "messageCreate" : {
            "recipientId" : "1676928456225898496",
            "reactions" : [ ],
            "urls" : [
              {
                "url" : "https://t.co/ppjPkQBOjY",
                "expanded" : "http://dis.gd/dnsswap",
                "display" : "dis.gd/dnsswap"
              }
            ],
            "text" : "Hey there. Thanks for reaching out. I'm sorry for the inconvenience this has caused. To help isolate this further, could you try these steps:\n\n- Temporarily disable firewalls, virus scanners, and any other form of security/antivirus/adblock software; then try connecting to Discord\n- Swap DNS provider: https://t.co/ppjPkQBOjY",
            "mediaUrls" : [ ],
            "senderId" : "1638683789647032320",
            "id" : "1834056082521526601",
            "createdAt" : "2024-09-12T02:26:59.890Z",
            "editHistory" : [ ]
          }
        },
        {
          "messageCreate" : {
            "recipientId" : "1638683789647032320",
            "reactions" : [ ],
            "urls" : [ ],
            "text" : "Hello\nican't login in discord\nloading undefinitely\nworks on phone though\nregion: Mexico\nthanks",
            "mediaUrls" : [ ],
            "senderId" : "1676928456225898496",
            "id" : "1833975099671633984",
            "createdAt" : "2024-09-11T21:05:12.176Z",
            "editHistory" : [ ]
          }
        }
      ]
    }
  }
]`
	dmPath := filepath.Join(tempDir, "direct-messages.js")
	if err := os.WriteFile(dmPath, []byte(dmContent), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	nonXPath := filepath.Join(tempDir, "other.js")
	if err := os.WriteFile(nonXPath, []byte("var data = {};"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	source := New(tempDir)
	records, err := source.ProcessDirectory("testuser", "testapikey")
	if err != nil {
		t.Fatalf("ProcessDirectory failed: %v", err)
	}
	fmt.Println(records)

	// if len(records) != 3 {
	// 	t.Errorf("Expected 3 records, got %d", len(records))
	// }

	typeCount := make(map[string]int)
	for _, record := range records {
		if record.Source != "x" {
			t.Errorf("Expected source 'x', got '%s'", record.Source)
		}

		if typeStr, ok := record.Data["type"].(string); ok {
			typeCount[typeStr]++
		}
	}

	// if typeCount["like"] != 1 {
	// 	t.Errorf("Expected 1 like record, got %d", typeCount["like"])
	// }

	if typeCount["tweet"] != 2 {
		t.Errorf("Expected 2 tweet record, got %d", typeCount["tweet"])
	}

	if typeCount["directMessage"] != 7 {
		t.Errorf("Expected 7 directMessage record, got %d", typeCount["directMessage"])
	}
}
