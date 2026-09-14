//go:build !linux

package collect

import "os"

func fileInode(st os.FileInfo) uint64 { return uint64(st.Size()) }
