package media_shrinker

import "os"
import "io"
import "image"
import "image/png"
import "image/jpeg"
import "github.com/disintegration/imageorient"
import "github.com/nfnt/resize"
import "fmt"

func ResizeImage(img image.Image) image.Image {
	size := img.Bounds().Size()
	desiredWidth := 2048
	if size.Y > size.X { // vertical image
		desiredWidth = 1080
	}
	if size.X < desiredWidth {
		desiredWidth = size.X // no scale up
	}

	return resize.Resize(uint(desiredWidth), 0, img, resize.Lanczos3)
}

type EncoderFn func(out io.Writer, img image.Image) error

func encodePng(out io.Writer, img image.Image) error {
	encoder := png.Encoder {
		CompressionLevel: png.BestCompression,
	}
	return encoder.Encode(out, img)
}

func encodeJpeg(out io.Writer, img image.Image) error {
	options := jpeg.Options {
		Quality: 90,
	}
	return jpeg.Encode(out, img, &options)
}

func ShrinkImage(request ProcessingRequest, encoder EncoderFn, ui UI) error {
	file, err := os.Open(request.InputPath)
	if err != nil {
		return fmt.Errorf("Could not open file %s: %w", request.InputPath, err)
	}
	defer file.Close()

	img, _, err := imageorient.Decode(file)
	if err != nil {
		return fmt.Errorf("Could not decode file %s: %w", request.InputPath, err)
	}

	imgResized := ResizeImage(img)

	out, err := os.Create(request.OutputPath)
	if err != nil {
		return fmt.Errorf("Could not create output file %s: %w", request.InputPath, err)
	}
	defer out.Close()

	return encoder(out, imgResized)
}

func ShrinkPNG(request ProcessingRequest, ui UI) error {
	return ShrinkImage(request, encodePng, ui)
}

func ShrinkJPG(request ProcessingRequest, ui UI) error {
	return ShrinkImage(request, encodeJpeg, ui)
}
