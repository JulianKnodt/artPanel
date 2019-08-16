package main

import (
	"bufio"
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
	"math"
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
	bufferSize = flag.Int("buf", 4096*2, "Size of buffer for output")
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
	var wg sync.WaitGroup
	var once sync.Once

	wg.Add(*numWorkers)
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
			wg.Done()
			wg.Wait()
			once.Do(func() {
				close(imgs)
			})
		}()
	}
	for _, info := range files {
		fileSend <- info
	}
	close(fileSend)

	bufOut := bufio.NewWriterSize(os.Stdout, *bufferSize)
	for img := range imgs {
		println("\033[H\033[2J")
		// this is the most expensive part so have to do something special
		// somehow this reduces the cost of printing, so...
		bufOut.WriteString(img)
		bufOut.WriteRune('\n')
		bufOut.Flush()
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

type Avg struct {
	r, g, b *big.Float
	count   int
}

func NewAvg() *Avg {
	return &Avg{big.NewFloat(0), big.NewFloat(0), big.NewFloat(0), 0}
}

func (a *Avg) Add(c color.Color) {
	r, g, b, _ := c.RGBA()
	a.r.Add(a.r, big.NewFloat(float64(r>>8)))
	a.g.Add(a.g, big.NewFloat(float64(g>>8)))
	a.b.Add(a.b, big.NewFloat(float64(b>>8)))
	a.count++
}

func (a *Avg) Out() (float64, float64, float64) {
	size := big.NewFloat(float64(a.count))
	r, _ := a.r.Quo(a.r, size).Float64()
	g, _ := a.g.Quo(a.g, size).Float64()
	b, _ := a.b.Quo(a.b, size).Float64()
	return r, g, b
}

type ColorString struct {
	r, g, b  int
	contents []rune
}

func (c *ColorString) str(w io.Writer) {
	fmt.Fprintf(w, "\x1b[38;2;%d;%d;%dm%s\x1b[0m", c.r, c.g, c.b, string(c.contents))
	return
}

func (c *ColorString) equal(r, g, b int) bool {
	return c.r == r && c.g == g && c.b == b
}

func makeImg(img image.Image) string {
	var res strings.Builder
	cols, rows := dimens()
	bounds := img.Bounds()
	width, height := bounds.Dx()-2, bounds.Dy()-2
	xStep := math.Max(float64(width)/float64(cols), 1)
	yStep := math.Max(float64(height)/float64(rows), 1)
	res.Grow(int((float64(height*width) / (yStep * xStep))))
	blocks := []rune(*chars)

	for y := 0.0; y < float64(height)-5; y += yStep {
		res.WriteRune('\n')
		var prev *ColorString
		for x := 0.0; x < float64(width)-5; x += xStep {
			avg := NewAvg()
			for i := 0; i < int(xStep); i++ {
				for j := 0; j < int(yStep); j++ {
					avg.Add(img.At(int(x)+int(i), int(y)+int(i)))
				}
			}
			r, g, b := avg.Out()
			lum := luminance(r, g, b)
			rI, gI, bI := int(r), int(g), int(b)
			content := blocks[int(lum/255*float64(len(blocks)-1))]
			if prev == nil {
				prev = &ColorString{rI, gI, bI, []rune{content}}
			} else if prev.equal(rI, gI, bI) {
				prev.contents = append(prev.contents, content)
			} else {
				prev.str(&res)
				prev = &ColorString{rI, gI, bI, []rune{content}}
			}
		}
		if prev != nil {
			prev.str(&res)
		}
	}

	return res.String()
}
