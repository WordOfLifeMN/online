package gclient

// methdos for accessing and interacting with Google API

import (
	"context"
	"fmt"

	"net/http"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// CRITICAL: This scope MUST remain read-only.
//
// The spreadsheet is the hand-maintained source of record for the entire media
// catalog, and it has no meaningful undo. This application only ever reads it - every
// Sheets call in the codebase is a .Get - so a read-only credential costs nothing and
// means a bug or a future change that tries to write fails at the API instead of
// silently damaging live data.
//
// Do NOT change this to sheets.SpreadsheetsScope (read/write) or any other writable
// scope as an incidental part of some other change. If writing to the sheet is ever
// genuinely wanted, raise it, discuss it, and get explicit approval first.
const sheetsScope = sheets.SpreadsheetsReadonlyScope

func GetGoogleClient(ctx context.Context) (*http.Client, error) {
	credentialFile, err := getCredentialFile()
	if err != nil {
		return nil, err
	}
	credentials, err := os.ReadFile(credentialFile)
	if err != nil {
		return nil, fmt.Errorf("Cannot read credentials from %s: %w", credentialFile, err)
	}
	config, err := google.JWTConfigFromJSON(credentials, sheetsScope)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse client JWT file to config: %w", err)
	}
	return config.Client(ctx), nil
}

func GetSheetService(ctx context.Context) (*sheets.Service, error) {
	client, err := GetGoogleClient(ctx)
	if err != nil {
		return nil, err
	}
	service, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve Sheets client: %W", err)
	}

	return service, nil
}

// getCredentialFile finds the file that contains the credentials. Looks for the
// file $HOME/.wolm/credentials.json
func getCredentialFile() (string, error) {
	// Find home directory.
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	credentialFile := home + "/.wolm/credentials.json"
	_, err = os.Stat(credentialFile)
	if err != nil {
		return "", err
	}

	return credentialFile, nil
}
