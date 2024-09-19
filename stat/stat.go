package stat

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fishBone000/xcat/log"
)

var (
	birth time.Time
)

type StatFile struct {
	f *os.File
}

func (s *StatFile) Init() {
	f, err := os.CreateTemp("", "xcat-*")
	if err != nil {
		log.Errf("Cannot create temp file for statistic: %v", err)
		os.Exit(255)
	}
	s.f = f
}

func (s *StatFile) Name() string {
	return s.f.Name()
}

func (s *StatFile) Write(t string, id int, msg string) {
  if s.f == nil {
    return
  }
	_, err := fmt.Fprintf(s.f, "%v %v %v %v\n", t, id, time.Since(birth).Milliseconds(), msg)
	if err != nil {
		log.Errf("Failed to write to statistic temp file: %v", err)
		os.Exit(255)
	}
}

type Counter struct {
	n int
	mux sync.Mutex
}

func (c *Counter) Tick() int {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.n += 1
	return c.n
}

func init() {
	birth = time.Now()
}
