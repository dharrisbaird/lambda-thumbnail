package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"regexp"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/disintegration/imaging"
)

var (
	sess        = session.Must(session.NewSession())
	uploader    = s3manager.NewUploader(sess)
	downloader  = s3manager.NewDownloader(sess)
	pathRegex   = regexp.MustCompile(`(designs|products)/(\d+)/`)
	errorLogger = log.New(os.Stderr, "ERROR ", log.Llongfile)
)

type Transform struct {
	Size        int
	Path        string
	Format      imaging.Format
	ContentType string
}

type Input struct {
	URL   string `json:"url"`
	Model string `json:"model"`
	ID    string `json:"id"`
}

var ImageTransforms = map[string]Transform{
	"small": {Format: imaging.JPEG, ContentType: "image/jpeg", Size: 300, Path: "%s/%s/300.jpg"},
	"large": {Format: imaging.JPEG, ContentType: "image/jpeg", Size: 1200, Path: "%s/%s/1200.jpg"},
}

var (
	cropSize = 30
	anchors  = []imaging.Anchor{
		imaging.TopLeft,
		imaging.TopRight,
		imaging.BottomLeft,
		imaging.BottomRight,
	}
)

func handle(ctx context.Context, req events.S3Event) (string, error) {
	for _, r := range req.Records {
		bucket := r.S3.Bucket.Name
		processFile(bucket, r.S3.Object.Key)
	}
	return fmt.Sprintf("%d records processed", len(req.Records)), nil
}

func processFile(bucket, key string) {
	log.Printf("Processing %s in bucket %s\n", bucket, key)

	matches := pathRegex.FindStringSubmatch(key)
	if len(matches) == 0 {
		log.Fatal()
	}
	model := matches[1]
	id := matches[2]

	var f *aws.WriteAtBuffer

	_, err := downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		log.Fatal(err)
	}

	img, err := imaging.Decode(bytes.NewReader(f.Bytes()))
	if err != nil {
		log.Fatal(err)
	}

	bgColor := findBackgroundColor(img)

	for _, transform := range ImageTransforms {
		thumb := transformImage(img, transform, bgColor)
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		err := imaging.Encode(w, thumb, imaging.JPEG, imaging.JPEGQuality(80))
		if err != nil {
			log.Fatal(err)
		}

		r := bufio.NewReader(&b)

		_, err = uploader.Upload(&s3manager.UploadInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(fmt.Sprintf(transform.Path, model, id)),
			Body:        r,
			ContentType: aws.String(transform.ContentType),
		})
		if err != nil {
			log.Fatal(err)
		}
	}
}

func transformImage(img image.Image, transform Transform, bgColor color.Color) image.Image {
	if transform.Size == 0 {
		return img
	}

	resizeWidth, resizeHeight := transform.Size, transform.Size
	bounds := img.Bounds()

	// Setting a dimension to zero resizes while maintaining aspect ratio.
	if bounds.Max.X > bounds.Max.Y {
		resizeHeight = 0
	} else {
		resizeWidth = 0
	}

	dstImage := imaging.Resize(img, resizeWidth, resizeHeight, imaging.CatmullRom)
	dstImage = imaging.Sharpen(dstImage, .5)

	// Create a new solid color image
	bgImage := imaging.New(transform.Size, transform.Size, bgColor)

	return imaging.OverlayCenter(bgImage, dstImage, 1)
}

func findBackgroundColor(img image.Image) color.Color {
	histogram := map[color.Color]int{}
	var bestColor color.Color
	bestCount := 0

	for _, anchor := range anchors {
		cropped := imaging.CropAnchor(img, cropSize, cropSize, anchor)
		for pos := 0; pos < cropSize; pos++ {
			currColor := cropped.At(pos, pos)

			histogram[currColor]++
			currCount := histogram[currColor]
			if currCount > bestCount {
				bestCount = currCount
				bestColor = currColor
			}
		}
	}

	return bestColor
}

func main() {
	lambda.Start(handle)
}
