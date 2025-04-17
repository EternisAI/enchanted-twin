package x

type Tweet struct {
	Tweet struct {
		CreatedAt     string `json:"created_at"`
		ID            string `json:"id_str"`
		FullText      string `json:"full_text"`
		RetweetCount  string `json:"retweet_count"`
		FavoriteCount string `json:"favorite_count"`
		Lang          string `json:"lang"`
	} `json:"tweet"`
}
