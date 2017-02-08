package main

import "log"

func main() {
	log.Println("Hello world")
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
