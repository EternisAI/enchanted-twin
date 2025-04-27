package twitter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/g8rswimmer/go-twitter/v2"
	mcp_golang "github.com/metoro-io/mcp-golang"
)

const LIST_FEED_TOOL_NAME = "list_feed_tweets"
const POST_TWEET_TOOL_NAME = "post_tweet"
const SEARCH_TWEETS_TOOL_NAME = "search_tweets"

const LIST_FEED_TOOL_DESCRIPTION = "List the tweets from the user's feed"
const POST_TWEET_TOOL_DESCRIPTION = "Post a tweet"
const SEARCH_TWEETS_TOOL_DESCRIPTION = "Search for tweets"

type User struct {
	Data struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"data"`
}

func GetUser(accessToken string) (*User, error) {
	// Twitter API v2 endpoint for authenticated user
	url := "https://api.twitter.com/2/users/me?user.fields=username,name"

	// Create HTTP client and request
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set Authorization header with user access token
	req.Header.Set("Authorization", "Bearer "+accessToken)

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error: %s, Response: %s\n", resp.Status, string(body))
		return nil, fmt.Errorf("error getting user: %s", resp.Status)
	}

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}
	return &user, nil
}

type ListFeedTweetsArguments struct {
	PaginationToken string `json:"pagination_token" jsonschema:"required,description=The pagination token to start the list from, empty if first page"`
	Limit           int    `json:"limit" jsonschema:"required,description=The number of tweets to list"`
}

type PostTweetArguments struct {
	Content string `json:"content" jsonschema:"required,description=The content of the tweet"`
}

type SearchTweetsArguments struct {
	Query string `json:"query" jsonschema:"required,description=The query to search for"`
	Limit int    `json:"limit" jsonschema:"required,description=The number of tweets to search for"`
}

type authorize struct {
	Token string
}

func (a authorize) Add(req *http.Request) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", a.Token))
}

func processListFeedTweets(ctx context.Context, accessToken string, arguments ListFeedTweetsArguments) ([]*mcp_golang.Content, error) {

	client := &twitter.Client{
		Authorizer: authorize{
			Token: accessToken,
		},
		Client: http.DefaultClient,
		Host:   "https://api.twitter.com",
	}

	user, err := GetUser(accessToken)
	if err != nil {
		return nil, err
	}

	maxResults := 100
	if arguments.Limit > 5 {
		maxResults = arguments.Limit
	}

	feed, err := client.UserTweetReverseChronologicalTimeline(ctx, user.Data.ID, twitter.UserTweetReverseChronologicalTimelineOpts{
		MaxResults:      maxResults,
		PaginationToken: arguments.PaginationToken,
		TweetFields:     []twitter.TweetField{twitter.TweetFieldPublicMetrics, twitter.TweetFieldCreatedAt, twitter.TweetFieldAuthorID},
		UserFields:      []twitter.UserField{twitter.UserFieldUserName},
		Expansions:      []twitter.Expansion{twitter.ExpansionAuthorID},
	})

	if err != nil {
		fmt.Println("Error getting feed:", err)
		return nil, err
	}

	contents := []*mcp_golang.Content{}
	users := feed.Raw.Includes.UsersByID()
	for _, tweet := range feed.Raw.Tweets {
		author, ok := users[tweet.AuthorID]
		if !ok {
			continue
		}

		tweetURL := fmt.Sprintf("https://twitter.com/%s/status/%s", author.UserName, tweet.ID)
		contents = append(contents, &mcp_golang.Content{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: fmt.Sprintf("Tweet: %s\nCreated at: %s\nAuthor: %s\nLink: %s\n", tweet.Text, tweet.CreatedAt, tweet.AuthorID, tweetURL),
			},
		})
	}

	contents = append(contents, &mcp_golang.Content{
		Type: "text",
		TextContent: &mcp_golang.TextContent{
			Text: fmt.Sprintf("Next pagination token: %s", feed.Meta.NextToken),
		},
	})

	return contents, nil
}

func processPostTweet(_ string, _arguments PostTweetArguments) ([]*mcp_golang.Content, error) {

	fmt.Println("Posting tweet", _arguments.Content)

	return []*mcp_golang.Content{
		{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: "Posted tweet",
			},
		},
	}, nil
}

func processSearchTweets(ctx context.Context, accessToken string, arguments SearchTweetsArguments) ([]*mcp_golang.Content, error) {
	client := &twitter.Client{
		Authorizer: authorize{
			Token: accessToken,
		},
		Client: http.DefaultClient,
		Host:   "https://api.twitter.com",
	}

	limit := 10
	if arguments.Limit > 10 {
		limit = arguments.Limit
	}

	search, err := client.TweetRecentSearch(ctx, arguments.Query, twitter.TweetRecentSearchOpts{
		MaxResults:  limit,
		Expansions:  []twitter.Expansion{twitter.ExpansionAuthorID},
		TweetFields: []twitter.TweetField{twitter.TweetFieldPublicMetrics, twitter.TweetFieldCreatedAt, twitter.TweetFieldAuthorID},
	})

	if err != nil {
		return nil, err
	}

	contents := []*mcp_golang.Content{}
	for _, tweet := range search.Raw.Tweets {
		contents = append(contents, &mcp_golang.Content{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: fmt.Sprintf("Tweet: %s\nCreated at: %s\nAuthor: %s\n", tweet.Text, tweet.CreatedAt, tweet.AuthorID),
			},
		})
	}

	contents = append(contents, &mcp_golang.Content{
		Type: "text",
		TextContent: &mcp_golang.TextContent{
			Text: fmt.Sprintf("Next pagination token: %s", search.Meta.NextToken),
		},
	})

	return contents, nil
}
