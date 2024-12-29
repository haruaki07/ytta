package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
)

type RequestBody struct {
	Text string `json:"text"`
}

type SynthesizeResult struct {
	AudioContent string `json:"audioContent"`
}

func main() {
	accessToken := os.Getenv("GCLOUD_ACCESS_TOKEN")
	if accessToken == "" {
		log.Fatalln("error GCLOUD_ACCESS_TOKEN not found")
	}

	projectId := os.Getenv("GCLOUD_PROJECT_ID")
	if projectId == "" {
		log.Fatalln("error GCLOUD_PROJECT_ID not found")
	}

	mux := mux.NewRouter()

	mux.HandleFunc("/api/tts", func(w http.ResponseWriter, r *http.Request) {
		var body RequestBody
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		payload := map[string]any{
			"input": map[string]string{
				"text": body.Text,
			},
			"voice": map[string]string{
				"languageCode": "en-US",
				"name":         "en-US-Standard-A",
			},
			"audioConfig": map[string]string{
				"audioEncoding": "MP3",
			},
		}

		reqBody, err := json.Marshal(payload)
		if err != nil {
			log.Printf("error json marshal: %v\n", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		client := &http.Client{}

		req, err := http.NewRequest("POST", "https://texttospeech.googleapis.com/v1/text:synthesize", bytes.NewBuffer(reqBody))
		req.Header.Add("authorization", "Bearer "+accessToken)
		req.Header.Add("x-goog-user-project", projectId)
		req.Header.Add("content-type", "application/json; charset=utf-8")
		if err != nil {
			log.Printf("error make request: %v\n", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		res, err := client.Do(req)
		if err != nil {
			log.Printf("error request tts: %v\n", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			msg, _ := io.ReadAll(res.Body)
			log.Printf("error response tts: %s\n", msg)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		var ttsRes SynthesizeResult
		json.NewDecoder(res.Body).Decode(&ttsRes)

		dec, err := base64.StdEncoding.DecodeString(ttsRes.AudioContent)
		if err != nil {
			log.Printf("error decoding base64 string: %v\n", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		buf := bytes.NewBuffer(dec)

		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
		w.Header().Set("Content-Disposition", "inline; filename=\"audio.mp3\"")

		_, err = w.Write(buf.Bytes())
		if err != nil {
			log.Printf("error writing response: %v\n", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	})

	n := negroni.Classic()
	n.UseHandler(mux)

	fmt.Println("Listening HTTP Server at http://127.0.0.1:3000")
	log.Fatalln(http.ListenAndServe(":3000", n))
}
