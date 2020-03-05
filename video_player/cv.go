package video_player

// This file hides the ffmpeg related computations.
//
// The api exposed by github.com/3d0c/gmf is messy (all go-ffmpeg lib are)
// which explain some of the following mess :).
//
// Based on the examples 'video-to-goImage.go' of 3d0c/gmf.
// TODO: fix the memory leak will occur..
import (
	// "errors"
	"fmt"
	// "io"
	// "sync"
	"gocv.io/x/gocv"
	"runtime"
	"sync/atomic"
)

type ffmpegVideo struct {
	// img            gocv.Mat
	// rgba           gocv.Mat
	source   string
	duration float64
	fps      float64
	Frames   chan *ffmpegFrame
	paused   chan bool
	player   *playerStatus
	width    int
	height   int
}

type ffmpegFrame struct {
	// ffmpeg packet
	buffer []byte
	// in second
	time float64
}

func (f *ffmpegFrame) Time() float64 {
	return f.time
}

func (f *ffmpegFrame) Data() []byte {
	return f.buffer
}

type playerStatus struct{ flag int32 }

const (
	stopped = 3
	paused  = 2
	playing = 1
	start   = 0
)

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func PrintMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tHeapAlloc = %v MiB", bToMb(m.HeapAlloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func (f *ffmpegVideo) Init(srcFileName string, bufferSize int) (err error) {
	f.player = new(playerStatus)
	f.paused = make(chan bool)

	f.Frames = make(chan *ffmpegFrame, bufferSize)

	f.source = srcFileName
	// webcam, err := gocv.OpenVideoCapture(srcFileName)
	webcam, err := gocv.OpenVideoCapture(0)
	defer webcam.Close()
	if err != nil {
		fmt.Printf("Error opening video capture device: %v\n", srcFileName)
		return
	}

	img := gocv.NewMat()
	defer img.Close()
	if ok := webcam.Read(&img); !ok {
		fmt.Printf("Cannot read device\n")
		return
	}
	f.width = int(webcam.Get(gocv.VideoCaptureFrameWidth))
	f.height = int(webcam.Get(gocv.VideoCaptureFrameHeight))
	webcam.Set(gocv.VideoCapturePosAVIRatio, 1)
	f.duration = webcam.Get(gocv.VideoCapturePosFrames)
	f.fps = webcam.Get(gocv.VideoCaptureFPS)

	return nil
}

func (f *ffmpegVideo) Free() {
	PrintMemUsage()
}

func (f *ffmpegVideo) Stream(onFirstFrame func()) {
	hasConsumer := false

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("go-flutter/plugins/video_player: recover: ", r)
			f.Cancel()
		}
	}()

	webcam, err := gocv.OpenVideoCapture(0)
	// webcam, err := gocv.OpenVideoCapture(f.source)
	defer webcam.Close()
	if err != nil {
		fmt.Printf("Error opening video capture device: %v\n", f.source)
		return
	}

	img := gocv.NewMat()
	rgba := gocv.NewMat()
	defer img.Close()
	defer rgba.Close()

	for webcam.Get(gocv.VideoCapturePosAVIRatio) < 1 && !f.Stopped() {

		if ok := webcam.Read(&img); !ok {
			break
		}
		if img.Empty() {
			continue
		}

		time := f.fps * webcam.Get(gocv.VideoCapturePosMsec)
		gocv.CvtColor(img, &rgba, gocv.ColorBGRToRGBA)
		buffer := rgba.ToBytes()
		f.Frames <- &ffmpegFrame{buffer, time}
		if !hasConsumer {
			f.play()
			go func() {
				onFirstFrame()
			}()
			hasConsumer = true
		}
	}
}

func (f *ffmpegVideo) Bounds() (int, int) {
	return f.width, f.height
}

func (f *ffmpegVideo) GetFrameRate() float64 {
	return f.fps
}

func (f *ffmpegVideo) Duration() float64 {
	return f.duration
}

func (f *ffmpegVideo) play() {
	f.Set(playing)
}

func (f *ffmpegVideo) Pause() {
	f.Set(paused)
}

func (f *ffmpegVideo) UnPause() {
	f.paused <- true
}

func (f *ffmpegVideo) Cancel() {
	f.Set(stopped)
}

func (f *ffmpegVideo) Stopped() bool {
	return f.Get() == stopped
}

func (f *ffmpegVideo) Get() int32 {
	return atomic.LoadInt32(&(f.player.flag))
}

func (f *ffmpegVideo) Set(value int32) {
	atomic.StoreInt32(&(f.player.flag), value)
}

func (f *ffmpegVideo) WaitUnPause() {
	if f.Get() == paused {
		<-f.paused
	}
}
