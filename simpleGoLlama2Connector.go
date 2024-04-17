package simplegollama2connector

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type llama2Request struct {
	Stream bool        `json:"stream"`
	Input  llama2input `json:"input"`
}

type llama2input struct {
	Top_k            int     `json:"top_k"`
	Top_p            int     `json:"top_p"`
	Prompt           string  `json:"prompt"`
	Temperature      float64 `json:"temperature"`
	System_prompt    string  `json:"system_prompt"`
	Length_penalty   int     `json:"length_penalty"`
	Max_new_tokens   int     `json:"max_new_tokens"`
	Prompt_template  string  `json:"prompt_template"`
	Presence_penalty int     `json:"presence_penalty"`
}

type Llama2UrlResponse struct {
	Urls struct {
		Get    string `json:"get"`
		Stream string `json:"stream"`
	} `json:"urls"`
}

func NewDefaultRequest(prompt, sysprompt string) llama2Request {
	return llama2Request{true, llama2input{
		0,
		1,
		prompt,
		0.75,
		sysprompt,
		1,
		500,
		"<s>[INST] <<SYS>>\n{system_prompt}\n<</SYS>>\n\n{prompt} [/INST]",
		0,
	}}
}

func NewRequest(
	Top_k *int,
	Top_p *int,
	Prompt string,
	Temperature *float64,
	System_prompt string,
	Length_penalty *int,
	Max_new_tokens *int,
	Prompt_template *string,
	Presence_penalty *int,
) llama2Request {
	return llama2Request{true, llama2input{
		func() int {
			if Top_k == nil {
				return 0
			}
			return *Top_k
		}(),
		func() int {
			if Top_p == nil {
				return 1
			}
			return *Top_p
		}(),
		Prompt,
		func() float64 {
			if Temperature == nil {
				return 0.75
			}
			return *Temperature
		}(),
		System_prompt,
		func() int {
			if Length_penalty == nil {
				return 1
			}
			return *Length_penalty
		}(),
		func() int {
			if Max_new_tokens == nil {
				return 500
			}
			return *Max_new_tokens
		}(),
		func() string {
			if Prompt_template == nil {
				return "<s>[INST] <<SYS>>\n{system_prompt}\n<</SYS>>\n\n{prompt} [/INST]"
			}
			return *Prompt_template
		}(),
		func() int {
			if Presence_penalty == nil {
				return 0
			}
			return *Presence_penalty
		}(),
	}}
}

type Llama213bChatConnector struct {
	ApiUrl string
	ApiKey string
}

func NewLlama213bChatConnector(ApiUrl, ApiKey string) Llama213bChatConnector {
	return Llama213bChatConnector{ApiUrl, ApiKey}
}

func (c *Llama213bChatConnector) PostPromptAndReturnStreamUrl(requestInput llama2Request) (string, error) {
	client := &http.Client{}
	requestBodyJson, err := json.Marshal(requestInput)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequest("POST", c.ApiUrl, bytes.NewReader(requestBodyJson))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+c.ApiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	resBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	var unMarshalledResBodyerr Llama2UrlResponse
	err = json.Unmarshal(resBody, &unMarshalledResBodyerr)
	if err != nil {
		return "", err
	}
	return unMarshalledResBodyerr.Urls.Stream, nil
}

func (c *Llama213bChatConnector) GetPromptResults(url string, outputChan chan string, errorChan chan error) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		errorChan <- err
		return
	}
	request.Header.Set("Authorization", "Bearer "+c.ApiKey)
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Cache-Control", "no-store")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		errorChan <- err
		return
	}

	defer func() {
		response.Body.Close()
		close(outputChan)
	}()

	scanner := bufio.NewScanner(response.Body)
	for {
		data, err := convertShittyFormatedTextToLessShittyString(string(scanner.Bytes()))
		if err == EndOfOutputError {
			errorChan <- err
			return
		}
		if err == nil {
			outputChan <- data
		}
		if !scanner.Scan() {
			errorChan <- err
			return
		}
	}
}

func convertShittyFormatedTextToLessShittyString(text string) (string, error) {
	if strings.Contains(text, "data:") {
		if text[6:] == "{}" {
			return "", EndOfOutputError
		}
		return text[6:], nil
	}
	return "", NotDataError
}

var (
	EndOfOutputError = errors.New("End of output")
	NotDataError     = errors.New("Not data")
)
