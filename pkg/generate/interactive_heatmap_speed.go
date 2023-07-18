package generate

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"sort"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/stashapp/stash/pkg/file/video"
	"github.com/stashapp/stash/pkg/logger"
)

const (
	heatmapNumSegments = 600
	heatmapWidth       = 1280
	heatmapHeight      = 60
)

func GenerateInteractiveHeatmapSpeed(input string, heatmapPath string, duration float64, drawRange bool) (int, error) {
	funscript, err := video.ParseFunscriptFile(input)
	if err != nil {
		return 0, err
	}

	if funscript.Actions == nil {
		return 0, fmt.Errorf("actions list missing in %s", input)
	}

	// trim actions with negative timestamps to avoid index range errors when generating heatmap
	// #3181 - also trim actions that occur after the scene duration
	loggedBadTimestamp := false
	durationMilli := int64(duration * 1000)
	isValid := func(x int64) bool {
		return x >= 0 && x < durationMilli
	}

	i := 0
	for _, x := range funscript.Actions {
		if isValid(x.At) {
			funscript.Actions[i] = x
			i++
		} else if !loggedBadTimestamp {
			loggedBadTimestamp = true
			logger.Warnf("Invalid timestamp %d in %s: subsequent invalid timestamps will not be logged", x.At, input)
		}
	}

	funscript.Actions = funscript.Actions[:i]

	if len(funscript.Actions) == 0 {
		return 0, fmt.Errorf("no valid actions in %s", input)
	}

	funscript.CalculateIntensityAndSpeed()

	err = saveHeatmap(funscript, heatmapPath, durationMilli, drawRange)
	if err != nil {
		return 0, err
	}

	median := funscript.CalculateMedian()

	return median, nil
}

// funscript needs to have intensity updated first
func saveHeatmap(funscript video.Funscript, output string, durationMilli int64, drawRange bool) error {
	gradient := getGradientTable(funscript, durationMilli)

	img := image.NewRGBA(image.Rect(0, 0, heatmapWidth, heatmapHeight))
	for x := 0; x < heatmapWidth; x++ {
		xPos := float64(x) / float64(heatmapWidth)
		c := gradient.GetInterpolatedColorFor(xPos)

		y0 := 0
		y1 := heatmapHeight

		if drawRange {
			yRange := gradient.GetYRange(xPos)
			top := int(yRange[0] / 100.0 * float64(heatmapHeight))
			bottom := int(yRange[1] / 100.0 * float64(heatmapHeight))

			y0 = heatmapHeight - top
			y1 = heatmapHeight - bottom
		}

		draw.Draw(img, image.Rect(x, y0, x+1, y1), &image.Uniform{c}, image.Point{}, draw.Src)
	}

	// add 10 minute marks
	maxts := durationMilli
	const tick = 600000
	var ts int64 = tick
	c, _ := colorful.Hex("#000000")
	for ts < maxts {
		x := int(float64(ts) / float64(maxts) * float64(heatmapWidth))
		draw.Draw(img, image.Rect(x-1, heatmapHeight/2, x+1, heatmapHeight), &image.Uniform{c}, image.Point{}, draw.Src)
		ts += tick
	}

	outpng, err := os.Create(output)
	if err != nil {
		return err
	}
	defer outpng.Close()

	err = png.Encode(outpng, img)
	return err
}

type GradientTable []struct {
	Col    colorful.Color
	Pos    float64
	YRange [2]float64
}

func (gt GradientTable) GetInterpolatedColorFor(t float64) colorful.Color {
	for i := 0; i < len(gt)-1; i++ {
		c1 := gt[i]
		c2 := gt[i+1]
		if c1.Pos <= t && t <= c2.Pos {
			// We are in between c1 and c2. Go blend them!
			t := (t - c1.Pos) / (c2.Pos - c1.Pos)
			return c1.Col.BlendHcl(c2.Col, t).Clamped()
		}
	}

	// Nothing found? Means we're at (or past) the last gradient keypoint.
	return gt[len(gt)-1].Col
}

func (gt GradientTable) GetYRange(t float64) [2]float64 {
	for i := 0; i < len(gt)-1; i++ {
		c1 := gt[i]
		c2 := gt[i+1]
		if c1.Pos <= t && t <= c2.Pos {
			// TODO: We are in between c1 and c2. Go blend them!
			return c1.YRange
		}
	}

	// Nothing found? Means we're at (or past) the last gradient keypoint.
	return gt[len(gt)-1].YRange
}

