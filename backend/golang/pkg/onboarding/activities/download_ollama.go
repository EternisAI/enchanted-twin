package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
	ollamaapi "github.com/ollama/ollama/api"
)

type OnboardingActivities struct {
	ollamaClient *ollamaapi.Client
	nc           *nats.Conn
}

func NewOnboardingActivities(ollamaClient *ollamaapi.Client, nc *nats.Conn) *OnboardingActivities {
	return &OnboardingActivities{
		ollamaClient: ollamaClient,
		nc:           nc,
	}
}

// This would be for a GraphQL subscription (optional)
type DownloadModelProgress struct {
	PercentageProgress float64
}

func (a *OnboardingActivities) DownloadOllamaModel(ctx context.Context) error {
	modelName := "gemma3:1b"

	models, err := a.ollamaClient.List(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	modelFound := false
	for _, model := range models.Models {
		if model.Name == modelName {
			modelFound = true
			break
		}
	}

	if modelFound {
		fmt.Println("Model already downloaded")
		return nil
	}

	req := &ollamaapi.PullRequest{
		Model: modelName,
	}

	pullProgressFunc := func(progress ollamaapi.ProgressResponse) error {
		if progress.Total == 0 {
			return nil
		}

		percentageProgress := float64(progress.Completed) / float64(progress.Total) * 100

		fmt.Println("Download progress ", percentageProgress)
		userMessageJson, err := json.Marshal(DownloadModelProgress{
			PercentageProgress: percentageProgress,
		})
		if err != nil {
			return err
		}

		err = a.nc.Publish("onboarding.download_model.progress", userMessageJson)
		if err != nil {
			return err
		}

		return nil
	}

	err = a.ollamaClient.Pull(context.Background(), req, pullProgressFunc)
	if err != nil {
		return err
	}

	fmt.Println("Model downloaded")

	return nil
}
