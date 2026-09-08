package oakestra_test

import (
	"context"
	"fmt"
	"log"

	oakestra "github.com/oakestra/oakestra-cli/oakestra-go"
)

func Example() {
	client, err := oakestra.NewClient(
		oakestra.WithBaseURL(oakestra.BaseURLForHost("127.0.0.1")),
		oakestra.WithBasicLogin("Admin", "Admin"),
	)
	if err != nil {
		log.Fatal(err)
	}

	apps, _, err := client.Applications.List(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, app := range apps {
		fmt.Println(app.ApplicationName)
	}
}
