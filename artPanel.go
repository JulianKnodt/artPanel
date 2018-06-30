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

	imgs := make(chan string, 5)
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
			imgs<-makeImg(img)
		}
    close(imgs)
	}(files)

  for img := range imgs {
    fmt.Print(img)
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

func clear() {
	cmd := exec.Command("clear")
	cmd.Run()
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

func colored(r, g, b float64, s string) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", int(r), int(g), int(b), s)
}

func makeImg(img image.Image) string {
  var res strings.Builder
	cols, rows := dimens()
	bounds := img.Bounds()
	width, height := bounds.Dx()-2, bounds.Dy()-2
	xChunkSize := float64(width) / float64(cols)
	yChunkSize := float64(height) / float64(rows)
	blocks := []rune("░▒▓█")

	for y := 0.0; y < float64(height)-5; y += yChunkSize {
    res.WriteRune('\n')
		for x := 0.0; x < float64(width)-5; x += xChunkSize {
			colors := make(chan color.Color, int(xChunkSize*yChunkSize))
			for i := 0; i < int(xChunkSize); i++ {
				for j := 0; j < int(yChunkSize); j++ {
					colors <- img.At(int(x)+int(i), int(y)+int(i))
				}
			}
			close(colors)
			r, g, b := avg(colors, len(colors))
			lum := luminance(r, g, b)
			s := colored(r, g, b, string(blocks[int(lum/255*float64(len(blocks)-1))]))
      res.WriteString(s)
		}
	}

  return res.String()
}
