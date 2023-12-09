package main

import (
	"encoding/json"
	"fmt"
	"io"

	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/polly"
	"github.com/rs/zerolog/log"
)

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

// newMetaData returns a new VideoMeta struct with required credentials retrieved from ENV.
func newMetaData() (*VideoMeta, error) {
	log.Info().Msg("gather api credentials")
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

	log.Debug().Msg("generating voiceover a")
	if err := vm.generateVoiceOver(vm.Script, vm.ScriptVoFileLocation); err != nil {
		return fmt.Errorf("error when generating script vo: %v", err)
	}

	log.Debug().Msg("generating voiceover b")
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

	log.Debug().Msgf("saving voiceover to file: %v", outFile.Name())
	_, err = io.Copy(outFile, output.AudioStream)
	if err != nil {
		return fmt.Errorf("error saving MP3: %v", err)
	}

	return nil
}

func (vm *VideoMeta) getImageGenPrompt() ([]ImagePrompt, error) {
	/*
		"prompt_prompt.txt" is a chatGPT prompt used to generate the chatGPT prompt for the
		images describing the created video script.
		The prompt being passed in here uses text formatting ("%s") within it,
		which gets populated here using 'fmt.Sprintf'.
	*/
	prompt, err := os.ReadFile("prompt_prompt.txt")
	if err != nil {
		return nil, err
	}

	resp, err := GetScript(vm.Credentials.openAiToken, fmt.Sprintf(string(prompt), vm.Script))
	if err != nil {
		return nil, err
	}

	var promptResp struct {
		ImagePrompts []ImagePrompt `json:"prompts"`
	}
	if err := json.Unmarshal([]byte(resp), &promptResp); err != nil {
		return nil, err
	}

	if len(promptResp.ImagePrompts) == 0 {
		return nil, fmt.Errorf("no image prompts generated")
	}

	return promptResp.ImagePrompts, nil
}

// getVideoScript fetches a prompt from a file (prompt.txt within root directory),
// then sends request to OpenAI Chat completion to recieve a "script" for the video.
// This script is saved to (VideoMeta).Script
func (vm *VideoMeta) getVideoScript(prompt string) error {
	resp, err := GetScript(vm.Credentials.openAiToken, prompt)
	if err != nil {
		return err
	}

	vidScript := &VideoScript{}
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
