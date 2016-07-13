package main

import (
	"flag"
	"fmt"
	"os"

	"ktkr.us/pkg/manga/core"
	"ktkr.us/pkg/manga/util"
)

var cmdResize = &Command{
	Name:    "resize",
	Summary: "[-n] [-h <height>] [ -x <images...> | <identifier> ]",
	Help: `
Resize and crop images to fit the smallest width and height among them for
consistency. Pages are cropped from the gutter side (right for odd, left for
even). Skips files with dashes in them (spreads).

The resizing via ImageMagick takes into account the issues[1] caused by
improper value vs. luminance interpretation when resizing.

[1] http://www.4p8.com/eric.brasseur/gamma.html`,
	Flags: flag.NewFlagSet("resize", flag.ExitOnError),
}

var (
	//resizeX      = cmdResize.Flags.Bool("x", false, "Resize named images instead of from manga")
	resizeH      = cmdResize.Flags.Int("h", 1600, "Limit all heights to `\033[4mHEIGHT\033[m` pixels")
	resizeW      = cmdResize.Flags.Int("w", -1, "Limit all widths to `\033[4mWIDTH\033[m` pixels (<0 = auto from height)")
	resizeC      = cmdResize.Flags.Int("c", 64, "Limit output to having `\033[4mN\033[m` colors")
	resizeN      = cmdResize.Flags.Bool("n", false, "Don't do colorspace correction")
	resizeFilter = cmdResize.Flags.String("filter", "Mitchell", "Resampling filter")
	resizeO      = cmdResize.Flags.Bool("O", false, "Don't optimize images")
)

func init() {
	cmdResize.Run = runResize
}

func runResize(cmd *Command, args []string) {
	if *globalX {
		ims := make([]*Image, len(args))
		for i, arg := range args {
			ims[i] = namedImage(arg)
		}
		resize(ims)
		return
	}

	core.LoadConfig()

	if len(args) == 0 {
		help(cmd)
	}

	cmd.in(args[0], "res")
	resize(images(Page, Spread))
}

func resize(ims []*Image) {
	// arbitrary, seems like that's when a delay would be noticeable
	if len(ims) > 20 {
		fmt.Println("Analyzing images...")
	}
	imageSizes(ims)

	scaled := make([]Rect, len(ims))
	targetWidth := 0
	n := 0

	// find minimum width
	for i, im := range ims {
		hRatio := float64(im.H) / float64(*resizeH)
		wScale := int(float64(im.W) / hRatio)
		scaled[i] = Rect{wScale, *resizeH}

		if im.H > *resizeH {
			n++
		}

		if im.Kind != Spread && (wScale < targetWidth || targetWidth == 0) {
			targetWidth = wScale
		}
	}

	if *resizeW > 0 {
		targetWidth = *resizeW
	}

	fmt.Printf("Resizing %d image%s to %d×%dpx.\n", n, util.Plural(n), targetWidth, *resizeH)

	if len(ims) > 1 {
		maxLoss := 0
		ii := 0
		for i, im := range scaled {
			if ims[i].Kind != Spread {
				loss := im.W - targetWidth
				if loss > maxLoss {
					maxLoss = loss
					ii = i
				}
			}
		}

		if maxLoss > 0 {
			if !util.Promptf("Max pixel loss from the sides will be %dpx (from %v). Continue?", maxLoss, ims[ii].base()) {
				cmdResize.Fatal("abort")
			}
		}
	}

	imgdo("Resizing", ims, func(im *Image) {
		if im.H <= *resizeH {
			return
		}

		args := make([]string, 0, 10)

		if !*resizeN {
			args = append(args, "-colorspace", "RGB")
		}
		if im.Kind == Page && targetWidth > 0 {
			n := im.ord()
			gravity := ""

			if n%2 == 0 {
				gravity = "east"
			} else {
				gravity = "west"
			}

			extent := fmt.Sprintf("%dx%d", targetWidth, *resizeH)
			args = append(args,
				"-gravity", gravity,
				"-filter", *resizeFilter,
				"-resize", extent+"^",
				"-extent", extent,
			)

		} else {
			args = append(args,
				"-filter", *resizeFilter,
				"-resize", fmt.Sprintf("x%d", *resizeH),
			)
		}

		if im.ext() == ".png" {
			dither := fmt.Sprintf("o8x8,%d", *resizeC)
			args = append(args, "-ordered-dither", dither, "-density", "72")
			//args = append(args, "-colors", "64", "-dither")
			if !*resizeN {
				args = append(args,
					"-colorspace", "sRGB",
					"-colorspace", "Gray",
				)
			}
		} else {
			if !*resizeN {
				args = append(args, "-colorspace", "sRGB")
			}
		}

		args = append(args, im.Path)

		im.convert(args...)
		if !*resizeO {
			im.optimize()
		}
	})

	fmt.Fprintln(os.Stderr)
}
