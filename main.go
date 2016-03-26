// +build ignore

package main

import (
	"."
	"log"
	"os"
	"runtime"
)

func main() {
	highmc.ServerName = "HighMC in-dev server"
	router, err := highmc.CreateRouter(19132)
	if err != nil {
		log.Fatalln(err)
	}
	router.Start()
	log.Println("Server running on :19132")
	for {
		fmt.Scanln()
		var b [1024 * 1024 * 16]byte
		n := runtime.Stack(b[:], true)
		os.Stdout.Write(b[:n])
	}
}
