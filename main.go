// +build ignore

package main

import (
	"log"

	"."
)

func main() {
	highmc.ServerName = "HighMC in-dev server"
	router, err := highmc.CreateRouter(19132)
	if err != nil {
		log.Fatalln(err)
	}
	router.Start()
	<-(chan struct{})(nil)
}