func getGradientTable(funscript video.Funscript, sceneDurationMilli int64) GradientTable {
	const windowSize = 15
	const backfillThreshold = 500

	segments := make([]struct {
		count     int
		intensity int
		yRange    [2]float64
		at        int64
	}, heatmapNumSegments)
	gradient := make(GradientTable, heatmapNumSegments)
	posList := []int{}

	maxts := sceneDurationMilli

	for _, a := range funscript.Actions {
		posList = append(posList, a.Pos)

		if len(posList) > windowSize {
			posList = posList[1:]
		}

		sortedPos := make([]int, len(posList))
		copy(sortedPos, posList)
		sort.Ints(sortedPos)

		topHalf := sortedPos[len(sortedPos)/2:]
		bottomHalf := sortedPos[0 : len(sortedPos)/2]

		var totalBottom int
		var totalTop int

		for _, value := range bottomHalf {
			totalBottom += value
		}
		for _, value := range topHalf {
			totalTop += value
		}

		averageBottom := float64(totalBottom) / float64(len(bottomHalf))
		averageTop := float64(totalTop) / float64(len(topHalf))

		segment := int(float64(a.At) / float64(maxts+1) * float64(heatmapNumSegments))
		// #3181 - sanity check. Clamp segment to heatmapNumSegments-1
		if segment >= heatmapNumSegments {
			segment = heatmapNumSegments - 1
		}
		segments[segment].at = a.At
		segments[segment].count++
		segments[segment].intensity += int(a.Intensity)
		segments[segment].yRange[0] = averageTop
		segments[segment].yRange[1] = averageBottom
	}

	lastSegment := segments[0]

	// Fill in gaps in segments
	for i := 0; i < heatmapNumSegments; i++ {
		segmentTS := int64(float64(i) / float64(heatmapNumSegments))

		// Empty segment - fill it with the previous up to backfillThreshold ms
		if segments[i].count == 0 {
			if segmentTS-lastSegment.at < backfillThreshold {
				segments[i].count = lastSegment.count
				segments[i].intensity = lastSegment.intensity
				segments[i].yRange[0] = lastSegment.yRange[0]
				segments[i].yRange[1] = lastSegment.yRange[1]
			}
		} else {
			lastSegment = segments[i]
		}
	}

	for i := 0; i < heatmapNumSegments; i++ {
		gradient[i].Pos = float64(i) / float64(heatmapNumSegments-1)
		gradient[i].YRange = segments[i].yRange
		if segments[i].count > 0 {
			gradient[i].Col = getSegmentColor(float64(segments[i].intensity) / float64(segments[i].count))
		} else {
			gradient[i].Col = getSegmentColor(0.0)
		}
	}

	return gradient
}

func getSegmentColor(intensity float64) colorful.Color {
	colorBlue, _ := colorful.Hex("#1e90ff")   // DodgerBlue
	colorGreen, _ := colorful.Hex("#228b22")  // ForestGreen
	colorYellow, _ := colorful.Hex("#ffd700") // Gold
	colorRed, _ := colorful.Hex("#dc143c")    // Crimson
	colorPurple, _ := colorful.Hex("#800080") // Purple
	colorBlack, _ := colorful.Hex("#0f001e")
	colorBackground, _ := colorful.Hex("#30404d") // Same as GridCard bg

	var stepSize = 60.0
	var f float64
	var c colorful.Color

	switch {
	case intensity <= 0.001:
		c = colorBackground
	case intensity <= 1*stepSize:
		f = (intensity - 0*stepSize) / stepSize
		c = colorBlue.BlendLab(colorGreen, f)
	case intensity <= 2*stepSize:
		f = (intensity - 1*stepSize) / stepSize
		c = colorGreen.BlendLab(colorYellow, f)
	case intensity <= 3*stepSize:
		f = (intensity - 2*stepSize) / stepSize
		c = colorYellow.BlendLab(colorRed, f)
	case intensity <= 4*stepSize:
		f = (intensity - 3*stepSize) / stepSize
		c = colorRed.BlendRgb(colorPurple, f)
	default:
		f = (intensity - 4*stepSize) / (5 * stepSize)
		f = math.Min(f, 1.0)
		c = colorPurple.BlendLab(colorBlack, f)
	}

	return c
}
