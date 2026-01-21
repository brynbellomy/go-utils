package bio_test

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bio "github.com/brynbellomy/go-utils/io"
)

func TestBufferedReadSeeker(t *testing.T) {
	t.Run("Seek() moves to correct offset and works between reads", func(t *testing.T) {
		data := "abcdefghijklmnopqrstuvwxyz"
		reader := strings.NewReader(data)
		brs := bio.NewBufferedReadSeeker(reader)

		buf := make([]byte, 5)
		n, err := brs.Read(buf)
		require.NoError(t, err)
		require.Equal(t, 5, n)
		require.Equal(t, "abcde", string(buf))

		offset, err := brs.Seek(10, io.SeekStart)
		require.NoError(t, err)
		require.Equal(t, int64(10), offset)

		n, err = brs.Read(buf)
		require.NoError(t, err)
		require.Equal(t, 5, n)
		require.Equal(t, "klmno", string(buf))

		offset, err = brs.Seek(3, io.SeekStart)
		require.NoError(t, err)
		require.Equal(t, int64(3), offset)

		n, err = brs.Read(buf)
		require.NoError(t, err)
		require.Equal(t, 5, n)
		require.Equal(t, "defgh", string(buf))
	})

	t.Run("Read() begins from correct offset after Seek()", func(t *testing.T) {
		data := "abcdefghijklmnopqrstuvwxyz"
		reader := strings.NewReader(data)
		brs := bio.NewBufferedReadSeeker(reader)

		offset, err := brs.Seek(5, io.SeekStart)
		require.NoError(t, err)
		require.Equal(t, int64(5), offset)

		buf := make([]byte, 5)
		n, err := brs.Read(buf)
		require.NoError(t, err)
		require.Equal(t, 5, n)
		require.Equal(t, "fghij", string(buf))
	})

	t.Run("Seek() blocks and unblocks when data is available", func(t *testing.T) {
		r, w := io.Pipe()
		brs := bio.NewBufferedReadSeeker(r)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			offset, err := brs.Seek(20, io.SeekStart)
			require.NoError(t, err)
			require.Equal(t, int64(20), offset)

			buf := make([]byte, 6)
			n, err := brs.Read(buf)
			require.NoError(t, err)
			require.Equal(t, 6, n)
			require.Equal(t, "uvwxyz", string(buf))
		}()

		// Give some time for the goroutine to start and block
		time.Sleep(50 * time.Millisecond)

		// Write data to unblock the seek
		n, err := w.Write([]byte("abcdefghijklmnopqrstuvwxyz"))
		require.NoError(t, err)
		require.Equal(t, 26, n)

		wg.Wait()
	})

	t.Run("Seek() returns io.ErrUnexpectedEOF if not enough data was available", func(t *testing.T) {
		r, w := io.Pipe()
		brs := bio.NewBufferedReadSeeker(r)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := brs.Seek(30, io.SeekStart)
			require.Equal(t, io.ErrUnexpectedEOF, err)
		}()

		// Give some time for the goroutine to start and block
		time.Sleep(50 * time.Millisecond)

		// Write data to unblock the seek
		n, err := w.Write([]byte("abcdefghijklmnopqrstuvwxyz"))
		require.NoError(t, err)
		require.Equal(t, 26, n)
		err = w.Close()
		require.NoError(t, err)

		wg.Wait()
	})
}
