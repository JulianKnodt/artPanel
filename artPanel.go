package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

func main() {
	dir := os.Args[1]
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	imgs := make(chan image.Image, 1000000)
	for _, file := range files {
		f, err := os.Open(path.Join(dir, file.Name()))
		if err != nil {
			continue
		}
		img, _, err := image.Decode(f)
		if err != nil {
			continue
		}
		f.Close()
		imgs <- img
	}
	close(imgs)

	for curr := range imgs {
		clear()
		drawImg(curr)
		time.Sleep(time.Duration(3) * time.Second)
		clear()
	}
}

func dimens() (int, int) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	res, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	out := strings.Split(string(res), " ")
	rows, cols := out[0], out[1]
	if err != nil {
		log.Fatal(err)
	}
	c, err := strconv.Atoi(strings.Trim(string(cols), "\n"))
	r, err := strconv.Atoi(strings.Trim(string(rows), "\n"))
	return c, r
}

func clear() {
	cmd := exec.Command("clear")
	cmd.Run()
}

func luminance(r, g, b float64) float64 {
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func avg(colors chan color.Color, size int) (float64, float64, float64) {
	var sumR, sumB, sumG float64
	sizeF := float64(size)
	count := 0
	for c := range colors {
		r, g, b, _ := c.RGBA()
		sumR += float64(r>>8) / sizeF
		sumG += float64(g>>8) / sizeF
		sumB += float64(b>>8) / sizeF
		count++
		if count == size {
			break
		}
	}
	return sumR, sumB, sumG
}

func printColored(r, g, b float64, s string) {
	fmt.Printf("%s", s)
}

func drawImg(img image.Image) {
	cols, rows := dimens()
	bounds := img.Bounds()
	width, height := bounds.Dx()-2, bounds.Dy()-2
	xChunkSize := float64(width) / float64(cols)
	yChunkSize := float64(height) / float64(rows)
	blocks := []rune(" ░▒▓▓██")

	for y := 0.0; y < float64(height); y += yChunkSize {
		for x := 0.0; x < float64(width)-10; x += xChunkSize {
			colors := make(chan color.Color, 1+int(xChunkSize*yChunkSize))
			for i := 0; i < int(xChunkSize); i++ {
				for j := 0; j < int(yChunkSize); j++ {
					colors <- img.At(int(x)+int(i), int(y)+int(i))
				}
			}
			close(colors)
			r, g, b := avg(colors, 1+int(xChunkSize*yChunkSize))
			lum := luminance(r, g, b)
			printColored(r, g, b, string(blocks[int(lum/255*float64(len(blocks)-1))]))
		}
		fmt.Println()
	}
}
