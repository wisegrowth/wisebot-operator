package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var (
	region   string
	filename string
	filepath string
	bucket   string
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Version is require", os.Args[0])
		return
	}
	version := os.Args[1]

	// create new session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)

	// define filenames
	binaryFile := filename + "-" + version
	checksumFile := filename + "-" + version + ".checksum"

	// create an uploader with the session
	fmt.Println("[aws] connecting...")
	uploader := s3manager.NewUploader(sess)

	//open binary file
	bin, err := os.Open(filepath + "/" + binaryFile)
	if err != nil {
		fmt.Println("[error] failed to open file ", binaryFile, err)
		return
	}

	//open checksum file
	checksum, err := os.Open(filepath + "/" + checksumFile)
	if err != nil {
		fmt.Println("[error] failed to open file ", checksumFile, err)
		return
	}

	// TODO: check if binary file exist inside bucket
	fmt.Println("[aws] uploading bin...")
	binResult, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(binaryFile),
		Body:   bin,
	})
	if err != nil {
		fmt.Println("[error] failed to upload file ", err)
		return
	}

	// TODO: check if checksum file exist inside bucket
	// TODO: make public uploaded files
	fmt.Println("[aws] uploading checksum...")
	checksumResult, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(checksumFile),
		Body:   checksum,
	})
	if err != nil {
		fmt.Println("[error] failed to upload file ", err)
		return
	}

	fmt.Println("[ready] binary file", binResult.Location)
	fmt.Println("[ready] checksum file", checksumResult.Location)
}
