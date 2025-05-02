package main

import (
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
)

func main() {
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("one"),
		openai.UserMessage("two"),
	}

	input, err := json.Marshal(messages)
	if err != nil {
		panic(err)
	}

	var unmarshalledMessages []openai.ChatCompletionMessageParamUnion

	err = json.Unmarshal(input, &unmarshalledMessages)
	if err != nil {
		panic(err)
	}
	fmt.Println(unmarshalledMessages)
}
