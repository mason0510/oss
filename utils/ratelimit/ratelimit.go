package ratelimit

import (
	"io"
	"time"
)

// New New
// rate 速度，rate/每秒
func New(rate int64) *Limiter {
	return &Limiter{
		rate:  time.Duration(rate),
		count: 0,
		t:     time.Now(),
	}
}

// Reader 返回一个带有Limiter的io.Reader
func Reader(r io.Reader, l *Limiter) io.Reader {
	return &reader{
		r: r,
		l: l,
	}
}

// ReadSeeker 返回一个带有Limiter的io.ReadSeeker
func _(rs io.ReadSeeker, l *Limiter) io.ReadSeeker {
	return &readSeeker{
		reader: reader{
			r: rs,
			l: l,
		},
		s: rs,
	}
}

// Writer 返回一个带有Limiter的io.Writer
func Writer(w io.Writer, l *Limiter) io.Writer {
	return &writer{
		w: w,
		l: l,
	}
}

// Limiter 速度限制器
type Limiter struct {
	rate  time.Duration
	count int64 // 最大8G
	t     time.Time
}

// Wait 传入需要处理的数量，计算并等待需要经过的时间
func (l *Limiter) Wait(count int64) {
	l.count += count
	t := time.Duration(l.count)*time.Second/l.rate - time.Since(l.t)
	if t > 0 {
		time.Sleep(t)
	}
}

type reader struct {
	r io.Reader
	l *Limiter
}

// Read Read
func (r *reader) Read(buf []byte) (int, error) {
	n, err := r.r.Read(buf)
	if n <= 0 {
		return n, err
	}
	r.l.Wait(int64(n))
	return n, err
}

type readSeeker struct {
	reader
	s io.Seeker
}

func (rs *readSeeker) Seek(offset int64, whence int) (int64, error) {
	return rs.s.Seek(offset, whence)
}

type writer struct {
	w io.Writer
	l *Limiter
}

// Write Write
func (w *writer) Write(buf []byte) (int, error) {
	w.l.Wait(int64(len(buf)))
	return w.w.Write(buf)
}
