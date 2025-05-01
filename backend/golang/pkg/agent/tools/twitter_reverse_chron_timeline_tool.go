package tools

import (
	"context"
	"fmt"
	"net/http"
	"time"

	twitter "github.com/g8rswimmer/go-twitter/v2"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"golang.org/x/oauth2"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type dbAuthorizer struct {
	src       oauth2.TokenSource
	store     db.Store
	lastToken string
}

func (a *dbAuthorizer) Add(r *http.Request) {
	tok, err := a.src.Token()
	if err != nil {
		r.Header.Set("Authorization", "")
		return
	}
	r.Header.Set("Authorization", "Bearer "+tok.AccessToken)

	if tok.AccessToken != a.lastToken {
		_ = a.store.SetOAuthTokens(
			r.Context(),
			db.OAuthTokens{
				Provider:     "twitter",
				TokenType:    tok.TokenType,
				AccessToken:  tok.AccessToken,
				RefreshToken: tok.RefreshToken,
				ExpiresAt:    tok.Expiry,
			},
		)
		a.lastToken = tok.AccessToken
	}
}

type TwitterReverseChronologicalTimelineTool struct {
	store    db.Store
	oauthCfg *oauth2.Config
}

// Constructor now does **zero** token IO.
func NewTwitterTool(store db.Store) *TwitterReverseChronologicalTimelineTool {
	clientID := "bEFtUmtyNm1wUFNtRUlqQTdmQmE6MTpjaQ"
	return &TwitterReverseChronologicalTimelineTool{
		store: store,
		oauthCfg: &oauth2.Config{
			ClientID: clientID,
			Endpoint: oauth2.Endpoint{
				TokenURL: "https://api.twitter.com/2/oauth2/token",
			},
		},
	}
}

func (t *TwitterReverseChronologicalTimelineTool) Execute(
	ctx context.Context,
	_ map[string]any,
) (ToolResult, error) {
	token, err := t.store.GetOAuthTokens(ctx, "twitter")
	if err != nil {
		return ToolResult{}, fmt.Errorf("oauth tokens: %w", err)
	}

	if token == nil {
		return ToolResult{
			Content: "You must first connect your Twitter account to use this tool.",
		}, nil
	}

	out, err := t.GetReverseChronologicalTimeline(ctx, token)
	if err != nil {
		return ToolResult{}, fmt.Errorf("get reverse chronological timeline: %w", err)
	}

	return ToolResult{Content: out}, nil
}

func (t *TwitterReverseChronologicalTimelineTool) GetReverseChronologicalTimeline(
	ctx context.Context,
	token *db.OAuthTokens,
) (string, error) {
	base := &oauth2.Token{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.ExpiresAt,
	}

	src := oauth2.ReuseTokenSourceWithExpiry(
		base,
		t.oauthCfg.TokenSource(ctx, base),
		1*time.Minute, // refresh 60 s before expiry
	)
	auth := &dbAuthorizer{src: src, store: t.store, lastToken: token.AccessToken}
	httpClient := oauth2.NewClient(ctx, src)

	cli := &twitter.Client{
		Authorizer: auth,
		Client:     httpClient,
		Host:       "https://api.twitter.com",
	}

	meResp, err := cli.AuthUserLookup(ctx, twitter.UserLookupOpts{
		UserFields: []twitter.UserField{twitter.UserFieldID},
	})
	if err != nil {
		return "", err
	}
	var meID string
	for id := range meResp.Raw.UserDictionaries() {
		meID = id
		break
	}

	resp, err := cli.UserTweetReverseChronologicalTimeline(
		ctx,
		meID,
		twitter.UserTweetReverseChronologicalTimelineOpts{
			MaxResults: 100,
			Expansions: []twitter.Expansion{
				twitter.ExpansionAuthorID,
				twitter.ExpansionReferencedTweetsID,
				twitter.ExpansionReferencedTweetsIDAuthorID,
			},
			TweetFields: []twitter.TweetField{
				twitter.TweetFieldCreatedAt,
				twitter.TweetFieldText,
				twitter.TweetFieldAuthorID,
				twitter.TweetFieldReferencedTweets,
			},
			UserFields: []twitter.UserField{
				twitter.UserFieldName,
				twitter.UserFieldUserName,
			},
		},
	)
	if err != nil {
		return "", err
	}

	tweets := resp.Raw.TweetDictionaries()
	users := resp.Raw.Includes.UsersByID()

	var out string
	for _, td := range tweets {
		tw := td.Tweet
		author, ok := users[tw.AuthorID]
		if !ok {
			continue
		}

		// Retweet?
		if len(tw.ReferencedTweets) > 0 && tw.ReferencedTweets[0].Type == "retweeted" {
			rt := tw.ReferencedTweets[0]
			orig, ok := tweets[rt.ID]
			if !ok {
				continue
			}
			origAuthor := users[orig.Tweet.AuthorID]
			out += fmt.Sprintf("%s [Retweet] %s (@%s) → %s (@%s): %s\n\n",
				tw.CreatedAt,
				author.Name, author.UserName,
				origAuthor.Name, origAuthor.UserName,
				orig.Tweet.Text)
		} else {
			out += fmt.Sprintf("%s [Tweet] %s (@%s): %s\n\n",
				tw.CreatedAt,
				author.Name, author.UserName,
				tw.Text)
		}
	}

	return out, nil
}

func (t *TwitterReverseChronologicalTimelineTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name: "twitter_get_timeline",
			Description: param.NewOpt(
				"Return the user's most recent Twitter home-timeline tweets.",
			),
		},
	}
}
