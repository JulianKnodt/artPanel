package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"
)

var (
	sleep      = flag.Duration("sleep", 3*time.Second, "How long to sleep for")
	p          = flag.String("p", ".", "Path to folder containing images to render")
	queueSize  = flag.Int("qs", 2, "# of imgs to queue(larger # == less latency, more mem)")
	chars      = flag.String("chars", "░▒▓█", "Characters to use for rendering")
	numWorkers = flag.Int("workers", 2, "Number of workers for rendering")
	fixWidth   = flag.Int("width", -1, "Fixed width(negative implies unfixed)")
	fixHeight  = flag.Int("height", -1, "Fixed height(negative implies unfixed)")
	shuffle    = flag.Bool("shuffle", true, "Shuffle source images")
)

func main() {
	flag.Parse()
	dir := *p
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	if *shuffle {
		rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(files), func(i, j int) {
			files[i], files[j] = files[j], files[i]
		})
	}

	imgs := make(chan string, *queueSize)
	fileSend := make(chan os.FileInfo, len(files))
	var once sync.Once

	for i := 0; i < *numWorkers; i++ {
		go func() {
			for file := range fileSend {
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
			once.Do(func() {
				close(imgs)
			})
		}()
	}
	for _, info := range files {
		fileSend <- info
	}
	close(fileSend)

	for img := range imgs {
		go func(i string) {
			// this is the most expensive part so have to do something special
			// somehow this reduces the cost of printing, so...
			println(i)
		}(img)
		time.Sleep(*sleep)
	}
	fmt.Println("That's all folks!")
}

func dimens() (c int, r int) {
	if *fixWidth > 0 && *fixHeight > 0 {
		return *fixWidth, *fixHeight
	}
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	res, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fscanf(bytes.NewReader(res), "%d %d", &r, &c)
	if *fixWidth > 0 && c > *fixWidth {
		ratio := float64(*fixWidth) / float64(c)
		c, r = *fixWidth, int(float64(r)*ratio)
	} else if *fixHeight > 0 && r > *fixHeight {
		ratio := float64(r) / float64(*fixHeight)
		c, r = int(float64(c)*ratio), *fixHeight
	}
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

func (c ColorString) str(w io.Writer) {
	fmt.Fprintf(w, "\x1b[38;2;%d;%d;%dm%s\x1b[0m", c.r, c.g, c.b, c.contents)
	return
}

func (c ColorString) equalColors(o ColorString) bool {
	return c.r == o.r && c.g == o.g && c.b == o.b
}

func colorsToString(w io.Writer, colors []ColorString) {
	for _, v := range colors {
		v.str(w)
	}
}

func makeImg(img image.Image) string {
	var res strings.Builder
	cols, rows := dimens()
	bounds := img.Bounds()
	width, height := bounds.Dx()-2, bounds.Dy()-2
	xChunkSize := float64(width) / float64(cols)
	yChunkSize := float64(height) / float64(rows)
	blocks := []rune(*chars)

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
		colorsToString(&res, line)
	}

	return res.String()
}
