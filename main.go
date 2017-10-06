package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	wave "github.com/dlclark/go-wav"
)

var lPattern = ".L.wav"
var rPattern = ".R.wav"
var stReplace = ".wav"

var doDel = flag.Bool("del", false, "true to delete L/R files once combined")

func main() {
	flag.Parse()
	// find all wav files in the current dir that have
	// identical names except .L. and .R. portions

	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	// build our list of files to join
	outs := map[string]lr{}

	for _, f := range files {
		// skip dirs and non-wav files
		name := f.Name()
		if f.IsDir() || filepath.Ext(name) != ".wav" {
			continue
		}

		var stereoFile string
		var lrfiles lr
		if strings.Contains(name, lPattern) {
			lrfiles.L = name
			stereoFile = strings.Replace(name, lPattern, stReplace, 1)
		} else if strings.Contains(name, rPattern) {
			lrfiles.R = name
			stereoFile = strings.Replace(name, rPattern, stReplace, 1)
		}

		// no stereo file pattern match, move on
		if stereoFile == "" {
			continue
		}

		if outlr, ok := outs[stereoFile]; ok {
			outs[stereoFile] = outlr.combine(lrfiles)
		} else {
			outs[stereoFile] = lrfiles
		}
	}

	for out, lrs := range outs {
		fmt.Printf("\tOpening %v...\n", lrs.L)
		lfile, err := os.Open(lrs.L)
		if err != nil {
			panic(err)
		}
		lread := wave.NewReader(lfile)

		fmt.Printf("\tOpening %v...\n", lrs.R)
		rfile, err := os.Open(lrs.R)
		if err != nil {
			panic(err)
		}
		rread := wave.NewReader(rfile)

		// confirm L/R channels match stats
		lfmt := validateEqual(lread, rread)

		outf, err := os.Create(fmt.Sprintf("./%s", out))
		if err != nil {
			panic(err)
		}
		buf := bufio.NewWriter(outf)

		w := wave.NewWriter(buf, lread.Size/uint32(lfmt.BlockAlign), 2, lfmt.SampleRate, lfmt.BitsPerSample)

		lsamps, err := lread.ReadSamples(lread.Size / uint32(lfmt.BlockAlign))
		if err != nil {
			panic(err)
		}

		rsamps, err := rread.ReadSamples(rread.Size / uint32(lfmt.BlockAlign))
		if err != nil {
			panic(err)
		}

		lfile.Close()
		rfile.Close()

		// combine into new samples and write them
		for i := 0; i < len(lsamps); i++ {
			// copy our rchan mono value into the right chan of lsamp
			lsamps[i].Values[1] = rsamps[i].Values[0]
		}

		fmt.Printf("Writing %v...", out)
		if err := w.WriteSamples(lsamps); err != nil {
			panic(err)
		}

		// done with our files
		buf.Flush()
		outf.Close()

		if *doDel {
			fmt.Printf("Cleaning...")
			os.Remove(lrs.L)
			os.Remove(lrs.R)
		}

		fmt.Printf("Done.\n")

	}
	//wave.NewReader()
}

func validateEqual(l *wave.Reader, r *wave.Reader) *wave.WavFormat {
	lfmt, err := l.Format()
	if err != nil {
		panic(err)
	}

	rfmt, err := r.Format()
	if err != nil {
		panic(err)
	}

	ldata, err := l.Data()
	if err != nil {
		panic(err)
	}

	rdata, err := r.Data()
	if err != nil {
		panic(err)
	}

	if lfmt.NumChannels != 1 {
		panic("L file not mono")
	}
	if rfmt.NumChannels != 1 {
		panic("R file not mono")
	}
	if lval, rval := lfmt.SampleRate, rfmt.SampleRate; lval != rval {
		panic(fmt.Errorf("Sample rates not equal: L %v R %v", lval, rval))
	}
	if lval, rval := lfmt.BitsPerSample, rfmt.BitsPerSample; lval != rval {
		panic(fmt.Errorf("Bit rates not equal: L %v R %v", lval, rval))
	}
	if lval, rval := lfmt.BlockAlign, rfmt.BlockAlign; lval != rval {
		panic(fmt.Errorf("Block size not equal: L %v R %v", lval, rval))
	}
	if lval, rval := lfmt.AudioFormat, rfmt.AudioFormat; lval != rval {
		panic(fmt.Errorf("Audio format not equal: L %v R %v", lval, rval))
	}
	if lval, rval := ldata.Size, rdata.Size; lval != rval {
		panic(fmt.Errorf("Sample counts not equal: L %v R %v", lval, rval))
	}

	return lfmt
}

type lr struct {
	L string
	R string
}

func (a lr) combine(b lr) lr {
	// if a and b have the same L/R set then we panic
	if a.L != "" && b.L != "" {
		panic(fmt.Errorf("mis-matched L files: '%v' and '%v'", a.L, b.L))
	}
	if a.R != "" && b.R != "" {
		panic(fmt.Errorf("mis-matched R files: '%v' and '%v'", a.R, b.R))
	}

	if a.L == "" {
		a.L = b.L
	} else {
		a.R = b.R
	}

	return a
}
