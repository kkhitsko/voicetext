package voicetext

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
	"os"
)

type VoiceTextAPI struct {
	RefreshToken string
	AccessToken  string
	ClientID     string
	ClientSecret string
}

type OAuthSecretRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
}

type OAuthTokenRequest struct {
	ClientID     string `json:"client_id"`
	RefreshToken string `json:"refresh_token"`
	GrantType    string `json:"grant_type"`
}

type OAuthResponse struct {
	RefreshToken string `json:"refresh_token"`
	AcessToken   string `json:"access_token"`
	ExpiredTs    string `json:"expired_in"`
	Scope        struct {
		HasTts       int32 `json:"tts"`
		HasAsrShort  int32 `json:"asr_short"`
		HasAsrStream int32 `json:"asr_stream"`
	} `json:"scope"`
}

type TextRecognitionRequest struct {
	Qid    string `json:"qid"`
	Result struct {
		Texts []struct {
			Text           string  `json:"text"`
			Confidence     float64 `json:"confidence"`
			PunctuatedText string  `json:"punctuated_text"`
		} `json:"texts"`
		PhraseID string `json:"phrase_id"`
	} `json:"result"`
}

func NewVoiceTextAPI(id string, secret string) (*VoiceTextAPI, error) {
	api := &VoiceTextAPI{
		RefreshToken: "",
		AccessToken:  "",
		ClientID:     id,
		ClientSecret: secret,
	}

	return api, nil
}

func (api *VoiceTextAPI) Auth() (string, error) {
	var requestData interface{}
	if len(api.RefreshToken) == 0 {
		log.Printf("Refresh token is not set. Try to get new token")
		requestData = OAuthSecretRequest{api.ClientID, api.ClientSecret, "client_credentials"}
	} else {
		log.Printf("Reveive access token")
		requestData = OAuthTokenRequest{api.ClientID, api.RefreshToken, "client_credentials"}
	}
	log.Printf("Request data: %+v", requestData)
	b, err := json.Marshal(requestData)
	if err != nil {
		fmt.Errorf("Error while marshall request: %s", err.Error())
	}
	log.Printf("OAuth Request: %s", b)
	req, err := http.NewRequest("POST", "https://mcs.mail.ru/auth/oauth/v1/token", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var respJson OAuthResponse

	respJsonErr := json.Unmarshal(responseData, &respJson)
	if respJsonErr != nil {
		log.Panicf("Error while unmarshall response body: %s", respJsonErr.Error())
	}

	log.Printf("Response body: %+v", respJson)

	api.RefreshToken = respJson.RefreshToken
	api.AccessToken = respJson.AcessToken

	return respJson.AcessToken, err
}

func (api *VoiceTextAPI) Text2Voice(text string, fileID string) (string, error) {
	params := url.Values{}
	params.Add("text", text)
	params.Add("model_name", "maria")
	params.Add("encoder", "opus")
	params.Add("tempo", "0.9")

        ctx, cncl := context.WithTimeout(context.Background(), time.Second*120)
        defer cncl()

	uri := fmt.Sprintf("https://voice.mcs.mail.ru/tts?%s", params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", uri, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+api.AccessToken)
	req.Header.Set("Content-Type", "audio/ogg; codecs=opus")

	filename := fmt.Sprintf("voice/%s.oga", fileID)
	file, err := os.Create(filename)
	defer file.Close()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	log.Printf("Voice file content type: %s", resp.Header.Get("Content-Type"))
	log.Printf("Content size: %s; status: %d", resp.Status, resp.ContentLength)

	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	return filename, err
}

func (api *VoiceTextAPI) Voice2Text(file string) (string, error) {

	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

        ctx, cncl := context.WithTimeout(context.Background(), time.Second*120)
        defer cncl()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://voice.mcs.mail.ru/asr", f)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+api.AccessToken)
	req.Header.Set("Content-Type", "audio/ogg; codecs=opus")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	responseString := string(responseData)

	fmt.Println(responseString)
	var recognitionResponse TextRecognitionRequest

	jsonErr := json.Unmarshal(responseData, &recognitionResponse)
	if jsonErr != nil {
		log.Fatal(jsonErr)
		return "", jsonErr
	}
	if len(recognitionResponse.Result.Texts) == 0 {
		return "", fmt.Errorf("Fail_speech_voice")
	}
	if recognitionResponse.Result.Texts[0].PunctuatedText != "" {
		return recognitionResponse.Result.Texts[0].PunctuatedText, nil
	} else {
		return "", err
	}
}
