package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"log"
	"math/big"
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

	imgs := make(chan string, 2)
	go func(files []os.FileInfo) {
		for _, file := range files {
			f, err := os.Open(path.Join(dir, file.Name()))
			if err != nil {
				continue
			}
			img, _, err := image.Decode(f)
			if err != nil {
				f.Close()
				continue
			}
			f.Close()
			imgs <- makeImg(img)
		}
		close(imgs)
	}(files)

	for img := range imgs {
		go func(i string) {
			// this is the most expensive part so have to do something special
			// somehow this reduces the cost of printing, so...
			print(i)
		}(img)
		time.Sleep(time.Duration(3) * time.Second)
	}
	fmt.Println("That's all folks!")
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

func luminance(r, g, b float64) float64 {
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func avg(colors chan color.Color, size int) (float64, float64, float64) {
	sumR, sumG, sumB := big.NewFloat(0), big.NewFloat(0), big.NewFloat(0)
	for c := range colors {
		r, g, b, _ := c.RGBA()
		sumR.Add(sumR, big.NewFloat(float64(r>>8)))
		sumG.Add(sumG, big.NewFloat(float64(g>>8)))
		sumB.Add(sumB, big.NewFloat(float64(b>>8)))
	}
	sizeF := big.NewFloat(float64(size))
	r, _ := sumR.Quo(sumR, sizeF).Float64()
	g, _ := sumG.Quo(sumG, sizeF).Float64()
	b, _ := sumB.Quo(sumB, sizeF).Float64()
	return r, g, b
}

type ColorString struct {
	r, g, b  int
	contents string
}

func (c ColorString) str() string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", c.r, c.g, c.b, c.contents)
}

func (c ColorString) equalColors(o ColorString) bool {
	return c.r == o.r && c.g == o.g && c.b == o.b
}

func colorsToString(colors []ColorString) string {
	var res strings.Builder
	for _, v := range colors {
		res.WriteString(v.str())
	}
	return res.String()
}

func makeImg(img image.Image) string {
	var res strings.Builder
	cols, rows := dimens()
	bounds := img.Bounds()
	width, height := bounds.Dx()-2, bounds.Dy()-2
	xChunkSize := float64(width) / float64(cols)
	yChunkSize := float64(height) / float64(rows)
	blocks := []rune("░▒▓")

	for y := 0.0; y < float64(height)-5; y += yChunkSize {
		res.WriteRune('\n')
		line := make([]ColorString, 0)
		for x := 0.0; x < float64(width)-5; x += xChunkSize {
			colors := make(chan color.Color, int(xChunkSize*yChunkSize))
			for i := 0; i < int(xChunkSize); i++ {
				for j := 0; j < int(yChunkSize); j++ {
					colors <- img.At(int(x)+int(i), int(y)+int(i))
				}
			}
			close(colors)
      if len(colors) == 0 {
        continue
      }
			r, g, b := avg(colors, len(colors))
			lum := luminance(r, g, b)
			contents := string(blocks[int(lum/255*float64(len(blocks)-1))])
			c := ColorString{int(r), int(g), int(b), contents}
			if len(line) == 0 {
				line = append(line, c)
			} else if line[len(line)-1].equalColors(c) {
				line[len(line)-1].contents += contents
			} else {
				line = append(line, c)
			}
		}
		res.WriteString(colorsToString(line))
	}

	return res.String()
}
