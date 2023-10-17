package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

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

func main() {
	token := os.Getenv("OAI_KEY")
	awsAccessKey := os.Getenv("AWS_KEY")
	awsAccessID := os.Getenv("AWS_ID")

	//get our super-secret AI prompt from file.
	prompt, err := os.ReadFile("prompt.txt")
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	resp, err := GetScript(token, string(prompt))
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-west-2"),
		Credentials: credentials.NewStaticCredentials(awsAccessID, awsAccessKey, ""),
	})
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	if err := CreateVoiceoverAudio(resp, sess); err != nil {
		log.Fatalf("%v\n", err)
	}
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

func CreateVoiceoverAudio(text string, awsSesh *session.Session) error {
	// Create Polly client
	svc := polly.New(awsSesh)

	// Output to MP3 using voice Joanna
	input := &polly.SynthesizeSpeechInput{OutputFormat: aws.String("mp3"), Text: &text, VoiceId: aws.String("Gregory"), Engine: aws.String("neural")}

	output, err := svc.SynthesizeSpeech(input)
	if err != nil {
		return fmt.Errorf("error calling SynthesizeSpeech: %v", err)
	}

	// Save as MP3
	names := strings.Split(AUDIO_FILE_NAME, ".")
	name := names[0]
	mp3File := name + ".mp3"

	outFile, err := os.Create(mp3File)
	if err != nil {
		return fmt.Errorf("error creating %s: %v\n", mp3File, err)
	}

	defer outFile.Close()
	_, err = io.Copy(outFile, output.AudioStream)
	if err != nil {
		return fmt.Errorf("error saving MP3: %v\n", err)
	}

	return nil
}
