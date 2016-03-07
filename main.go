package main

import (
	"fmt"
	"os"

	"github.com/alexflint/go-arg"

	"github.com/petrkotek/go-google-contacts/contacts"
)

func main() {
	var args struct {
		ClientID     string `arg:"required,env:CLIENT_ID,help:Client ID from Google Developer Console. Also can be provided as CLIENT_ID env variable."`
		ClientSecret string `arg:"required,env:CLIENT_SECRET,help:Client secret from Google Developer Console. Also can be provided as CLIENT_SECRET env variable."`
		Raw          bool   `arg:"help:Display raw XML response"`
		AuthFile     string `arg:"required,-A,help:Path to JSON file with refresh_token e.g. {\"refresh_token\": \"XYZ\"}"`
		Command      string `arg:"required,positional,help:fetch_feed|fetch|update_nickname"`
		ContactId    string `arg:"positional,help:Contact ID to fetch/update"`
		Value        string `arg:"positional,help:New value for updated field"`
	}
	arg.MustParse(&args)

	apiClient := contacts.NewClient(&contacts.StandardAuthManager{
		AuthStorage: &contacts.FileAuthStorage{args.AuthFile},
		AccessTokenRetriever: &contacts.StandardAccessTokenRetriever{
			ClientID:     args.ClientID,
			GoogleSecret: args.ClientSecret,
		},
	})

	switch args.Command {
	case "fetch_feed":
		if args.Raw {
			data, err := apiClient.FetchFeedRaw()
			if err != nil {
				fmt.Println("error: ", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
		} else {
			feed, err := apiClient.FetchFeed()
			if err != nil {
				fmt.Println("error: ", err)
				os.Exit(1)
			}
			for i, entry := range feed.Entries {
				fmt.Println("ENTRY ", i, ":")
				fmt.Printf("%+v\n\n", entry)
			}
		}

	case "fetch":
		if args.ContactId == "" {
			fmt.Println("Contact ID must be specified")
			os.Exit(1)
		}
		if args.Raw {
			data, err := apiClient.FetchContactRaw(args.ContactId)
			if err != nil {
				fmt.Println("error: ", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
		} else {
			entry, err := apiClient.FetchContact(args.ContactId)
			if err != nil {
				fmt.Println("error: ", err)
				os.Exit(1)
			}
			fmt.Println("ENTRY:")
			fmt.Printf("%+v\n\n", entry)
		}

	case "update_nickname":
		if args.Raw {
			fmt.Println("update_nickname doesn't support --raw")
			os.Exit(1)
		}
		if args.ContactId == "" {
			fmt.Println("Contact ID must be specified")
			os.Exit(1)
		}
		entry, err := apiClient.FetchContact(args.ContactId)
		if err != nil {
			fmt.Println("error: ", err)
			os.Exit(1)
		}
		fmt.Println("ORIGINAL ENTRY:")
		fmt.Printf("%+v\n\n", entry)

		entry.Nickname = args.Value
		updatedEntry, err := apiClient.Save(entry)

		if err != nil {
			fmt.Println("error: " + err.Error())
			os.Exit(1)
		}
		fmt.Println("UPDATED ENTRY:")
		fmt.Printf("%+v\n", updatedEntry)
	}
}
