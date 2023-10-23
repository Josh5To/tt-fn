package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/polly"
	openai "github.com/sashabaranov/go-openai"
)

const (
	AUDIO_FILE_NAME = "voiceover_a"
	TEST_VO_STRING  = "Yeah this is just a test to verify that the file writing works. Thanks."
)

type AuthenticationData struct {
	openAiToken  string
	awsAccessId  string
	awsAccessKey string
}

type FakeFactResponse struct {
	Fact    string `json:"FakeFact"`
	SignOff string `json:"SignOff"`
}

// VideoMeta holds various data necessary to create our video.
type VideoMeta struct {
	AwsSession     *session.Session
	Credentials    *AuthenticationData
	Images         *[]os.File
	Script         string
	SignOff        string
	VOFileLocation string
}

func main() {
	videoMeta, err := newMetaData()
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	// Get prompt from file, generate out video script
	if err := videoMeta.getVideoScript(); err != nil {
		log.Fatalf("%v\n", err)
	}

	//Initialize our AWS session
	if err := videoMeta.initAwsSession(); err != nil {
		log.Fatalf("%v\n", err)
	}

	//Generate our voice over mp3 from AWS Polly
	if err := videoMeta.generateVoiceOver(); err != nil {
		log.Fatalf("%v\n", err)
	}

	// url, err := GenerateFrames(videoMeta.Credentials.openAiToken, "")
	// if err != nil {
	// 	log.Fatalf("%v\n", err)
	// }

	// fmt.Printf("image url: %v\n", url)
	log.Printf("finished operation")
}

func generateFrames(apiToken, prompt string) (string, error) {
	ctx := context.Background()
	client := openai.NewClient(apiToken)

	resp, err := client.CreateImage(ctx, openai.ImageRequest{
		Prompt:         "Design a plausible image reflecting a historical setting in 1927 when Sir Frederick Marmalade conducted an experiment. Depict cats reading and writing poetry in a realistic laboratory environment. Portray the cats in a way that suggests their secret literary abilities, without losing the sense of reality",
		N:              1,
		Size:           openai.CreateImageSize512x512,
		ResponseFormat: openai.CreateImageResponseFormatURL,
		User:           "",
	})

	if err != nil {
		return "", fmt.Errorf("API ImageRequest error: %v\n", err)
	}

	url := resp.Data[0].URL
	return url, nil
}

func GetScript(apiToken, prompt string) (string, error) {
	client := openai.NewClient(apiToken)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return "", fmt.Errorf("ChatCompletion error: %v\n", err)
	}

	return resp.Choices[0].Message.Content, nil
}

func getCredentials() (*AuthenticationData, error) {
	token := os.Getenv("OAI_KEY")
	awsAccessKey := os.Getenv("AWS_KEY")
	awsAccessID := os.Getenv("AWS_ID")

	switch {
	case token == "":
		return nil, fmt.Errorf("Open AI Token not retrieved from ENV")
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

// newMetaData returns a new VideoMeta struct with required credentials retrieved from ENV.
func newMetaData() (*VideoMeta, error) {
	authData, err := getCredentials()
	if err != nil {
		return nil, err
	}

	vmd := new(VideoMeta)
	vmd.Credentials = authData
	return vmd, nil
}

// Request a generated voice over for each scene
func (vm *VideoMeta) generateVoiceOver() error {
	// Create Polly client
	svc := polly.New(vm.AwsSession)

	vm.VOFileLocation = fmt.Sprintf("%s.%s", AUDIO_FILE_NAME, "mp3")

	// Output to MP3 using voice Gregory
	input := &polly.SynthesizeSpeechInput{
		Engine:       aws.String("neural"),
		OutputFormat: aws.String("mp3"),
		Text:         &vm.Script,
		VoiceId:      aws.String("Gregory"),
	}

	output, err := svc.SynthesizeSpeech(input)
	if err != nil {
		return fmt.Errorf("error calling SynthesizeSpeech: %v", err)
	}

	outFile, err := os.Create(vm.VOFileLocation)
	if err != nil {
		return fmt.Errorf("error creating %s: %v\n", vm.VOFileLocation, err)
	}

	vm.VOFileLocation = outFile.Name()

	defer outFile.Close()
	_, err = io.Copy(outFile, output.AudioStream)
	if err != nil {
		return fmt.Errorf("error saving MP3: %v\n", err)
	}

	return nil
}

// getVideoScript fetches a prompt from a file (prompt.txt within root directory),
// then sends request to OpenAI Chat completion to recieve a "script" for the video.
// This script is saved to (VideoMeta).Script
func (vm *VideoMeta) getVideoScript() error {
	prompt, err := os.ReadFile("prompt.txt")
	if err != nil {
		return err
	}

	resp, err := GetScript(vm.Credentials.openAiToken, string(prompt))
	if err != nil {
		return err
	}

	vidScript := new(FakeFactResponse)

	if err := json.Unmarshal([]byte(resp), vidScript); err != nil {
		return err
	}

	if vidScript == nil {
		return fmt.Errorf("no data received from text generation to be unmarshalled")
	}
	vm.Script = vidScript.Fact
	vm.SignOff = vidScript.SignOff
	return nil
}

func (vm *VideoMeta) initAwsSession() error {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-west-2"),
		Credentials: credentials.NewStaticCredentials(vm.Credentials.awsAccessId, vm.Credentials.awsAccessKey, ""),
	})
	if err != nil {
		return err
	}

	vm.AwsSession = sess
	return nil
}
