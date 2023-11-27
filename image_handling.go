package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"os"

	"golang.org/x/image/webp"
)

func base64toPNG(b64Data, filepath string) error {
	decodedB64, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return fmt.Errorf("err decoding b64: %v", err)
	}

	// fmt.Printf("Content type: %v\n", http.DetectContentType(decodedB64))

	im, err := webp.Decode(bytes.NewReader(decodedB64))
	if err != nil {
		return fmt.Errorf("err decoding b64 via png: %v", err)
	}

	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("err creating file: %v", err)
	}
	defer f.Close()

	//Encode to PNG
	if err := png.Encode(f, im); err != nil {
		return fmt.Errorf("err encoding as png: %v", err)
	}

	if err := f.Sync(); err != nil {
		return fmt.Errorf("err syncing file: %v", err)
	}

	return nil
}
