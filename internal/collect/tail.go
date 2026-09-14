package collect

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"watcher/internal/store"
)

type Tailer struct {
	Path     string
	StateKey string
	Store    *store.Store
	Backfill int64
}

func (t *Tailer) Follow(ctx context.Context, handle func(string)) error {
	if t.Path == "" {
		return fmt.Errorf("empty path")
	}
	var lastInode uint64
	var offset int64
	if raw, ok := t.Store.GetMeta(t.StateKey); ok {
		parts := strings.Split(raw, ":")
		if len(parts) == 2 {
			lastInode, _ = strconv.ParseUint(parts[0], 10, 64)
			offset, _ = strconv.ParseInt(parts[1], 10, 64)
		}
	}
	first := true
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		f, err := os.Open(t.Path)
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
				continue
			}
		}
		st, err := f.Stat()
		if err != nil {
			f.Close()
			return err
		}
		inode := fileInode(st)
		size := st.Size()
		if first && lastInode == 0 {
			if t.Backfill <= 0 {
				t.Backfill = 512 * 1024
			}
			start := size - t.Backfill
			if start < 0 {
				start = 0
			}
			offset = start
			lastInode = inode
		} else if inode != lastInode || size < offset {
			offset = 0
			lastInode = inode
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			f.Close()
			return err
		}
		buf := make([]byte, 32*1024)
		var carry []byte
		for {
			if err := ctx.Err(); err != nil {
				_ = t.Store.SetMeta(t.StateKey, fmt.Sprintf("%d:%d", lastInode, offset))
				f.Close()
				return err
			}
			n, err := f.Read(buf)
			if n > 0 {
				chunk := append(carry, buf[:n]...)
				lines := bytes.Split(chunk, []byte("\n"))
				carry = lines[len(lines)-1]
				for _, line := range lines[:len(lines)-1] {
					if len(line) == 0 {
						continue
					}
					handle(string(line))
				}
				offset += int64(n)
			}
			if err == io.EOF {
				if len(carry) == 0 {
					_ = t.Store.SetMeta(t.StateKey, fmt.Sprintf("%d:%d", lastInode, offset))
				}
				st, sterr := f.Stat()
				f.Close()
				if sterr == nil && fileInode(st) != lastInode {
					lastInode = 0
					offset = 0
					first = false
					break
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(800 * time.Millisecond):
				}
				first = false
				break
			}
			if err != nil {
				f.Close()
				return err
			}
		}
	}
}

func readable(path string) string {
	if path == "" {
		return "未配置"
	}
	st, err := os.Stat(path)
	if err != nil {
		return err.Error()
	}
	f, err := os.Open(path)
	if err != nil {
		return path + " 存在但不可读: " + err.Error()
	}
	f.Close()
	return fmt.Sprintf("%s (%d bytes)", path, st.Size())
}
