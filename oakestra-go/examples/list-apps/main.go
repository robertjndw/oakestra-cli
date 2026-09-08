// Command list-apps connects to an Oakestra System Manager and prints its
// applications. It is a minimal, runnable example of using the oakestra-go
// client library.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/oakestra/oakestra-cli/oakestra-go"
)

func main() {
	host := flag.String("host", "127.0.0.1", "Oakestra System Manager host")
	username := flag.String("username", "Admin", "login username")
	password := flag.String("password", "Admin", "login password")
	flag.Parse()

	client, err := oakestra.NewClient(
		oakestra.WithBaseURL(oakestra.BaseURLForHost(*host)),
		oakestra.WithBasicLogin(*username, *password),
	)
	if err != nil {
		log.Fatalf("creating client: %v", err)
	}

	apps, _, err := client.Applications.List(context.Background())
	if err != nil {
		log.Fatalf("listing applications: %v", err)
	}

	if len(apps) == 0 {
		fmt.Println("no applications found")
		return
	}
	for _, app := range apps {
		fmt.Printf("%s\t%s\t%s\n", app.ApplicationID, app.ApplicationName, app.ApplicationNamespace)
	}
}
