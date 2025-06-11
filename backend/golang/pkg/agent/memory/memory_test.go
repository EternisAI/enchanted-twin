package memory

import (
	"strings"
	"testing"
	"time"
)

func TestConversationDocumentContent(t *testing.T) {
	// Create the ConversationDocument from the JSON data
	doc := &ConversationDocument{
		FieldID:     "whatsapp_family_dinner_2024_01_14",
		FieldSource: "whatsapp",
		People:      []string{"Sarah Johnson", "Mom", "Dad", "Jake"},
		User:        "Sarah Johnson",
		Conversation: []ConversationMessage{
			{
				Speaker: "Sarah Johnson",
				Content: "Hey family! Just got promoted to Senior Software Engineer at Google! 🎉",
				Time:    time.Date(2024, 1, 14, 14, 30, 15, 0, time.UTC),
			},
			{
				Speaker: "Mom",
				Content: "Oh honey that's wonderful! We're so proud of you. How's the new apartment in San Francisco?",
				Time:    time.Date(2024, 1, 14, 14, 31, 22, 0, time.UTC),
			},
			{
				Speaker: "Sarah Johnson",
				Content: "Thanks Mom! The apartment is great, finally have a view of the bay. Still getting used to the $4000/month rent though 😅",
				Time:    time.Date(2024, 1, 14, 14, 32, 45, 0, time.UTC),
			},
			{
				Speaker: "Dad",
				Content: "That's expensive! But you're doing great kiddo. How's your boyfriend Alex doing? Still working on that startup?",
				Time:    time.Date(2024, 1, 14, 14, 33, 12, 0, time.UTC),
			},
			{
				Speaker: "Sarah Johnson",
				Content: "Alex is good! His AI startup just raised $2M in seed funding. We're thinking about getting a dog together, maybe a Golden Retriever",
				Time:    time.Date(2024, 1, 14, 14, 34, 1, 0, time.UTC),
			},
			{
				Speaker: "Jake",
				Content: "Congrats sis! BTW I'm graduating from Stanford next month with my CS degree. Job hunting is brutal though",
				Time:    time.Date(2024, 1, 14, 14, 35, 18, 0, time.UTC),
			},
			{
				Speaker: "Sarah Johnson",
				Content: "Jake that's amazing! I can refer you to Google if you want. They're always looking for new grads. I know the hiring manager for our team",
				Time:    time.Date(2024, 1, 14, 14, 36, 42, 0, time.UTC),
			},
			{
				Speaker: "Mom",
				Content: "You kids are doing so well! Sarah, are you still doing those CrossFit classes? And Jake, remember to eat properly during finals",
				Time:    time.Date(2024, 1, 14, 14, 37, 55, 0, time.UTC),
			},
			{
				Speaker: "Sarah Johnson",
				Content: "Yes still doing CrossFit 4x a week! Actually competing in a local competition next month. And I'm training for the Bay to Breakers run in May",
				Time:    time.Date(2024, 1, 14, 14, 39, 12, 0, time.UTC),
			},
			{
				Speaker: "Dad",
				Content: "That's my athletic daughter! Speaking of May, don't forget about cousin Emma's wedding on May 15th in Portland",
				Time:    time.Date(2024, 1, 14, 14, 40, 33, 0, time.UTC),
			},
			{
				Speaker: "Sarah Johnson",
				Content: "Oh right! I need to book flights. Alex and I will definitely be there. I'm actually the maid of honor so I need to plan her bachelorette party too",
				Time:    time.Date(2024, 1, 14, 14, 41, 44, 0, time.UTC),
			},
			{
				Speaker: "Jake",
				Content: "Can't wait to see everyone! Sarah thanks for the Google referral offer, I'll definitely take you up on that. My GPA is 3.8 so hopefully that helps",
				Time:    time.Date(2024, 1, 14, 14, 42, 58, 0, time.UTC),
			},
			{
				Speaker: "Sarah Johnson",
				Content: "3.8 is great! I'll send you the referral link tomorrow. Also, I'm flying home for Easter weekend, can't wait to see everyone and Mom's famous lasagna!",
				Time:    time.Date(2024, 1, 14, 14, 44, 15, 0, time.UTC),
			},
		},
		FieldTags: []string{"family", "career", "personal_updates"},
		FieldMetadata: map[string]string{
			"group_name":                  "Johnson Family",
			"participant_count":           "4",
			"conversation_length_minutes": "14",
			"platform":                    "WhatsApp",
		},
	}

	// Expected output with primaryUser normalization
	expected := `CONVO|whatsapp_family_dinner_2024_01_14|whatsapp
PEOPLE|primaryUser|Mom|Dad|Jake
PRIMARY|primaryUser
|||
primaryUser|||2024-01-14T14:30:15Z|||Hey family! Just got promoted to Senior Software Engineer at Google! 🎉
Mom|||2024-01-14T14:31:22Z|||Oh honey that's wonderful! We're so proud of you. How's the new apartment in San Francisco?
primaryUser|||2024-01-14T14:32:45Z|||Thanks Mom! The apartment is great, finally have a view of the bay. Still getting used to the $4000/month rent though 😅
Dad|||2024-01-14T14:33:12Z|||That's expensive! But you're doing great kiddo. How's your boyfriend Alex doing? Still working on that startup?
primaryUser|||2024-01-14T14:34:01Z|||Alex is good! His AI startup just raised $2M in seed funding. We're thinking about getting a dog together, maybe a Golden Retriever
Jake|||2024-01-14T14:35:18Z|||Congrats sis! BTW I'm graduating from Stanford next month with my CS degree. Job hunting is brutal though
primaryUser|||2024-01-14T14:36:42Z|||Jake that's amazing! I can refer you to Google if you want. They're always looking for new grads. I know the hiring manager for our team
Mom|||2024-01-14T14:37:55Z|||You kids are doing so well! Sarah, are you still doing those CrossFit classes? And Jake, remember to eat properly during finals
primaryUser|||2024-01-14T14:39:12Z|||Yes still doing CrossFit 4x a week! Actually competing in a local competition next month. And I'm training for the Bay to Breakers run in May
Dad|||2024-01-14T14:40:33Z|||That's my athletic daughter! Speaking of May, don't forget about cousin Emma's wedding on May 15th in Portland
primaryUser|||2024-01-14T14:41:44Z|||Oh right! I need to book flights. Alex and I will definitely be there. I'm actually the maid of honor so I need to plan her bachelorette party too
Jake|||2024-01-14T14:42:58Z|||Can't wait to see everyone! Sarah thanks for the Google referral offer, I'll definitely take you up on that. My GPA is 3.8 so hopefully that helps
primaryUser|||2024-01-14T14:44:15Z|||3.8 is great! I'll send you the referral link tomorrow. Also, I'm flying home for Easter weekend, can't wait to see everyone and Mom's famous lasagna!
|||
TAGS|family|career|personal_updates
`

	// Call the Content() method
	actual := doc.Content()

	// Compare the results
	if actual != expected {
		t.Errorf("Content() output mismatch")
		t.Errorf("Expected:\n%s", expected)
		t.Errorf("Actual:\n%s", actual)

		// Show line-by-line differences for easier debugging
		expectedLines := strings.Split(expected, "\n")
		actualLines := strings.Split(actual, "\n")

		maxLines := len(expectedLines)
		if len(actualLines) > maxLines {
			maxLines = len(actualLines)
		}

		for i := 0; i < maxLines; i++ {
			var expectedLine, actualLine string
			if i < len(expectedLines) {
				expectedLine = expectedLines[i]
			}
			if i < len(actualLines) {
				actualLine = actualLines[i]
			}

			if expectedLine != actualLine {
				t.Errorf("Line %d differs:", i+1)
				t.Errorf("  Expected: %q", expectedLine)
				t.Errorf("  Actual:   %q", actualLine)
			}
		}
	}
}
