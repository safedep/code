package main

import (
	"crypto/ecdh"
	"crypto/rand"

	"github.com/golang-jwt/jwt/v5"
	openai "github.com/sashabaranov/go-openai"
)

func main() {
	key, _ := ecdh.X25519().GenerateKey(rand.Reader)
	_ = key
	jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{})
	openai.NewClient("k").CreateChatCompletion(nil, openai.ChatCompletionRequest{})
	openai.NewClientWithConfig(openai.DefaultConfig("k")).Config.HTTPClient.Do(nil)
}
