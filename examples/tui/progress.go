package main

import (
	"time"

	"github.com/safedep/dry/tui"
	"github.com/safedep/dry/tui/progress"
)

func demoProgress() {
	tui.Heading("Progress — two trackers, one finishes early")

	p := progress.New()
	defer p.Wait() // MANDATORY in Rich mode to stop the animation goroutine.

	dl := p.Track("downloading", 100)
	vf := p.Track("verifying", 50)

	for i := 0; i < 5; i++ {
		dl.Increment(20)
		if i < 4 {
			vf.Increment(10)
		}
		if i == 3 {
			vf.Done()
		}
		time.Sleep(80 * time.Millisecond)
	}
	dl.Done()
}
