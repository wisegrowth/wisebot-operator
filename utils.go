package main

import (
	"os"

	homedir "github.com/mitchellh/go-homedir"
)

func newFile(name string) (*os.File, error) {
	expanded, err := homedir.Expand(name)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(expanded, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0744)
	if err != nil {
		if os.IsNotExist(err) {
			file, err = os.Create(name)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return file, err
}
