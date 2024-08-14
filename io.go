package utils

import (
	"bytes"
	"io"

	"github.com/brynbellomy/go-utils/errors"
)

// EnsureSeekable ensures that the given reader is seekable by reading it all
// into memory and returning a seekable buffer.
func EnsureSeekable(r io.Reader) (io.ReadSeeker, error) {
	if rs, is := r.(io.ReadSeeker); is {
		return rs, nil
	}

	// Read the entire stream into memory
	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}

// BufferedReadSeeker is a ReadSeeker that buffers data read from the underlying
// reader into memory incrementally, allowing seeking up to the current position.
// If `Seek` is called with an offset that is not yet available, it will attempt
// to read up to that offset.  If the underlying reader returns EOF before that
// offset, `Seek` will return `io.ErrUnexpectedEOF`.
type BufferedReadSeeker struct {
	reader io.Reader
	buffer []byte
	pos    int64
}

func NewBufferedReadSeeker(reader io.Reader) *BufferedReadSeeker {
	return &BufferedReadSeeker{
		reader: reader,
		buffer: make([]byte, 0),
	}
}

func (brs *BufferedReadSeeker) Read(p []byte) (int, error) {
	n := copy(p, brs.buffer[brs.pos:])
	brs.pos += int64(n)

	var n2 int
	var err error
	if len(p[n:]) > 0 {
		n2, err = brs.reader.Read(p[n:])
		if n2 > 0 {
			brs.buffer = append(brs.buffer, p[n:n+n2]...)
			brs.pos += int64(n2)
		}
	}
	n += n2

	if err == io.EOF && n > 0 {
		return n, nil
	}
	return n, err
}

func (brs *BufferedReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var absoluteOffset int64
	switch whence {
	case io.SeekStart:
		absoluteOffset = offset
	case io.SeekCurrent:
		absoluteOffset = int64(brs.pos) + offset
	case io.SeekEnd:
		return 0, errors.New("SeekEnd not supported")
	default:
		return 0, errors.New("invalid whence")
	}

	if absoluteOffset < 0 {
		return 0, errors.New("negative position")
	}

	for {
		if int64(len(brs.buffer)) > absoluteOffset {
			brs.pos = absoluteOffset
			return brs.pos, nil
		}

		// If we haven't buffered enough data yet, read more
		_, err := brs.Read(make([]byte, 1024))
		if err == io.EOF {
			return 0, io.ErrUnexpectedEOF
		} else if err != nil {
			return 0, err
		}
	}
}

func (brs *BufferedReadSeeker) ReadAt(p []byte, off int64) (n int, err error) {
	_, err = brs.Seek(off, io.SeekStart)
	if err != nil {
		return 0, err
	}
	return brs.Read(p)
}
