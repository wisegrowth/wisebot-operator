package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
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

func getJSON(url string, data interface{}) error {
	res, err := http.Get(url)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	if res.StatusCode != 200 {
		return errors.New("invalid status code")
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	// fmt.Printf("%s", body)

	err = json.Unmarshal(body, &data)
	if err != nil {
		return err
	}

	return nil
}
