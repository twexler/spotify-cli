package login

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"spotify/pkg"
	"spotify/pkg/model"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	//go:embed success.html
	successHTML string
	//go:embed failure.html
	failureHTML string
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Log in to Spotify.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			token, err := authorize()
			if err != nil {
				return err
			}

			if err := SaveToken(token); err != nil {
				return err
			}

			cmd.Println("Success!")
			return nil
		},
	}
}

func SaveToken(token *model.Token) error {
	// Save token
	viper.Set("token", token.AccessToken)

	// Save expiration
	expiration := time.Now().Unix() + int64(token.ExpiresIn)
	viper.Set("expiration", expiration)

	// Save refresh token
	viper.Set("refresh_token", token.RefreshToken)

	return viper.WriteConfig()
}

func authorize() (*model.Token, error) {
	// https://developer.spotify.com/documentation/general/guides/authorization-guide/#authorization-code-flow-with-proof-key-for-code-exchange-pkce

	// 1. Create the code verifier and challenge
	verifier, challenge := pkg.StartProof()

	// 2. Construct the authorization URI
	uri := pkg.BuildAuthURI(challenge)

	// 3. Your app redirects the user to the authorization URI
	if err := exec.Command(findOpenCommand(), uri).Run(); err != nil {
		return nil, err
	}

	code, err := listenForCode()
	if err != nil {
		return nil, err
	}

	// 4. Your app exchanges the code for an access token
	token, err := pkg.RequestToken(code, verifier)
	if err != nil {
		return nil, err
	}

	return token, err
}

func findOpenCommand() string {
	switch os := runtime.GOOS; os {
	case "linux":
		return "xdg-open"
	default:
		return "open"
	}
}

func listenForCode() (string, error) {
	server := &http.Server{Addr: ":1024"}

	var code string
	var err error

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("error") == "" {
			code = r.URL.Query().Get("code")
			fmt.Fprintln(w, successHTML)
		} else {
			err = errors.New("Login failed.")
			fmt.Fprintln(w, failureHTML)
		}

		// Use a separate thread so browser doesn't show a "No Connection" message
		go func() {
			server.Shutdown(context.TODO())
		}()
	})

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return "", err
	}

	return code, err
}