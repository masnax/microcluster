package main

import (
	"fmt"

	"github.com/canonical/microcluster/internal/db/update"
)

func main() {
	err := update.SchemaDotGo()
	fmt.Println(err)
}
