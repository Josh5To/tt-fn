package main

import (
	"context"
	"encoding/base64"
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
	"golang.org/x/sync/errgroup"
)

const (
	MAIN_AUDIO_FILE_NAME = "voiceover_a"
	END_AUDIO_FILE_NAME  = "voiceover_b"
	IMAGE_FILE_PREFIX    = "vid_frame"
	TEST_VO_STRING       = "Yeah this is just a test to verify that the file writing works. Thanks."
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

type ImagePrompt struct {
	Prompt string `json:"imagePrompt"`
}

// VideoMeta holds various data necessary to create our video.
// "Script" and "SignOff" separated so we can better control the transitions between 'main' video segment and ending.
type VideoMeta struct {
	AwsSession            *session.Session
	Credentials           *AuthenticationData
	SignOffFrame          *os.File
	FrameGlob             string
	Script                string
	SignOff               string
	ScriptVoFileLocation  string
	SignOffVoFileLocation string
}

func main() {
	videoMeta, err := newMetaData()
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	// Get prompt from file, generate our video script and 'signoff' message
	if err := videoMeta.getVideoScript(); err != nil {
		log.Fatalf("%v\n", err)
	}

	//Initialize our AWS session
	if err := videoMeta.initAwsSession(); err != nil {
		log.Fatalf("%v\n", err)
	}

	if err := videoMeta.CreateVoiceOvers(); err != nil {
		log.Fatalf("%v\n", err)
	}

	imagePrompt, err := videoMeta.getImageGenPrompt()
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	images, err := generateFrames(videoMeta.Credentials.openAiToken, imagePrompt)
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	if err := saveFrames(images); err != nil {
		log.Fatalf("%v\n", err)
	}

	// //TODO: Do something with the images
	// for _, image := range images {
	// 	fmt.Printf("image url: %v\n", image.Data[0].URL)
	// }

	log.Printf("finished operation")
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
					Size:           openai.CreateImageSize512x512,
					ResponseFormat: openai.CreateImageResponseFormatB64JSON,
					User:           "tt-fn",
				})
				if err != nil {
					return err
				}
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

func saveFrames(imageData []openai.ImageResponse) error {
	//Set the image glob
	for i, img := range imageData {
		filePth := fmt.Sprintf("%s_%b.png", IMAGE_FILE_PREFIX, i)

		dec, err := base64.StdEncoding.DecodeString(img.Data[0].B64JSON)
		if err != nil {
			return err
		}

		f, err := os.Create(filePth)
		if err != nil {
			return err
		}
		defer f.Close()

		di, err := f.Write(dec)
		if err != nil {
			return err
		}
		if err := f.Sync(); err != nil {
			return err
		}

		fmt.Printf("Image Data written: %v\n", di)
	}
	return nil
}

func GetScript(apiToken, prompt string) (string, error) {
	client := openai.NewClient(apiToken)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return "", fmt.Errorf("ChatCompletion error: %v", err)
	}

	return resp.Choices[0].Message.Content, nil
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

// Send request to generate voice over mp3s for our script and signoff.
func (vm *VideoMeta) CreateVoiceOvers() error {
	//Generate our voice over mp3s from AWS Polly
	vm.ScriptVoFileLocation = fmt.Sprintf("%s.%s", MAIN_AUDIO_FILE_NAME, "mp3")
	vm.SignOffVoFileLocation = fmt.Sprintf("%s.%s", END_AUDIO_FILE_NAME, "mp3")

	if err := vm.generateVoiceOver(vm.Script, vm.ScriptVoFileLocation); err != nil {
		return fmt.Errorf("error when generating script vo: %v", err)
	}

	if err := vm.generateVoiceOver(vm.SignOff, vm.SignOffVoFileLocation); err != nil {
		return fmt.Errorf("error when generating signoff vo: %v", err)
	}

	return nil
}

// Request a generated voice over for each scene
func (vm *VideoMeta) generateVoiceOver(script, filepath string) error {
	// Create Polly client
	svc := polly.New(vm.AwsSession)

	// Output to MP3 using voice Gregory
	input := &polly.SynthesizeSpeechInput{
		Engine:       aws.String("neural"),
		OutputFormat: aws.String("mp3"),
		Text:         aws.String(script),
		VoiceId:      aws.String("Gregory"),
	}

	output, err := svc.SynthesizeSpeech(input)
	if err != nil {
		return fmt.Errorf("error calling SynthesizeSpeech: %v", err)
	}

	outFile, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", filepath, err)
	}

	defer outFile.Close()
	_, err = io.Copy(outFile, output.AudioStream)
	if err != nil {
		return fmt.Errorf("error saving MP3: %v", err)
	}

	return nil
}

func (vm *VideoMeta) getImageGenPrompt() ([]ImagePrompt, error) {
	prompt, err := os.ReadFile("prompt_prompt.txt")
	if err != nil {
		return nil, err
	}
	fmt.Printf(string(prompt), vm.Script)

	resp, err := GetScript(vm.Credentials.openAiToken, fmt.Sprintf(string(prompt), vm.Script))
	if err != nil {
		return nil, err
	}

	var promptResp []ImagePrompt

	if err := json.Unmarshal([]byte(resp), &promptResp); err != nil {
		return nil, err
	}

	if len(promptResp) == 0 {
		return nil, fmt.Errorf("no image prompts generated")
	}

	return promptResp, nil
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

	vidScript := &FakeFactResponse{}

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
