package main

import (
	"fmt"

	"github.com/WiseGrowth/operator/rasp"
)

func main() {
	fmt.Println("Starting...")
	if err := rasp.TurnOffPins(); err != nil {
		fmt.Println(err)
		panic(err)
	}
	fmt.Println("Done...")
}
