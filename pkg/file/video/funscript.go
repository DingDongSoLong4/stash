package video

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Action is a move at a specific time.
type FunscriptAction struct {
	// At time in milliseconds the action should fire.
	At int64 `json:"at"`
	// Pos is the place in percent to move to.
	Pos int `json:"pos"`

	// Calculated by CalculateIntensityAndSpeed
	Slope     float64
	Intensity int64
	Speed     float64
}

type Funscript struct {
	// Version of Launchscript
	Version string `json:"version"`
	// Inverted causes up and down movement to be flipped.
	Inverted bool `json:"inverted,omitempty"`
	// Range is the percentage of a full stroke to use.
	Range int `json:"range,omitempty"`
	// Actions are the timed moves.
	Actions []FunscriptAction `json:"actions"`
}

// GetFunscriptPath returns the path of a file
// with the extension changed to .funscript
func GetFunscriptPath(path string) string {
	ext := filepath.Ext(path)
	fn := strings.TrimSuffix(path, ext)
	return fn + ".funscript"
}

func ParseFunscriptFile(path string) (Funscript, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Funscript{}, err
	}
	return ParseFunscript(data)
}

func ParseFunscript(data []byte) (Funscript, error) {
	var ret Funscript
	err := json.Unmarshal(data, &ret)
	if err != nil {
		return Funscript{}, err
	}

	sort.SliceStable(ret.Actions, func(i, j int) bool { return ret.Actions[i].At < ret.Actions[j].At })

	return ret, nil
}

func (s *Funscript) CalculateIntensityAndSpeed() {
	var t1, t2 int64
	var p1, p2 int
	var slope float64
	var intensity int64
	for i := range s.Actions {
		if i == 0 {
			continue
		}
		t1 = s.Actions[i].At
		t2 = s.Actions[i-1].At
		p1 = s.Actions[i].Pos
		p2 = s.Actions[i-1].Pos

		slope = math.Min(math.Max(1/(2*float64(t1-t2)/1000), 0), 20)
		intensity = int64(slope * math.Abs((float64)(p1-p2)))
		speed := math.Abs(float64(p1-p2)) / float64(t1-t2) * 1000

		s.Actions[i].Slope = slope
		s.Actions[i].Intensity = intensity
		s.Actions[i].Speed = speed
	}
}

func (s *Funscript) CalculateMedian() int {
	tmp := make([]FunscriptAction, len(s.Actions))
	copy(tmp, s.Actions)

	sort.Slice(tmp, func(i, j int) bool {
		return tmp[i].Speed < tmp[j].Speed
	})

	mNumber := len(tmp) / 2

	if len(tmp)%2 != 0 {
		return int(tmp[mNumber].Speed)
	}

	return int((tmp[mNumber-1].Speed + tmp[mNumber].Speed) / 2)
}

func convertRange(value int, fromLow int, fromHigh int, toLow int, toHigh int) int {
	return ((value-fromLow)*(toHigh-toLow))/(fromHigh-fromLow) + toLow
}

func (s Funscript) ConvertToCSV() []byte {
	var ret []byte
	for _, action := range s.Actions {
		pos := action.Pos

		if s.Inverted {
			pos = convertRange(pos, 0, 100, 100, 0)
		}

		if s.Range > 0 {
			pos = convertRange(pos, 0, s.Range, 0, 100)
		}

		ret = append(ret, fmt.Sprintf("%d,%d\r\n", action.At, pos)...)
	}
	return ret
}
