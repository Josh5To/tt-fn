package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/sync/errgroup"
)

const (
	MAIN_AUDIO_FILE_NAME = "voiceover_a"
	END_AUDIO_FILE_NAME  = "voiceover_b"
	IMAGE_FILE_PREFIX    = "vid_frame"
)

type AuthenticationData struct {
	openAiToken  string
	awsAccessId  string
	awsAccessKey string
}

type VideoScript struct {
	Fact    string `json:"FakeFact"`
	SignOff string `json:"SignOff"`
}

type ImagePrompt struct {
	Prompt string `json:"imagePrompt"`
}

// chatGPT prompt used to generate the json payload for our video. Containing main script and a signoff.
var (
	debugFlag        *bool
	scriptPromptFlag *string
)

func init() {
	scriptPromptFlag = flag.String("prompt", "",
		"Use this to pass in a prompt tasking chatGPT with writing a narration script for a video. (Story, Documentary, etc.)")
	debugFlag = flag.Bool("debug", false, "Set logger to debug level.")
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debugFlag {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	videoMeta, err := newMetaData()
	if err != nil {
		log.Fatal().
			Err(err).
			Msgf("%v\n", err)
	}

	// Get prompt from file, generate our video script and 'signoff' message
	log.Info().Msg("Getting video script...")
	if err := videoMeta.getVideoScript(*scriptPromptFlag); err != nil {
		log.Fatal().
			Err(err).
			Msgf("Error getting video script: %v\n", err)
	}

	log.Info().Msgf("video script completed: %s", videoMeta.Script)

	//Initialize our AWS session
	log.Info().Msg("Initializing AWS Session...")
	if err := videoMeta.initAwsSession(); err != nil {
		log.Fatal().
			Err(err).
			Msgf("Error initiation aws session: %v\n", err)
	}

	log.Info().Msg("Creating voiceovers...")
	if err := videoMeta.CreateVoiceOvers(); err != nil {
		log.Fatal().
			Err(err).
			Msgf("Error creating voice overs: %v\n", err)
	}

	log.Info().Msg("Getting image prompt...")
	imagePrompt, err := videoMeta.getImageGenPrompt()
	if err != nil {
		log.Fatal().
			Err(err).
			Msgf("Error getting image prompt: %v\n", err)
	}

	log.Info().Msg("Generating video frames...")
	images, err := generateFrames(videoMeta.Credentials.openAiToken, imagePrompt)
	if err != nil {
		log.Fatal().
			Err(err).
			Msgf("Error generating frames: %v\n", err)
	}

	if err := saveFrames(images); err != nil {
		log.Fatal().
			Err(err).
			Msgf("%v\n", err)
	}

	log.Info().Msg("finished operation")
}

func GetScript(apiToken, prompt string) (string, error) {
	client := openai.NewClient(apiToken)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4TurboPreview,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			ResponseFormat: &openai.ChatCompletionResponseFormat{openai.ChatCompletionResponseFormatTypeJSONObject},
		},
	)

	if err != nil {
		return "", fmt.Errorf("ChatCompletion error: %v", err)
	}

	return resp.Choices[0].Message.Content, nil
}

func generateFrames(apiToken string, prompts []ImagePrompt) ([]openai.ImageResponse, error) {
	ctx := context.Background()
	client := openai.NewClient(apiToken)
	g, ctx := errgroup.WithContext(ctx)

	//Let's just make sure we only do max four loops. 2 cents a piece for these images!
	var responses = make([]openai.ImageResponse, (len(prompts) - 1))

	for i, prompt := range prompts {
		if i < 3 {
			//For the bug
			i := i
			prompt := prompt
			g.Go(func() error {
				resp, err := client.CreateImage(ctx, openai.ImageRequest{
					Prompt:         prompt.Prompt,
					N:              1,
					Model:          openai.CreateImageModelDallE3,
					Size:           openai.CreateImageSize1024x1024,
					ResponseFormat: openai.CreateImageResponseFormatB64JSON,
					User:           "tt-fn",
				})
				if err != nil {
					return err
				}
				log.Debug().Msgf("imagery created with base64 size: %d", len(resp.Data[0].B64JSON))
				responses[i] = resp
				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("API ImageRequest error: %v", err)
	}

	return responses, nil
}

func getCredentials() (*AuthenticationData, error) {
	token := os.Getenv("OAI_KEY")
	awsAccessKey := os.Getenv("AWS_KEY")
	awsAccessID := os.Getenv("AWS_ID")
	switch {
	case token == "":
		return nil, fmt.Errorf("access token for Open AI not retrieved from ENV")
	case awsAccessKey == "":
		return nil, fmt.Errorf("AWS access key not retrieved from ENV")
	case awsAccessID == "":
		return nil, fmt.Errorf("AWS access ID not retrieved from ENV")
	default:
		return &AuthenticationData{
			openAiToken:  token,
			awsAccessId:  awsAccessID,
			awsAccessKey: awsAccessKey,
		}, nil
	}
}

func saveFrames(imageData []openai.ImageResponse) error {
	//Set the image glob
	for i, img := range imageData {
		filePth := fmt.Sprintf("%s_%d.png", IMAGE_FILE_PREFIX, i)
		if err := base64toPNG(img.Data[0].B64JSON, filePth); err != nil {
			return err
		}
	}
	return nil
}
